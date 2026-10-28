package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// examplesDir points at the repo's shipped example plugins so a refactor can
// never silently break the documented samples.
func examplesDir() string {
	return filepath.Join("..", "..", "..", "examples", "plugins")
}

func TestShippedExamplesLoadAndRun(t *testing.T) {
	st := newMemPlatformStore()
	m := NewHost(RuntimeConfig{Dir: examplesDir(), HostVersion: "0.4.0", Store: st, SessionResolver: fakeSessionResolver{}})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(examplesDir()); err != nil {
		t.Fatal(err)
	}
	list := m.List()
	if len(list) != 6 {
		t.Fatalf("examples = %d, want 6 (%+v)", len(list), list)
	}
	for _, st := range list {
		if st.State != "disabled" {
			t.Fatalf("%s state = %s, want disabled by default", st.Name, st.State)
		}
	}

	ctx := context.Background()
	for _, name := range []string{"texture-cdn", "lifecycle-log", "config-greeter", "nav-links", "signup-guard", "player-notes"} {
		if err := m.Enable(ctx, name); err != nil {
			t.Fatalf("enable %s: %v", name, err)
		}
	}

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plugins/config-greeter/hello", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Hello from a plugin") {
		t.Fatalf("config-greeter route = %d %s", rec.Code, rec.Body.String())
	}
	if err := m.CheckGuard(ctx, "user.registration.check", EventPayload{
		Data: map[string]any{"email": "x@blocked.example"},
	}); err == nil {
		t.Fatal("signup-guard should deny blocked domain")
	}
	if ui := m.UI(); len(ui) != 2 { // nav-links + player-notes
		t.Fatalf("example UI = %+v", ui)
	}

	got := m.TextureURL(ctx, "abc123", "http://host/textures/abc123", "skin")
	if got != "https://cdn.example.com/textures/abc123" {
		t.Fatalf("texture-cdn rewrite = %s", got)
	}
	// capes stay on the host (example filters skins only)
	if got := m.TextureURL(ctx, "abc123", "http://host/textures/abc123", "cape"); got != "http://host/textures/abc123" {
		t.Fatalf("cape URL = %s", got)
	}

	m.EmitUserLifecycle(ctx, UserLifecycleEvent{Kind: "registered", UserID: 3})
	// events are async; give the worker a moment then check the log buffer
	deadline := ctx
	_ = deadline
	waitForLog(t, m, "lifecycle-log", "lifecycle registered uid=3")

	for _, name := range []string{"texture-cdn", "lifecycle-log", "config-greeter", "nav-links", "signup-guard", "player-notes"} {
		if err := m.Disable(ctx, name); err != nil {
			t.Fatalf("disable %s: %v", name, err)
		}
	}
	if got := m.TextureURL(ctx, "abc123", "http://host/textures/abc123", "skin"); got != "http://host/textures/abc123" {
		t.Fatalf("after disable URL = %s", got)
	}
}

func waitForLog(t *testing.T, m *Manager, name, want string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		m.mu.RLock()
		inst := m.instances[name]
		var found bool
		if inst != nil && inst.rt != nil {
			inst.rt.mu.Lock()
			for _, l := range inst.rt.logs {
				if contains(l, want) {
					found = true
				}
			}
			inst.rt.mu.Unlock()
		}
		m.mu.RUnlock()
		if found {
			return
		}
		sleepMS(20)
	}
	t.Fatalf("log line %q not found for %s", want, name)
}
