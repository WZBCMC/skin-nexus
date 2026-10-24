package poc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

// TestPEMMetadataSmoke covers M0-P4 at the crypto/format level:
//  1. textures property = Base64(JSON) with SHA1withRSA signature round-trip;
//  2. PEM SubjectPublicKeyInfo output with strict format (authlib-injector
//     exits the game on malformed PEM);
//  3. JSON zero-value semantics: empty textures map must serialize as {}
//     (JSON_FORCE_OBJECT equivalent), never null or [].
func TestPEMMetadataSmoke(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// 1. textures property sign/verify (SHA1withRSA over the value string).
	tex := map[string]interface{}{
		"timestamp":   1761100000000,
		"profileId":   "069a79f444e94726a5befca90e38aaf5",
		"profileName": "Steve",
		"textures": map[string]interface{}{
			"SKIN": map[string]interface{}{
				"url":      "https://skin.example.com/textures/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				"metadata": map[string]interface{}{"model": "default"},
			},
		},
	}
	value := base64.StdEncoding.EncodeToString(mustJSON(t, tex))
	digest := sha1.Sum([]byte(value))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, digest[:], sig); err != nil {
		t.Fatalf("signature verify failed: %v", err)
	}

	// 2. PEM strict format.
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	s := string(pubPEM)
	if !strings.HasPrefix(s, "-----BEGIN PUBLIC KEY-----\n") || !strings.HasSuffix(s, "-----END PUBLIC KEY-----\n") {
		t.Fatalf("PEM header/footer malformed:\n%s", s)
	}
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if !strings.HasPrefix(line, "-----") && (len(line) > 64 || strings.ContainsAny(line, "\r")) {
			t.Fatalf("PEM body line invalid (len=%d, CR): %q", len(line), line)
		}
	}

	// 3. Zero-value semantics: empty map stays {}.
	empty := map[string]interface{}{
		"timestamp":   1761100000000,
		"profileId":   "069a79f444e94726a5befca90e38aaf5",
		"profileName": "Steve",
		"textures":    map[string]interface{}{},
	}
	b := mustJSON(t, empty)
	if !strings.Contains(string(b), `"textures":{}`) {
		t.Fatalf("empty textures collapsed: %s", b)
	}
	if strings.Contains(string(b), `"textures":null`) || strings.Contains(string(b), `"textures":[]`) {
		t.Fatalf("empty textures collapsed to null/array: %s", b)
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
