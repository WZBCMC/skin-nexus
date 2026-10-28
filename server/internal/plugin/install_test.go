package plugin

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skin-nexus/server/internal/signedpkg"
)

func makeSignedBundle(t *testing.T, dir, name, version string, keyring signedpkg.Keyring, priv ed25519.PrivateKey, keyID string) string {
	t.Helper()
	manifest := `{"name":"` + name + `","version":"` + version + `","hookApi":2,"entrypoint":"index.js","permissions":[]}`
	files := map[string]string{"plugin.json": manifest, "index.js": "skin.log('" + version + "');"}
	first := filepath.Join(dir, "first.zip")
	writeZip(t, first, files)
	digest, err := signedpkg.ContentDigest(first, "bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	sig := signedpkg.Sign(priv, signedpkg.BundlePayload(name, version, digest))
	meta, _ := json.Marshal(map[string]string{
		"name": name, "version": version, "keyId": keyID,
		"signature": base64.StdEncoding.EncodeToString(sig),
	})
	files["bundle.json"] = string(meta)
	out := filepath.Join(dir, name+"_"+version+".zip")
	writeZip(t, out, files)
	return out
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, body := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
}

func newInstallHost(t *testing.T) (*Manager, signedpkg.Keyring, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(nil)
	keyring := signedpkg.Keyring{"test": pub}
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	m := NewHost(RuntimeConfig{Dir: root, StateDir: state, HostVersion: "0.4.0", Keyring: keyring, Store: newMemPlatformStore()})
	t.Cleanup(func() { _ = m.Close() })
	return m, keyring, priv, root
}

func TestInstallUpgradeRollback(t *testing.T) {
	m, kr, priv, _ := newInstallHost(t)
	dir := t.TempDir()
	z1 := makeSignedBundle(t, dir, "demo", "1.0.0", kr, priv, "test")
	res, err := m.InstallBundle(context.Background(), z1, InstallOptions{Source: "upload"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "disabled" {
		t.Fatalf("new install state = %s", res.State)
	}
	if err := m.Enable(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	z2 := makeSignedBundle(t, dir, "demo", "2.0.0", kr, priv, "test")
	if _, err := m.InstallBundle(context.Background(), z2, InstallOptions{Source: "upload"}); err != nil {
		t.Fatal(err)
	}
	st := m.statusOf("demo")
	if st == nil || st.Version != "2.0.0" || !st.Enabled {
		t.Fatalf("after upgrade = %+v", st)
	}
	if err := m.Rollback(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	st = m.statusOf("demo")
	if st == nil || st.Version != "1.0.0" {
		t.Fatalf("after rollback = %+v", st)
	}
	if err := m.Uninstall(context.Background(), "demo", false); err != nil {
		t.Fatal(err)
	}
	if m.statusOf("demo") != nil {
		t.Fatal("still listed after uninstall")
	}
}

func TestInstallRejectsTamperedBundle(t *testing.T) {
	m, kr, priv, _ := newInstallHost(t)
	dir := t.TempDir()
	z := makeSignedBundle(t, dir, "demo", "1.0.0", kr, priv, "test")
	// Replace index.js after signing: content digest no longer matches.
	rewriteZipEntry(t, z, "index.js", "tampered")
	if _, err := m.InstallBundle(context.Background(), z, InstallOptions{Source: "upload"}); !errors.Is(err, ErrUnverifiedPlugin) {
		t.Fatalf("tampered bundle = %v, want ErrUnverifiedPlugin", err)
	}
}

func rewriteZipEntry(t *testing.T, path, name, body string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		buf := make([]byte, f.UncompressedSize64)
		_, _ = rc.Read(buf)
		rc.Close()
		files[f.Name] = string(buf)
	}
	zr.Close()
	files[name] = body
	writeZip(t, path, files)
}
