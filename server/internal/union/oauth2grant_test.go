package union

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestGrantor(t *testing.T, sigKey *rsa.PrivateKey, mutate func(*Config)) *Grantor {
	t.Helper()
	cfg := Config{
		Enabled:          true,
		UnionOAuth2:      true,
		ApiRoot:          "https://hub.example/api/union",
		MemberKey:        "mk-grant",
		SigPrivateKeyPEM: privPEM(t, sigKey),
		SigPublicKeyPEM:  pubPEM(t, &sigKey.PublicKey),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	g, err := NewGrantor(cfg)
	if err != nil {
		t.Fatalf("NewGrantor: %v", err)
	}
	return g
}

func TestGrantTokenRoundTrip(t *testing.T) {
	sigKey := genRSA(t, 2048) // member's union_oauth2_sig pair
	hubKey := genRSA(t, 2048) // hub /oauth2/backend encryption key
	g := newTestGrantor(t, sigKey, nil)
	base := time.Unix(1_800_000_000, 0)
	g.now = func() time.Time { return base }

	token, err := g.GrantToken(42, "Nick", "nick@example.com", pubPEM(t, &hubKey.PublicKey))
	if err != nil {
		t.Fatalf("GrantToken: %v", err)
	}

	// 4. Self-decrypt with the hub private key (round-trip).
	plain, err := DecryptToken(hubKey, token)
	if err != nil {
		t.Fatalf("DecryptToken: %v", err)
	}
	var doc GrantTokenDoc
	if err := json.Unmarshal(plain, &doc); err != nil {
		t.Fatalf("token document not JSON: %v (%s)", err, plain)
	}
	if len(doc.MAC) != 64 || doc.Signature == "" || doc.UserInfo == "" {
		t.Fatalf("token document incomplete: %+v", doc)
	}

	// 1. userInfo payload: exact JSON field set/order, expires_at = now+600.
	infoJSON, err := base64.StdEncoding.DecodeString(doc.UserInfo)
	if err != nil {
		t.Fatalf("userInfo is not base64: %v", err)
	}
	wantInfo := `{"uid":42,"nickname":"Nick","email":"nick@example.com","expires_at":1800000600}`
	if string(infoJSON) != wantInfo {
		t.Fatalf("userInfo JSON drifted:\n got: %s\nwant: %s", infoJSON, wantInfo)
	}

	// 2. mac = hex(HMAC-SHA256(key=member key, msg=userInfoBase64)).
	m := hmac.New(sha256.New, []byte("mk-grant"))
	m.Write([]byte(doc.UserInfo))
	if doc.MAC != hex.EncodeToString(m.Sum(nil)) {
		t.Fatalf("mac mismatch: %s", doc.MAC)
	}

	// 3. signature = base64(RSA-SHA256 over "userInfo.mac").
	sig, err := base64.StdEncoding.DecodeString(doc.Signature)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	digest := sha256.Sum256([]byte(doc.UserInfo + "." + doc.MAC))
	if err := rsa.VerifyPKCS1v15(&sigKey.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("grant signature does not verify: %v", err)
	}

	if g.SignaturePublicKeyPEM() != pubPEM(t, &sigKey.PublicKey) {
		t.Fatal("SignaturePublicKeyPEM must serve the member signing public key")
	}
}

func TestGrantTokenEncryptionChunkBoundaries(t *testing.T) {
	// Multi-chunk payloads across key sizes: PKCS#1 v1.5 chunk = keyBytes-11.
	for _, bits := range []int{2048, 4096} {
		key := genRSA(t, bits)
		chunk := key.Size() - 11
		for _, size := range []int{0, 1, chunk, chunk + 1, chunk * 3, chunk*3 + 7, 10_000} {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i * 7)
			}
			enc, err := EncryptToken(&key.PublicKey, payload)
			if err != nil {
				t.Fatalf("%d bits / %d bytes: EncryptToken: %v", bits, size, err)
			}
			dec, err := DecryptToken(key, enc)
			if err != nil {
				t.Fatalf("%d bits / %d bytes: DecryptToken: %v", bits, size, err)
			}
			if !bytes.Equal(dec, payload) {
				t.Fatalf("%d bits / %d bytes: round-trip mismatch", bits, size)
			}
		}
	}
}

func TestGrantTokenRejectsBadHubKey(t *testing.T) {
	sigKey := genRSA(t, 2048)
	g := newTestGrantor(t, sigKey, nil)
	if _, err := g.GrantToken(1, "n", "e", "garbage"); err == nil {
		t.Fatal("garbage hub key must fail")
	}
}

func TestGrantRedirectURL(t *testing.T) {
	g := newTestGrantor(t, genRSA(t, 2048), nil)
	q := url.Values{}
	q.Set("client_id", "abc")
	q.Set("scope", "Yggdrasil.Server.Join")
	q.Set("userInfoToken", "attacker-controlled") // must be dropped

	got := g.GrantRedirectURL("tok+en/=", "https://hub.example/api/union/oauth2/continue", q)
	wantPrefix := "https://hub.example/api/union/oauth2/continue?userInfoToken="
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("redirect = %s, want prefix %s", got, wantPrefix)
	}
	if strings.Contains(got, "attacker-controlled") {
		t.Fatalf("original query must not override userInfoToken: %s", got)
	}
	if !strings.Contains(got, "client_id=abc") || !strings.Contains(got, "scope=Yggdrasil.Server.Join") {
		t.Fatalf("original query must be preserved: %s", got)
	}
	if !strings.Contains(got, "userInfoToken=tok%2Ben%2F%3D") {
		t.Fatalf("token must be query-escaped: %s", got)
	}
}

func TestSigKeyHandler(t *testing.T) {
	sigKey := genRSA(t, 2048)

	t.Run("serves signature key with hub-only CORS", func(t *testing.T) {
		g := newTestGrantor(t, sigKey, func(cfg *Config) { cfg.HubOrigin = "https://hub.example" })
		rec := httptest.NewRecorder()
		g.SigKeyHandler().ServeHTTP(rec, newRequest(t, http.MethodGet, "/api/union/member/oauth2/", ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("sig key endpoint = %d %s", rec.Code, rec.Body)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://hub.example" {
			t.Fatalf("CORS origin = %q, want hub origin only", rec.Header().Get("Access-Control-Allow-Origin"))
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Fatal("credentials header missing")
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body not JSON: %v", err)
		}
		if body["signaturePublicKey"] != pubPEM(t, &sigKey.PublicKey) || len(body) != 1 {
			t.Fatalf("body = %v", body)
		}
	})

	t.Run("origin derived from api root", func(t *testing.T) {
		g := newTestGrantor(t, sigKey, func(cfg *Config) { cfg.HubOrigin = "" })
		rec := httptest.NewRecorder()
		g.SigKeyHandler().ServeHTTP(rec, newRequest(t, http.MethodGet, "/api/union/member/oauth2/", ""))
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://hub.example" {
			t.Fatalf("derived CORS origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("oauth2 feature off is 403", func(t *testing.T) {
		g := newTestGrantor(t, sigKey, func(cfg *Config) { cfg.UnionOAuth2 = false })
		rec := httptest.NewRecorder()
		g.SigKeyHandler().ServeHTTP(rec, newRequest(t, http.MethodGet, "/api/union/member/oauth2/", ""))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("disabled oauth2 = %d, want 403", rec.Code)
		}
	})
}

func TestGrantorDisabled(t *testing.T) {
	g, err := NewGrantor(Config{Enabled: false, UnionOAuth2: true, SigPrivateKeyPEM: "garbage"})
	if err != nil {
		t.Fatalf("disabled grantor must construct without parsing keys: %v", err)
	}
	if g.Enabled() {
		t.Fatal("disabled grantor must report Enabled()==false")
	}
	if _, err := g.GrantToken(1, "n", "e", "x"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("GrantToken on disabled = %v, want ErrDisabled", err)
	}
	if g.SignaturePublicKeyPEM() != "" {
		t.Fatal("disabled grantor must not expose a key")
	}
}
