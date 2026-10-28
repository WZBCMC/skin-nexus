package plugin

import (
	"strings"
	"testing"
)

const v2Manifest = `{
  "name": "notes",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["config", "store:docs", "http:routes:session", "ui:page", "ui:assets"],
  "minServer": ">=0.4.0 <1.0.0",
  "config": {"schema": {"type": "object", "properties": {"max": {"type": "integer", "default": 10}}}},
  "store": {"schemaVersion": 1, "collections": {"notes": {"maxRows": 100}}},
  "routes": [
    {"method": "GET", "path": "/notes", "tier": "session"},
    {"method": "POST", "path": "/notes", "tier": "session", "csrf": true}
  ],
  "ui": {
    "assets": "ui/",
    "pages": [{"id": "notes", "path": "/ext/notes/notes", "area": "user", "nav": {"label": "Notes"}, "render": {"type": "iframe", "src": "notes.html"}}],
    "slots": [{"id": "skinlib.detail.sidebar", "render": {"type": "iframe", "src": "slot.html"}}],
    "i18n": {"zh-CN": {"title": "笔记"}, "en": {"title": "Notes"}}
  }
}`

func TestManifestV2Valid(t *testing.T) {
	root := t.TempDir()
	dir := writePlugin(t, root, "notes", v2Manifest, "skin.log('hi');")
	info, err := loadManifest(dir, "notes")
	if err != nil {
		t.Fatalf("valid v2 manifest: %v", err)
	}
	if info.HookAPI != 2 || info.Runtime != "goja" {
		t.Fatalf("hook/runtime = %d/%q", info.HookAPI, info.Runtime)
	}
	if info.Config == nil || len(info.Routes) != 2 || info.UI == nil {
		t.Fatalf("v2 blocks not parsed: %+v", info.Manifest)
	}
	if info.Store == nil || info.Store.SchemaVersion != 1 {
		t.Fatalf("store = %+v", info.Store)
	}
	if err := info.CheckMinServer("0.4.0"); err != nil {
		t.Fatalf("minServer match: %v", err)
	}
	if err := info.CheckMinServer("0.3.0"); err == nil {
		t.Fatal("minServer should reject 0.3.0")
	}
}

func TestManifestV2Rejections(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"v1 with v2 field", `{"name":"notes","version":"1","hookApi":1,"entrypoint":"index.js","permissions":[],"config":{"schema":{"type":"object"}}}`, "hookApi 1 cannot declare v2"},
		{"bad runtime", `{"name":"notes","version":"1","hookApi":2,"runtime":"php","entrypoint":"index.js","permissions":[]}`, "runtime"},
		{"bad hook api", `{"name":"notes","version":"1","hookApi":3,"entrypoint":"index.js","permissions":[]}`, "hookApi 3"},
		{"bad tier", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["http:routes:session"],"routes":[{"method":"GET","path":"/a","tier":"root"}]}`, "invalid tier"},
		{"missing tier perm", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":[],"routes":[{"method":"GET","path":"/a","tier":"session"}]}`, "missing permission"},
		{"post without csrf", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["http:routes:session"],"routes":[{"method":"POST","path":"/a","tier":"session"}]}`, "must enable csrf"},
		{"bad method", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":[],"routes":[{"method":"TRACE","path":"/a"}]}`, "method"},
		{"bad slot", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["ui:slot"],"ui":{"slots":[{"id":"anywhere","render":{"type":"schema","schema":"x"}}]}}`, "unknown UI slot"},
		{"bad page path", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["ui:page"],"ui":{"pages":[{"id":"x","path":"/notes/x","area":"user","render":{"type":"schema","schema":"x"}}]}}`, "/ext/notes/"},
		{"iframe without assets", `{"name":"notes","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["ui:page"],"ui":{"pages":[{"id":"x","path":"/ext/notes/x","area":"user","render":{"type":"iframe","src":"x.html"}}]}}`, "ui.assets is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := writePlugin(t, root, "notes", tc.json, "skin.log('x');")
			_, err := loadManifest(dir, "notes")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want contains %q", err, tc.want)
			}
		})
	}
}
