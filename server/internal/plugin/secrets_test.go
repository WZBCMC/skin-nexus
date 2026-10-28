package plugin

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSecretBoxFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin-secrets.key")
	box, err := NewSecretBox("", path)
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() != 32 {
		t.Fatalf("key file = %v/%v", fi, err)
	}
	ct, nonce, err := box.SealString("hello")
	if err != nil {
		t.Fatal(err)
	}
	got, err := box.OpenString(ct, nonce)
	if err != nil || got != "hello" {
		t.Fatalf("open = %q/%v", got, err)
	}
	if string(ct) == "hello" {
		t.Fatal("ciphertext is plaintext")
	}

	// A different key must not decrypt.
	other, err := NewSecretBox("", filepath.Join(dir, "other.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.OpenString(ct, nonce); err == nil {
		t.Fatal("wrong key decrypted")
	}

	// Env key path (hex).
	raw, _ := os.ReadFile(path)
	envBox, err := NewSecretBox(hex.EncodeToString(raw), "")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := envBox.OpenString(ct, nonce); err != nil || got != "hello" {
		t.Fatalf("env open = %q/%v", got, err)
	}
	if _, err := NewSecretBox("bad", ""); err == nil {
		t.Fatal("bad env key accepted")
	}
}
