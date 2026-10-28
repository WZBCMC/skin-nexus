package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeygenPackRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "market.pem")
	var out, errOut bytes.Buffer
	if code := runKeygen([]string{"--id", "test", "--out", keyPath}, &out, &errOut); code != 0 {
		t.Fatalf("keygen=%d %s", code, errOut.String())
	}
	keys := strings.TrimSpace(out.String())
	if !strings.HasPrefix(keys, "test:") {
		t.Fatalf("keygen output = %q", keys)
	}

	pluginDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"demo","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":[],"minServer":">=0.3.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "index.js"), []byte("skin.log('x');"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "market.json"), []byte(`{"schema":1,"title":{"en":"Demo"},"description":{"en":"demo"},"minServer":">=0.3.0","capabilities":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	if code := runPack([]string{"--dir", pluginDir, "--out", dist, "--key", keyPath, "--key-id", "test", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("pack=%d %s", code, errOut.String())
	}
	bundle := filepath.Join(dist, "demo-1.0.0.zip")
	if _, err := os.Stat(bundle); err != nil {
		t.Fatalf("bundle missing: %v", err)
	}
	out.Reset()
	errOut.Reset()
	if code := runVerify([]string{"--bundle", bundle, "--keys", keys, "--host-version", "0.3.0"}, &out, &errOut); code != 0 {
		t.Fatalf("verify=%d %s", code, errOut.String())
	}

	regPath := filepath.Join(dir, "registry.json")
	out.Reset()
	errOut.Reset()
	if code := runRegistry([]string{"--dist", dist, "--url-base", "https://example.com/artifacts", "--key", keyPath, "--key-id", "test", "--out", regPath, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("registry=%d %s", code, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := runVerifyRegistry([]string{"--registry", regPath, "--keys", keys, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("verify-registry=%d %s", code, errOut.String())
	}
}
