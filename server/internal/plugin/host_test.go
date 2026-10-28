package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type memState struct {
	mu sync.Mutex
	m  map[string]string
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func sleepMS(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func newMemState() *memState { return &memState{m: map[string]string{}} }

func (s *memState) GetOption(_ context.Context, k string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	return v, ok, nil
}

func (s *memState) SetOption(_ context.Context, k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}

// putPlugin writes a plugin dir and returns the host rooted at parent.
func putPlugin(t *testing.T, name, manifest, script string) string {
	t.Helper()
	root := t.TempDir()
	dir := writePlugin(t, root, name, manifest, script)
	_ = dir
	return root
}

func newTestHost(t *testing.T, root string) *Manager {
	t.Helper()
	m := NewHost(RuntimeConfig{
		Dir:          root,
		HostVersion:  "0.3.0",
		State:        newMemState(),
		HookTimeout:  80 * time.Millisecond,
		AuthTimeout:  40 * time.Millisecond,
		RouteTimeout: 200 * time.Millisecond,
	})
	t.Cleanup(func() { _ = m.Close() })
	return m
}

const cdnManifest = `{
  "name": "cdn",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["textures:read", "http:routes"],
  "title": "CDN mirror",
  "author": "YMZS"
}`

const cdnScript = `
skin.hooks.onTextureURLFilter(function (e) {
  if (e.type !== "skin") { return undefined; }
  return "https://cdn.example.com/textures/" + e.hash;
});
`

func TestLoadDefaultDisabledAndEnable(t *testing.T) {
	root := putPlugin(t, "cdn", cdnManifest, cdnScript)
	m := newTestHost(t, root)
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	list := m.List()
	if len(list) != 1 || list[0].State != "disabled" || list[0].Enabled {
		t.Fatalf("expected disabled by default, got %+v", list)
	}
	if list[0].Title != "CDN mirror" || list[0].Author != "YMZS" {
		t.Fatalf("metadata = %+v", list[0])
	}

	// disabled: URL untouched
	if got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "skin"); got != "http://x/textures/abc" {
		t.Fatalf("disabled filter changed URL: %s", got)
	}

	if err := m.Enable(context.Background(), "cdn"); err != nil {
		t.Fatal(err)
	}
	got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "skin")
	if got != "https://cdn.example.com/textures/abc" {
		t.Fatalf("enabled filter URL = %s", got)
	}
	// cape untouched (filter returns undefined for non-skin)
	if got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "cape"); got != "http://x/textures/abc" {
		t.Fatalf("cape URL = %s", got)
	}

	if err := m.Disable(context.Background(), "cdn"); err != nil {
		t.Fatal(err)
	}
	if got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "skin"); got != "http://x/textures/abc" {
		t.Fatalf("after disable URL = %s", got)
	}
}

func TestRouteDispatch(t *testing.T) {
	manifest := strings.Replace(cdnManifest, `"title": "CDN mirror"`, `"routes": [{"method": "GET", "path": "/ping", "tier": "public"}], "title": "CDN mirror"`, 1)
	script := cdnScript + `
skin.route("GET", "/ping", function (req) {
  return { status: 200, body: { pong: req.query.q || "none" } };
});
`
	root := putPlugin(t, "cdn", manifest, script)
	m := newTestHost(t, root)
	_ = m.Load(root)
	_ = m.Enable(context.Background(), "cdn")

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plugins/cdn/ping?q=1", nil))
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["pong"] != "1" {
		t.Fatalf("body = %v", body)
	}

	// disabled plugin -> 404
	_ = m.Disable(context.Background(), "cdn")
	rec = httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plugins/cdn/ping", nil))
	if rec.Code != 404 {
		t.Fatalf("disabled route code = %d", rec.Code)
	}

	// wrong method -> 405 (re-enable first: disabled is a true 404)
	if err := m.Enable(context.Background(), "cdn"); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/plugins/cdn/ping", nil))
	if rec.Code != 405 {
		t.Fatalf("post code = %d", rec.Code)
	}
}

func TestPermissionGatingFailsLoad(t *testing.T) {
	manifest := strings.Replace(cdnManifest, `["textures:read", "http:routes"]`, `["http:routes"]`, 1)
	script := `skin.hooks.onTextureURLFilter(function (e) { return e.url; });`
	root := putPlugin(t, "cdn", manifest, script)
	m := newTestHost(t, root)
	_ = m.Load(root)
	if err := m.Enable(context.Background(), "cdn"); err == nil {
		t.Fatal("expected permission rejection")
	}
	list := m.List()
	if list[0].State != "error" || !strings.Contains(list[0].LastError, "permission") {
		t.Fatalf("state = %+v", list[0])
	}
}

func TestTimeoutFailOpen(t *testing.T) {
	script := `skin.hooks.onTextureURLFilter(function () { while (true) {} });`
	root := putPlugin(t, "cdn", cdnManifest, script)
	m := newTestHost(t, root)
	_ = m.Load(root)
	if err := m.Enable(context.Background(), "cdn"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "skin")
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout did not interrupt")
	}
	if got != "http://x/textures/abc" {
		t.Fatalf("fail-open violated: %s", got)
	}
	list := m.List()
	if !strings.Contains(list[0].LastError, "timeout") {
		t.Fatalf("lastError = %q", list[0].LastError)
	}
}

func TestReloadPicksNewScript(t *testing.T) {
	root := putPlugin(t, "cdn", cdnManifest, cdnScript)
	m := newTestHost(t, root)
	_ = m.Load(root)
	_ = m.Enable(context.Background(), "cdn")
	// rewrite entrypoint to a no-op filter
	if err := os.WriteFile(root+"/cdn/index.js", []byte(`skin.hooks.onTextureURLFilter(function(){ return undefined; });`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(context.Background(), "cdn"); err != nil {
		t.Fatal(err)
	}
	if got := m.TextureURL(context.Background(), "abc", "http://x/textures/abc", "skin"); got != "http://x/textures/abc" {
		t.Fatalf("reload did not apply: %s", got)
	}
}

func TestEventDelivery(t *testing.T) {
	script := `skin.hooks.onUserLifecycle(function (e) { skin.log("lifecycle:" + e.kind + ":" + e.userId); });`
	manifest := strings.Replace(cdnManifest, `["textures:read", "http:routes"]`, `["auth:hooks"]`, 1)
	root := putPlugin(t, "cdn", manifest, script)
	m := newTestHost(t, root)
	_ = m.Load(root)
	_ = m.Enable(context.Background(), "cdn")
	m.EmitUserLifecycle(context.Background(), UserLifecycleEvent{Kind: "registered", UserID: 7})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		rt := m.instances["cdn"].rt
		var found bool
		if rt != nil {
			rt.mu.Lock()
			for _, l := range rt.logs {
				if strings.Contains(l, "lifecycle:registered:7") {
					found = true
				}
			}
			rt.mu.Unlock()
		}
		m.mu.RUnlock()
		if found {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("event not delivered")
}
