package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const hostInfoRouteManifest = `{
  "name": "hostinfo",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["plugins:read", "http:routes"],
  "routes": [{"method": "GET", "path": "/info", "tier": "public"}]
}`

func TestHostInfoSnapshot(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "alpha", `{
	  "name":"alpha","version":"1.2.3","hookApi":2,"entrypoint":"index.js",
	  "permissions":[],"title":"Alpha"
	}`, "0;")
	writePlugin(t, root, "beta", `{
	  "name":"beta","version":"0.1.0","hookApi":2,"entrypoint":"index.js",
	  "permissions":[]
	}`, "0;")
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "alpha"); err != nil {
		t.Fatal(err)
	}

	info := m.HostInfo()
	if info.Version != "0.4.0" || info.HookAPI != 2 {
		t.Fatalf("version/hookApi = %q/%d", info.Version, info.HookAPI)
	}
	want := map[string]bool{CapHostIntrospect: true, CapSlots: true, CapForms: true, CapAvatarFilter: true}
	for _, c := range info.Capabilities {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Fatalf("missing capabilities: %v (got %v)", want, info.Capabilities)
	}
	joined := strings.Join(info.Slots, ",")
	if !strings.Contains(joined, "auth.register.after") || strings.Contains(joined, "app.nav.user") {
		t.Fatalf("slots = %v", info.Slots)
	}
	if len(info.Plugins) != 2 {
		t.Fatalf("plugins = %+v", info.Plugins)
	}
	byName := map[string]HostPluginInfo{}
	for _, p := range info.Plugins {
		byName[p.Name] = p
	}
	if !byName["alpha"].Enabled || byName["alpha"].Version != "1.2.3" || byName["alpha"].Title != "Alpha" {
		t.Fatalf("alpha = %+v", byName["alpha"])
	}
	if byName["beta"].Enabled {
		t.Fatalf("beta should be disabled: %+v", byName["beta"])
	}
}

func TestHostInfoRejectsUnknownSlot(t *testing.T) {
	root := t.TempDir()
	dir := writePlugin(t, root, "badslot", `{
	  "name":"badslot","version":"1.0.0","hookApi":2,"entrypoint":"index.js",
	  "permissions":["ui:slot","ui:assets"],
	  "ui":{"assets":"ui/","slots":[{"id":"app.nav.user","render":{"type":"iframe","src":"s.html"}}]}
	}`, "0;")
	if _, err := loadManifest(dir, "badslot"); err == nil || !strings.Contains(err.Error(), "unknown UI slot") {
		t.Fatalf("removed slot should be rejected, got %v", err)
	}
}

func TestHostInfoJSApi(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "hostinfo", hostInfoRouteManifest, `
skin.route("GET", "/info", function () {
  return {info: skin.host.info(), has: skin.host.hasPlugin("hostinfo")};
});
`)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "hostinfo"); err != nil {
		t.Fatal(err)
	}
	rec := doPluginReq(m, "GET", "/api/plugins/hostinfo/info", "")
	if rec.Code != 200 {
		t.Fatalf("route = %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Info HostInfo `json:"info"`
		Has  bool     `json:"has"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Info.Version != "0.4.0" || len(out.Info.Capabilities) == 0 || !out.Has {
		t.Fatalf("js info = %+v (has=%v)", out.Info, out.Has)
	}
	if len(out.Info.Plugins) != 1 || out.Info.Plugins[0].Name != "hostinfo" {
		t.Fatalf("plugins = %+v", out.Info.Plugins)
	}
}

func TestHostInfoRequiresPermission(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "noperm", `{
	  "name":"noperm","version":"1.0.0","hookApi":2,"entrypoint":"index.js","permissions":[]
	}`, `skin.host.info();`)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "noperm"); err == nil {
		t.Fatal("host.info without plugins:read must fail the load")
	}
}
