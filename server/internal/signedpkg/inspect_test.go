package signedpkg

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestInspectStatuses(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	ring := Keyring{"test": pub}
	dir := t.TempDir()
	path := filepath.Join(dir, "signed.zip")
	if _, _, err := WriteNamedBundle(path, "demo", "1.0.0", []Entry{
		{Name: "plugin.json", Data: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{Name: "index.js", Data: []byte("1;")},
	}, priv, "test"); err != nil {
		t.Fatal(err)
	}
	insp, err := Inspect(path, ring)
	if err != nil || insp.Status != StatusVerified || insp.Meta.KeyID != "test" || insp.Digest == "" || insp.ZipSHA == "" {
		t.Fatalf("verified inspect = %+v %v", insp, err)
	}

	unknown, err := Inspect(path, Keyring{"other": pub})
	if err != nil || unknown.Status != StatusUnknownKey || unknown.Reason == "" {
		t.Fatalf("unknown-key inspect = %+v %v", unknown, err)
	}

	unsigned := filepath.Join(dir, "unsigned.zip")
	makeZip(t, unsigned, map[string]string{"plugin.json": `{"name":"demo","version":"1.0.0"}`, "index.js": "1;"})
	insp, err = Inspect(unsigned, ring)
	if err != nil || insp.Status != StatusUnsigned || insp.HasMeta {
		t.Fatalf("unsigned inspect = %+v %v", insp, err)
	}

	tampered := filepath.Join(dir, "tampered.zip")
	makeZip(t, tampered, map[string]string{
		"plugin.json": `{"name":"demo","version":"1.0.0"}`,
		"index.js":    "2;",
		"bundle.json": mustBundleJSON(t, "demo", "1.0.0", "test", priv),
	})
	insp, err = Inspect(tampered, ring)
	if err != nil || insp.Status != StatusInvalid {
		t.Fatalf("tampered inspect = %+v %v", insp, err)
	}
}

func mustBundleJSON(t *testing.T, name, version, keyID string, priv ed25519.PrivateKey) string {
	t.Helper()
	// Sign over a deliberately wrong digest: the signature decodes, but does
	// not match the contents.
	sig := Sign(priv, BundlePayload(name, version, "00"))
	b, _ := json.Marshal(BundleMeta{Name: name, Version: version, KeyID: keyID, Signature: base64.StdEncoding.EncodeToString(sig)})
	return string(b)
}
