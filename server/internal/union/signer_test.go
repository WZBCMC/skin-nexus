package union

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// newKeyHub serves GET /privatekey from a precomputed key queue: each call
// consumes the next key, then the last one repeats.
func newKeyHub(t *testing.T, keys []*rsa.PrivateKey) *httptest.Server {
	t.Helper()
	pems := make([]string, len(keys))
	for i, k := range keys {
		pems[i] = privPEM(t, k)
	}
	idx := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := idx
		if i >= len(pems) {
			i = len(pems) - 1
		} else {
			idx++
		}
		jsonRespond(w, http.StatusOK, map[string]any{
			"privateKey":        pems[i],
			"privateKeyVersion": fmt.Sprintf("v%d", i+1),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestSigner(t *testing.T, hubURL string, log *auditLog) *UnionSigner {
	t.Helper()
	cfg := Config{Enabled: true, ApiRoot: hubURL, MemberKey: "mk-signer"}
	out, err := NewOutbound(cfg, nil)
	if err != nil {
		t.Fatalf("NewOutbound: %v", err)
	}
	var audit AuditFunc
	if log != nil {
		audit = log.fn()
	}
	return NewUnionSigner(cfg, out, &fakeSigner{fp: "SHA256:local"}, audit)
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestSignerFallbackThenManaged(t *testing.T) {
	log := &auditLog{}
	s := NewUnionSigner(Config{Enabled: true}, nil, &fakeSigner{fp: "SHA256:local"}, log.fn())

	if s.Mode() != ModeLocalFallback {
		t.Fatalf("initial mode = %s, want local", s.Mode())
	}
	if s.PublicKeyPEM() != "FALLBACK-PEM" || s.KeyFingerprint() != "SHA256:local" {
		t.Fatal("fallback signer must back PublicKeyPEM/KeyFingerprint")
	}
	if sig, err := s.SignValue("textures"); err != nil || sig != "signed:textures" {
		t.Fatalf("fallback SignValue = %q, %v", sig, err)
	}
	if s.PrivateKeyVersion() != "" {
		t.Fatalf("initial version = %q", s.PrivateKeyVersion())
	}

	key := genRSA(t, 2048)
	if err := s.SetKey(privPEM(t, key), "v1"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if s.Mode() != ModeUnionManaged {
		t.Fatalf("mode after SetKey = %s", s.Mode())
	}
	if s.PrivateKeyVersion() != "v1" {
		t.Fatalf("version = %q", s.PrivateKeyVersion())
	}
	// SHA1withRSA, base64 — identical to ygg.LocalSigner (§7.1-5).
	sig, err := s.SignValue("textures")
	if err != nil {
		t.Fatalf("SignValue: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature not base64: %v", err)
	}
	digest := sha1.Sum([]byte("textures"))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, digest[:], raw); err != nil {
		t.Fatalf("signature does not verify against the hub key: %v", err)
	}
	// PKCS#1 PEM input is accepted too (hub key-format drift).
	if err := s.SetKey(privPEMPKCS1(t, key), "v2"); err != nil {
		t.Fatalf("SetKey(PKCS#1): %v", err)
	}
}

func TestSignerRotationSwitchesOnlyOnFingerprintChange(t *testing.T) {
	// The hub repeats k1 once so the second poll sees the same key.
	k1, k2 := genRSA(t, 2048), genRSA(t, 2048)
	hub := newKeyHub(t, []*rsa.PrivateKey{k1, k1, k2})
	log := &auditLog{}
	s := newTestSigner(t, hub.URL, log)
	ctx := context.Background()

	changed, err := s.rotateOnce(ctx)
	if err != nil || !changed {
		t.Fatalf("first rotation = %v, %v", changed, err)
	}
	if s.Mode() != ModeUnionManaged || s.PrivateKeyVersion() != "v1" {
		t.Fatalf("after first rotation: mode=%s version=%s", s.Mode(), s.PrivateKeyVersion())
	}

	// Same key again: no switch, no version bump.
	changed, err = s.rotateOnce(ctx)
	if err != nil || changed {
		t.Fatalf("identical key must not switch: %v, %v", changed, err)
	}

	// Fingerprint difference: switch (one-way; new signatures use the new key).
	changed, err = s.rotateOnce(ctx)
	if err != nil || !changed {
		t.Fatalf("new key must switch: %v, %v", changed, err)
	}
	// Version follows the hub's call order (third poll served the new key).
	if s.PrivateKeyVersion() != "v3" {
		t.Fatalf("version after switch = %s", s.PrivateKeyVersion())
	}
	if n := log.count("union.signer.rotation.switched"); n != 2 {
		t.Fatalf("switch audit count = %d", n)
	}
}

func TestSignerDailyChangeBudget(t *testing.T) {
	// One distinct key per hub call so every rotation is a real change.
	keys := make([]*rsa.PrivateKey, 10)
	for i := range keys {
		keys[i] = genRSA(t, 2048)
	}
	hub := newKeyHub(t, keys)
	log := &auditLog{}
	s := newTestSigner(t, hub.URL, log)
	day := time.Unix(1_800_000_000, 0)
	s.now = func() time.Time { return day }
	ctx := context.Background()

	for i := 0; i < maxKeyChangesPerDay; i++ {
		changed, err := s.rotateOnce(ctx)
		if err != nil || !changed {
			t.Fatalf("change %d = %v, %v", i+1, changed, err)
		}
	}
	fp := s.KeyFingerprint()

	// The 4th change in one day is an anomaly: refuse + suspend + alert.
	if _, err := s.rotateOnce(ctx); !errors.Is(err, ErrRotationSuspended) {
		t.Fatalf("4th change = %v, want ErrRotationSuspended", err)
	}
	if !s.RotationSuspended() {
		t.Fatal("rotation must be suspended")
	}
	if s.KeyFingerprint() != fp {
		t.Fatal("suspended rotation must not switch keys")
	}
	if _, ok := log.last("union.signer.rotation.suspended"); !ok {
		t.Fatalf("suspension audit missing: %+v", log.events)
	}
	if _, err := s.rotateOnce(ctx); !errors.Is(err, ErrRotationSuspended) {
		t.Fatal("suspended signer must keep refusing")
	}

	// Human review: ResumeRotation lifts the suspension and resets the budget.
	if err := s.ResumeRotation(); err != nil {
		t.Fatalf("ResumeRotation: %v", err)
	}
	if s.RotationSuspended() {
		t.Fatal("suspension must be lifted")
	}
	changed, err := s.rotateOnce(ctx)
	if err != nil || !changed {
		t.Fatalf("post-resume change = %v, %v", changed, err)
	}

	// The budget is per UTC day: rolling the clock resets it.
	for i := 0; i < maxKeyChangesPerDay+1; i++ {
		_, _ = s.rotateOnce(ctx)
	}
	if !s.RotationSuspended() {
		t.Fatal("budget must suspend again within the same day")
	}
	s.now = func() time.Time { return day.Add(25 * time.Hour) }
	if err := s.ResumeRotation(); err != nil {
		t.Fatalf("ResumeRotation: %v", err)
	}
	if changed, err := s.rotateOnce(ctx); err != nil || !changed {
		t.Fatalf("next-day change = %v, %v", changed, err)
	}
}

func TestSignerConcurrentSignAndRotate(t *testing.T) {
	k1, k2 := genRSA(t, 2048), genRSA(t, 2048)
	pem1, pem2 := privPEM(t, k1), privPEM(t, k2)
	s := NewUnionSigner(Config{Enabled: true}, nil, nil, nil)
	if err := s.SetKey(pem1, "v1"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				sig, err := s.SignValue("textures")
				if err != nil {
					errs <- err
					return
				}
				raw, err := base64.StdEncoding.DecodeString(sig)
				if err != nil {
					errs <- err
					return
				}
				digest := sha1.Sum([]byte("textures"))
				ok1 := rsa.VerifyPKCS1v15(&k1.PublicKey, crypto.SHA1, digest[:], raw) == nil
				ok2 := rsa.VerifyPKCS1v15(&k2.PublicKey, crypto.SHA1, digest[:], raw) == nil
				if !ok1 && !ok2 {
					errs <- errors.New("signature verifies against neither key")
					return
				}
			}
		}()
	}
	for i := 0; i < 200; i++ {
		if i%2 == 0 {
			_ = s.SetKey(pem2, "v2")
		} else {
			_ = s.SetKey(pem1, "v1")
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent signing: %v", err)
	}
}

func TestStartRotationValidatesInterval(t *testing.T) {
	hub := newKeyHub(t, []*rsa.PrivateKey{genRSA(t, 2048)})
	s := newTestSigner(t, hub.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.StartRotation(ctx, 5*time.Minute); err == nil {
		t.Fatal("interval below the 10-15min policy window must be refused")
	}
	if err := s.StartRotation(ctx, 16*time.Minute); err == nil {
		t.Fatal("interval above the policy window must be refused")
	}
	if err := s.StartRotation(ctx, 12*time.Minute); err != nil {
		t.Fatalf("policy-window interval must start: %v", err)
	}
	if err := s.StartRotation(ctx, 12*time.Minute); err == nil {
		t.Fatal("second loop must be refused")
	}
}

func TestRotationLoopSwitchesThenSuspends(t *testing.T) {
	// Drive the real loop fast by narrowing the policy window for the test.
	oldMin, oldMax, oldJitter := rotationIntervalMin, rotationIntervalMax, rotationJitterMax
	rotationIntervalMin, rotationIntervalMax, rotationJitterMax = 5*time.Millisecond, 50*time.Millisecond, time.Millisecond
	t.Cleanup(func() { rotationIntervalMin, rotationIntervalMax, rotationJitterMax = oldMin, oldMax, oldJitter })

	keys := []*rsa.PrivateKey{genRSA(t, 2048), genRSA(t, 2048), genRSA(t, 2048), genRSA(t, 2048)}
	hub := newKeyHub(t, keys)
	s := newTestSigner(t, hub.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.StartRotation(ctx, 10*time.Millisecond); err != nil {
		t.Fatalf("StartRotation: %v", err)
	}
	// The hub serves a new key per call: 3 changes land, the 4th suspends.
	waitFor(t, 5*time.Second, s.RotationSuspended)
	if s.PrivateKeyVersion() != "v3" {
		t.Fatalf("version after budget = %s, want v3 (4th key refused)", s.PrivateKeyVersion())
	}

	// Resume re-arms the loop; the previously refused key is then accepted.
	if err := s.ResumeRotation(); err != nil {
		t.Fatalf("ResumeRotation: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return s.PrivateKeyVersion() == "v4" })
}

func TestSignerDisabledIsInert(t *testing.T) {
	s := NewUnionSigner(Config{Enabled: false}, nil, &fakeSigner{fp: "x"}, nil)
	if s.Enabled() {
		t.Fatal("disabled signer must report Enabled()==false")
	}
	if s.Mode() != ModeDisabled {
		t.Fatalf("mode = %s", s.Mode())
	}
	if _, err := s.SignValue("v"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("SignValue on disabled = %v", err)
	}
	if err := s.SetKey("garbage", "v"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("SetKey on disabled = %v", err)
	}
	if err := s.StartRotation(context.Background(), 12*time.Minute); !errors.Is(err, ErrDisabled) {
		t.Fatalf("StartRotation on disabled = %v", err)
	}
	if err := s.ResumeRotation(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("ResumeRotation on disabled = %v", err)
	}
}
