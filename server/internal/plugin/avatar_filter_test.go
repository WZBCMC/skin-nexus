package plugin

import (
	"context"
	"strings"
	"testing"
)

const avatarManifest = `{
  "name": "cdnavatar",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["media:avatar"],
  "media": {"avatarHosts": ["cdn.example.com", "*.img.example.net"]}
}`

const avatarScript = `
skin.hooks.onAvatarFilter(function (a) {
  return "https://cdn.example.com/avatar/" + a.hash + "/" + a.size + (a.threeD ? "/3d" : "") + ".png";
});
`

func TestAvatarURLRedirect(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "cdnavatar", avatarManifest, avatarScript)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "cdnavatar"); err != nil {
		t.Fatal(err)
	}
	u, ok := m.AvatarURL(context.Background(), "deadbeef", 128, true)
	if !ok || u != "https://cdn.example.com/avatar/deadbeef/128/3d.png" {
		t.Fatalf("avatar url = %q ok=%v", u, ok)
	}
	if _, ok := m.AvatarURL(context.Background(), "", 128, false); ok {
		t.Fatal("empty hash must never redirect")
	}
	if err := m.Disable(context.Background(), "cdnavatar"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.AvatarURL(context.Background(), "deadbeef", 128, false); ok {
		t.Fatal("disabled plugin must not redirect")
	}
}

func TestAvatarURLHostAllowlist(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "badhost", `{
	  "name":"badhost","version":"1.0.0","hookApi":2,"entrypoint":"index.js",
	  "permissions":["media:avatar"],
	  "media":{"avatarHosts":["cdn.example.com"]}
	}`, `skin.hooks.onAvatarFilter(function () { return "https://evil.example.org/x.png"; });`)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "badhost"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.AvatarURL(context.Background(), "deadbeef", 64, false); ok {
		t.Fatal("undeclared host must not redirect")
	}
}

func TestMediaManifestValidation(t *testing.T) {
	cases := []struct{ name, manifest string }{
		{"missing permission", `{"name":"mx","version":"1","hookApi":2,"entrypoint":"index.js","permissions":[],"media":{"avatarHosts":["cdn.example.com"]}}`},
		{"empty hosts", `{"name":"mx","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["media:avatar"],"media":{"avatarHosts":[]}}`},
		{"scheme", `{"name":"mx","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["media:avatar"],"media":{"avatarHosts":["https://cdn.example.com"]}}`},
		{"ip", `{"name":"mx","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["media:avatar"],"media":{"avatarHosts":["10.0.0.1"]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			dir := writePlugin(t, root, "mx", c.manifest, "0;")
			if _, err := loadManifest(dir, "mx"); err == nil {
				t.Fatal("manifest should be rejected")
			}
		})
	}
}

func TestValidAvatarURL(t *testing.T) {
	hosts := []string{"cdn.example.com", "*.img.example.net"}
	cases := []struct {
		url  string
		want bool
	}{
		{"https://cdn.example.com/a.png", true},
		{"https://x.img.example.net/a.png", true},
		{"https://img.example.net/a.png", false},
		{"http://cdn.example.com/a.png", false},
		{"https://user:pass@cdn.example.com/a.png", false},
		{"https://cdn.example.com:8443/a.png", false},
		{"https://cdn.example.com:443/a.png", true},
		{"https://10.0.0.1/a.png", false},
		{"https://evil.example.org/a.png", false},
		{"https://cdn.example.com/" + strings.Repeat("a", maxAvatarURLLen), false},
	}
	for _, c := range cases {
		if got := validAvatarURL(c.url, hosts); got != c.want {
			t.Fatalf("validAvatarURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
	if validAvatarURL("https://cdn.example.com/a.png", nil) {
		t.Fatal("no declared hosts must reject")
	}
}
