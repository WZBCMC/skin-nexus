package signedpkg

import (
	"archive/zip"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyringSignVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	raw := "test:" + base64.StdEncoding.EncodeToString(pub)
	kr, err := ParseKeys(raw)
	if err != nil {
		t.Fatal(err)
	}
	data := BundlePayload("demo", "1.0.0", "abc")
	sig := Sign(priv, data)
	if err := kr.Verify("test", data, sig); err != nil {
		t.Fatal(err)
	}
	if err := kr.Verify("test", []byte("tampered"), sig); err == nil {
		t.Fatal("tampered payload accepted")
	}
	if err := kr.Verify("other", data, sig); err == nil {
		t.Fatal("unknown key accepted")
	}
	if _, err := ParseKeys("bad"); err == nil {
		t.Fatal("malformed keyring accepted")
	}
}

func TestContentDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.zip")
	makeZip(t, path, map[string]string{"plugin.json": "{}", "index.js": "1", "bundle.json": "sig"})
	d1, err := ContentDigest(path, "bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	// bundle.json is excluded, so changing it keeps the digest.
	makeZip(t, path, map[string]string{"plugin.json": "{}", "index.js": "1", "bundle.json": "other"})
	d2, _ := ContentDigest(path, "bundle.json")
	if d1 != d2 {
		t.Fatal("bundle.json affected content digest")
	}
	makeZip(t, path, map[string]string{"plugin.json": "{}", "index.js": "2", "bundle.json": "sig"})
	d3, _ := ContentDigest(path, "bundle.json")
	if d1 == d3 {
		t.Fatal("payload change did not affect digest")
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
}
