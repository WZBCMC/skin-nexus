package signedpkg

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndVerifyBundle(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	ring := Keyring{"test": pub}
	path := filepath.Join(t.TempDir(), "demo.zip")
	digest, sha, err := WriteNamedBundle(path, "demo", "1.0.0", []Entry{
		{Name: "plugin.json", Data: []byte(`{"name":"demo"}`)},
		{Name: "index.js", Data: []byte("skin.log('x');")},
	}, priv, "test")
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" || sha == "" {
		t.Fatal("empty digest/sha")
	}
	meta, err := VerifyBundle(path, ring)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "demo" || meta.Version != "1.0.0" {
		t.Fatalf("meta = %+v", meta)
	}
	entry, err := ReadEntry(path, "index.js", 1024)
	if err != nil || string(entry) != "skin.log('x');" {
		t.Fatalf("entry = %q %v", entry, err)
	}
	// Tamper with a payload entry but keep bundle.json.
	zr, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = zr.Close()
	if _, err := VerifyBundle(path, Keyring{"other": pub}); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestValidateEntryName(t *testing.T) {
	ok := []string{"plugin.json", "ui/a.html", "a/b-c.js"}
	bad := []string{"../x", "/abs", `a\b`, "CON.txt", "a/./b", "a//b", "a/..", "a.", "a ", "x:y"}
	for _, n := range ok {
		if err := ValidateEntryName(n); err != nil {
			t.Fatalf("valid %q: %v", n, err)
		}
	}
	for _, n := range bad {
		if err := ValidateEntryName(n); err == nil {
			t.Fatalf("invalid %q accepted", n)
		}
	}
}
