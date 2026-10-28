package marketfmt

import (
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"skin-nexus/server/internal/signedpkg"
)

func TestRegistrySignVerifyAndCanonicalPayload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	ring := signedpkg.Keyring{"test": pub}
	now := time.Now().UTC()
	pkgs := []Package{{
		Name: "demo", Version: "1.0.0",
		Title:        map[string]string{"zh-CN": "演示", "en": "Demo"},
		Capabilities: []string{"config"},
		Dist:         Dist{URL: "https://example.com/demo-1.0.0.zip", Size: 123, SHA256: strings.Repeat("a", 64)},
	}}
	reg := Build(pkgs, "test", now, now.Add(24*time.Hour))
	if err := Sign(&reg, priv); err != nil {
		t.Fatal(err)
	}
	payload, err := Payload(reg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "signature") {
		t.Fatalf("signature leaked into payload: %s", payload)
	}
	raw := []byte(`{"schema":1,"generatedAt":"` + reg.GeneratedAt + `","expiresAt":"` + reg.ExpiresAt + `","keyId":"` + reg.KeyID + `","packages":` + string(mustJSON(reg.Packages)) + `,"signature":"` + reg.Signature + `"}`)
	got, err := VerifyRegistry(raw, ring, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Packages[0].Name != "demo" {
		t.Fatalf("packages = %+v", got.Packages)
	}
	// Tamper with a signed field.
	raw = []byte(strings.Replace(string(raw), `"version":"1.0.0"`, `"version":"9.9.9"`, 1))
	if _, err := VerifyRegistry(raw, ring, now); err == nil {
		t.Fatal("tampered registry accepted")
	}
	// Expired.
	if _, err := VerifyRegistry([]byte(strings.Replace(string(raw), reg.ExpiresAt, now.Add(-time.Hour).Format(time.RFC3339), 1)), ring, now); err == nil {
		t.Fatal("expired registry accepted")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.2", "1.1.9", 1},
		{"2.0", "2.0.0", 0},
	}
	for _, tc := range cases {
		got, err := CompareVersions(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Fatalf("compare %s %s = %d,%v want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	if _, err := CompareVersions("bad", "1.0"); err == nil {
		t.Fatal("invalid version accepted")
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
