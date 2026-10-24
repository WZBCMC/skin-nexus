package union

// hotmode_test.go — 2026-10-24 拍板「热切换 + 现取」回归：
//  1. hub 验签公钥经 keySource 现取（带缓存），pinned 仅作现取失败兜底；
//  2. UnionSigner.SetHubMode(false) 即弃用 hub 键回本地（信任声明：关闭 =
//     共享钥不参与签名），再开启可继续用已落地 hub 键；
//  3. Outbound.SetEnabled/SetAPIRoot 热生效、关闭全 ErrDisabled。

import (
	"context"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// keySourceHub serves the hub root JSON with the union_host_signature_public_key
// field (the UnionHostVerify.php:67 fetch target) and counts fetches.
func keySourceHub(t *testing.T, pub string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		jsonRespond(w, 200, map[string]any{"union_host_signature_public_key": pub})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestHostKeySourceFetchesAndCaches(t *testing.T) {
	t.Parallel()
	hub := genRSA(t, 2048)
	srv, calls := keySourceHub(t, pubPEM(t, &hub.PublicKey))

	out, err := NewOutbound(Config{Enabled: true, ApiRoot: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(Config{Enabled: true}, func(string, map[string]any) {}) // no pinned key
	if err != nil {
		t.Fatal(err)
	}
	v.SetHostKeySource(func() (string, error) { return out.HostPublicKey(context.Background()) })

	for i := 0; i < 2; i++ {
		body := `{"op":"hello"}`
		req := signedRequest(t, hub, http.MethodPost, "/api/union/member/sync", body, time.Now(), "nonce-cache-"+string(rune('a'+i)))
		rec, called := serve(v, req)
		if rec.Code != http.StatusOK || !called {
			t.Fatalf("verify %d rejected: status=%d body=%s", i, rec.Code, rec.Body)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("hub root fetched %d times, want 1 (second verify must hit cache)", got)
	}
}

func TestHostKeySourceFallsBackToPinned(t *testing.T) {
	t.Parallel()
	hub := genRSA(t, 2048)
	v, err := NewVerifier(Config{Enabled: true, HostPublicKeyPEM: pubPEM(t, &hub.PublicKey)},
		func(string, map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	v.SetHostKeySource(func() (string, error) { return "", errors.New("hub down") })

	body := `{"op":"hello"}`
	req := signedRequest(t, hub, http.MethodPost, "/api/union/member/sync", body, time.Now(), "nonce-pin-fb")
	rec, called := serve(v, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("pinned fallback must verify: status=%d body=%s", rec.Code, rec.Body)
	}
}

func TestHostKeySourceUnreachableWithoutPinnedRejects(t *testing.T) {
	t.Parallel()
	hub := genRSA(t, 2048)
	v, err := NewVerifier(Config{Enabled: true}, func(string, map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	v.SetHostKeySource(func() (string, error) { return "", errors.New("hub down") })

	body := `{"op":"hello"}`
	req := signedRequest(t, hub, http.MethodPost, "/api/union/member/sync", body, time.Now(), "nonce-no-key")
	rec, called := serve(v, req)
	if called || rec.Code != http.StatusForbidden {
		t.Fatalf("no key source and no pin must reject, status=%d called=%v", rec.Code, called)
	}
}

func TestSignerHubModeHotToggle(t *testing.T) {
	t.Parallel()
	local := &fakeSigner{fp: "SHA256:local"}
	s := NewUnionSigner(Config{Enabled: true}, nil, local, nil)

	// land a hub key (SetKey is the landing primitive, same as rotation)
	hub := genRSA(t, 2048)
	hubPEM := privPEM(t, hub)
	if err := s.SetKey(hubPEM, "7"); err != nil {
		t.Fatal(err)
	}
	if s.Mode() != ModeUnionManaged {
		t.Fatalf("mode=%s want %s", s.Mode(), ModeUnionManaged)
	}
	if s.PrivateKeyVersion() != "7" {
		t.Fatalf("PrivateKeyVersion=%q want 7", s.PrivateKeyVersion())
	}

	// hot-off: must sign with the LOCAL fallback even though a hub key landed
	s.SetHubMode(false)
	if s.Mode() != ModeLocalFallback {
		t.Fatalf("mode=%s want %s after SetHubMode(false)", s.Mode(), ModeLocalFallback)
	}
	if s.PrivateKeyVersion() != "" {
		t.Fatalf("hot-off must not report a hub key version, got %q", s.PrivateKeyVersion())
	}
	if s.PublicKeyPEM() != "FALLBACK-PEM" {
		t.Fatalf("hot-off PublicKeyPEM=%q want FALLBACK-PEM", s.PublicKeyPEM())
	}

	// hot-on again: hub key resumes without a restart
	s.SetHubMode(true)
	if s.Mode() != ModeUnionManaged {
		t.Fatalf("mode=%s want %s after re-enable", s.Mode(), ModeUnionManaged)
	}
	if s.PrivateKeyVersion() != "7" {
		t.Fatalf("re-enabled PrivateKeyVersion=%q want 7", s.PrivateKeyVersion())
	}
}

func TestRotateOnceBlockedWhileHubModeOff(t *testing.T) {
	t.Parallel()
	k1 := genRSA(t, 2048)
	srv := newKeyHub(t, []*rsa.PrivateKey{k1})
	out, err := NewOutbound(Config{Enabled: true, ApiRoot: srv.URL, MemberKey: "mk"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := NewUnionSigner(Config{Enabled: true}, out, &fakeSigner{fp: "SHA256:local"}, nil)
	s.SetHubMode(false) // hot-off
	if _, err := s.rotateOnce(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("rotateOnce while hub mode off must return ErrDisabled, got %v", err)
	}
	s.SetHubMode(true) // hot-on resumes the poll
	if _, err := s.rotateOnce(context.Background()); err != nil {
		t.Fatalf("rotateOnce after re-enable: %v", err)
	}
	if s.Mode() != ModeUnionManaged {
		t.Fatalf("mode=%s want %s (hub key must land)", s.Mode(), ModeUnionManaged)
	}
}

func TestOutboundHotSwitch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonRespond(w, 200, map[string]any{})
	}))
	defer srv.Close()
	out, err := NewOutbound(Config{Enabled: true, ApiRoot: srv.URL, MemberKey: "k"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Enabled() {
		t.Fatal("must start enabled")
	}
	out.SetEnabled(false)
	if out.Enabled() {
		t.Fatal("SetEnabled(false) must disable")
	}
	if _, err := out.ServerList(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled outbound must return ErrDisabled, got %v", err)
	}
	out.SetEnabled(true)
	if err := out.SetAPIRoot("not-a-url"); err == nil {
		t.Fatal("SetAPIRoot must reject garbage")
	}
	if got := out.APIRoot(); got != srv.URL {
		t.Fatalf("APIRoot=%q want %q (rejected root must not swap)", got, srv.URL)
	}
	if err := out.SetAPIRoot(srv.URL + "/moved/"); err != nil {
		t.Fatal(err)
	}
	if got := out.APIRoot(); got != srv.URL+"/moved" {
		t.Fatalf("APIRoot=%q want trailing-slash-trimmed %q", got, srv.URL+"/moved")
	}
}
