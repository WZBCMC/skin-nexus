package plugin

import (
	"context"
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"testing"

	"skin-nexus/server/internal/signedpkg"
)

func makeUnsignedBundle(t *testing.T, dir, name, version string) string {
	t.Helper()
	manifest := `{"name":"` + name + `","version":"` + version + `","hookApi":2,"entrypoint":"index.js","permissions":[]}`
	out := filepath.Join(dir, name+"_"+version+"_unsigned.zip")
	writeZip(t, out, map[string]string{"plugin.json": manifest, "index.js": "skin.log('unsigned');"})
	return out
}

func TestInstallTrustLevels(t *testing.T) {
	m, kr, priv, _ := newInstallHost(t)
	dir := t.TempDir()

	// verified
	verified := makeSignedBundle(t, dir, "verified", "1.0.0", kr, priv, "test")
	insp, err := m.InspectBundle(verified)
	if err != nil || insp.Status != TrustVerified || insp.KeyID != "test" {
		t.Fatalf("inspect verified = %+v %v", insp, err)
	}
	res, err := m.InstallBundle(context.Background(), verified, InstallOptions{Source: "upload"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inspection.Status != TrustVerified {
		t.Fatalf("result = %+v", res)
	}
	st := m.statusOf("verified")
	if st.Trust != TrustVerified || st.Source != "upload" || st.Signer != "test" || st.Digest == "" {
		t.Fatalf("status provenance = %+v", st)
	}

	// unsigned: rejected by default, installs only with explicit acceptance
	unsigned := makeUnsignedBundle(t, dir, "plain", "1.0.0")
	insp, err = m.InspectBundle(unsigned)
	if err != nil || insp.Status != TrustUnsigned {
		t.Fatalf("inspect unsigned = %+v %v", insp, err)
	}
	if _, err := m.InstallBundle(context.Background(), unsigned, InstallOptions{Source: "upload"}); !errors.Is(err, ErrUnverifiedPlugin) {
		t.Fatalf("unsigned install = %v", err)
	}
	if m.statusOf("plain") != nil {
		t.Fatal("rejected bundle must not be installed")
	}
	res, err = m.InstallBundle(context.Background(), unsigned, InstallOptions{
		Source: "upload", AcceptUnverified: true, ExpectedZipSHA: insp.ZipSHA,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inspection.Status != TrustUnsigned || m.statusOf("plain").Trust != TrustUnsigned {
		t.Fatalf("unsigned install = %+v", res)
	}

	// unknown key: valid signature, key not pinned
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	unknown := makeSignedBundle(t, dir, "other", "1.0.0", kr, otherPriv, "other")
	insp, err = m.InspectBundle(unknown)
	if err != nil || insp.Status != TrustUnknownKey || insp.KeyID != "other" {
		t.Fatalf("inspect unknown = %+v %v", insp, err)
	}
	if _, err := m.InstallBundle(context.Background(), unknown, InstallOptions{Source: "upload"}); !errors.Is(err, ErrUnverifiedPlugin) {
		t.Fatalf("unknown key install = %v", err)
	}

	// invalid signature: tampered after signing
	tampered := makeSignedBundle(t, dir, "tampered", "1.0.0", kr, priv, "test")
	rewriteZipEntry(t, tampered, "index.js", "tampered")
	insp, err = m.InspectBundle(tampered)
	if err != nil || insp.Status != TrustInvalid {
		t.Fatalf("inspect tampered = %+v %v", insp, err)
	}
	if _, err := m.InstallBundle(context.Background(), tampered, InstallOptions{Source: "upload"}); !errors.Is(err, ErrUnverifiedPlugin) {
		t.Fatalf("tampered install = %v", err)
	}
	if _, err := m.InstallBundle(context.Background(), tampered, InstallOptions{
		Source: "upload", AcceptUnverified: true, ExpectedZipSHA: insp.ZipSHA,
	}); err != nil {
		t.Fatal(err)
	}
	if st := m.statusOf("tampered"); st.Trust != TrustInvalid {
		t.Fatalf("tampered status = %+v", st)
	}
}

func TestInstallDigestBinding(t *testing.T) {
	m, _, _, _ := newInstallHost(t)
	dir := t.TempDir()
	unsigned := makeUnsignedBundle(t, dir, "bind", "1.0.0")
	if _, err := m.InstallBundle(context.Background(), unsigned, InstallOptions{
		Source: "upload", AcceptUnverified: true, ExpectedZipSHA: "deadbeef",
	}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("digest mismatch = %v", err)
	}
}

func TestTrustPersistsAcrossReload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	kr := signedpkg.Keyring{"test": pub}
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	st := newMemPlatformStore()
	m := NewHost(RuntimeConfig{Dir: root, StateDir: state, HostVersion: "0.4.0", Keyring: kr, Store: st})
	dir := t.TempDir()
	z := makeSignedBundle(t, dir, "persist", "1.0.0", kr, priv, "test")
	if _, err := m.InstallBundle(context.Background(), z, InstallOptions{Source: "upload"}); err != nil {
		t.Fatal(err)
	}
	_ = m.Close()

	m2 := NewHost(RuntimeConfig{Dir: root, StateDir: state, HostVersion: "0.4.0", Keyring: kr, Store: st})
	t.Cleanup(func() { _ = m2.Close() })
	if err := m2.Load(root); err != nil {
		t.Fatal(err)
	}
	got := m2.statusOf("persist")
	if got == nil || got.Trust != TrustVerified || got.Source != "upload" || got.Signer != "test" {
		t.Fatalf("provenance after reload = %+v", got)
	}
}
