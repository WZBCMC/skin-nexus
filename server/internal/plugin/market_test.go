package plugin

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"skin-nexus/server/internal/signedpkg"
)

func TestMarketRegistrySignatureAndCatalog(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	kr := signedpkg.Keyring{"test": pub}
	reg := &MarketRegistry{
		Schema: 1, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		KeyID:     "test", Packages: []MarketPackage{{Name: "demo", Version: "2.0.0"}},
	}
	payload, err := (&Manager{}).marketRegistryPayload(reg)
	if err != nil {
		t.Fatal(err)
	}
	reg.Signature = base64.StdEncoding.EncodeToString(signedpkg.Sign(priv, payload))
	sig, _ := signedpkg.DecodeSignature(reg.Signature)
	if err := kr.Verify("test", payload, sig); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	writePlugin(t, root, "demo", `{"name":"demo","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":[]}`, "skin.log('x');")
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0", Keyring: kr, MarketURL: "https://market.example/registry.json"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	m.marketCache, m.marketFetched = reg, time.Now()
	list, err := m.MarketCatalog(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].CanUpdate || list[0].Installed != "1.0.0" {
		t.Fatalf("catalog = %+v", list)
	}
}
