package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSessionResolver struct{}

func (fakeSessionResolver) ResolveSession(_ context.Context, token string) (*SessionIdentity, error) {
	switch token {
	case "user-token":
		return &SessionIdentity{ID: 5, Nickname: "player", Permission: 0}, nil
	case "admin-token":
		return &SessionIdentity{ID: 6, Nickname: "admin", Permission: 1}, nil
	case "super-token":
		return &SessionIdentity{ID: 7, Nickname: "root", Permission: 2}, nil
	}
	return nil, errors.New("no session")
}

const routesV2Manifest = `{
  "name": "routes",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["http:routes", "http:routes:session", "http:routes:admin", "http:routes:superadmin"],
  "routes": [
    {"method": "GET", "path": "/ping", "tier": "public", "rateLimit": "2/m"},
    {"method": "GET", "path": "/me", "tier": "session"},
    {"method": "POST", "path": "/me", "tier": "session", "csrf": true},
    {"method": "GET", "path": "/admin", "tier": "admin"},
    {"method": "GET", "path": "/super", "tier": "superadmin"},
    {"method": "GET", "path": "/item/{id}", "tier": "session"},
    {"method": "GET", "path": "/files/{rest...}", "tier": "public"}
  ]
}`

const routesV2Script = `
skin.route("GET", "/ping", function () { return {pong: true}; });
skin.route("GET", "/me", function (r) { return {user: r.user, path: r.path, params: r.params, query: r.query}; });
skin.route("POST", "/me", function (r) { return {body: r.body, user: r.user}; });
skin.route("GET", "/admin", function (r) { return {admin: r.user.admin}; });
skin.route("GET", "/super", function () { return {super: true}; });
skin.route("GET", "/item/{id}", function (r) { return {id: r.params.id}; });
skin.route("GET", "/files/{rest...}", function (r) { return {rest: r.params.rest}; });
`

func newRoutesHost(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	writePlugin(t, root, "routes", routesV2Manifest, routesV2Script)
	m := NewHost(RuntimeConfig{
		Dir: root, HostVersion: "0.4.0", SessionResolver: fakeSessionResolver{}, SessionCookie: "skin_session",
	})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "routes"); err != nil {
		t.Fatal(err)
	}
	return m
}

func doPluginReq(m *Manager, method, target string, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Host = "skin.test"
	for _, c := range cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, r)
	return rec
}

func TestRouteTiersAndIdentity(t *testing.T) {
	m := newRoutesHost(t)
	if rec := doPluginReq(m, "GET", "/api/plugins/routes/me", ""); rec.Code != 401 {
		t.Fatalf("no session = %d", rec.Code)
	}
	rec := doPluginReq(m, "GET", "/api/plugins/routes/me?x=1", "", &http.Cookie{Name: "skin_session", Value: "user-token"})
	if rec.Code != 200 {
		t.Fatalf("session = %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	u := out["user"].(map[string]any)
	if u["id"].(float64) != 5 || out["path"] != "/me" || out["query"].(map[string]any)["x"] != "1" {
		t.Fatalf("out = %v", out)
	}

	if rec := doPluginReq(m, "GET", "/api/plugins/routes/admin", "", &http.Cookie{Name: "skin_session", Value: "user-token"}); rec.Code != 403 {
		t.Fatalf("normal admin = %d", rec.Code)
	}
	if rec := doPluginReq(m, "GET", "/api/plugins/routes/admin", "", &http.Cookie{Name: "skin_session", Value: "admin-token"}); rec.Code != 200 {
		t.Fatalf("admin = %d", rec.Code)
	}
	if rec := doPluginReq(m, "GET", "/api/plugins/routes/super", "", &http.Cookie{Name: "skin_session", Value: "admin-token"}); rec.Code != 403 {
		t.Fatalf("admin on super = %d", rec.Code)
	}
	if rec := doPluginReq(m, "GET", "/api/plugins/routes/super", "", &http.Cookie{Name: "skin_session", Value: "super-token"}); rec.Code != 200 {
		t.Fatalf("super = %d", rec.Code)
	}
}

func TestRouteParamsAndCatchAll(t *testing.T) {
	m := newRoutesHost(t)
	rec := doPluginReq(m, "GET", "/api/plugins/routes/item/42", "", &http.Cookie{Name: "skin_session", Value: "user-token"})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":"42"`) {
		t.Fatalf("item = %d %s", rec.Code, rec.Body.String())
	}
	rec = doPluginReq(m, "GET", "/api/plugins/routes/files/a/b/c", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"rest":"a/b/c"`) {
		t.Fatalf("rest = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRouteCSRFAndBody(t *testing.T) {
	m := newRoutesHost(t)
	user := &http.Cookie{Name: "skin_session", Value: "user-token"}
	if rec := doPluginReq(m, "POST", "/api/plugins/routes/me", `{"x":1}`, user); rec.Code != 403 {
		t.Fatalf("no csrf = %d", rec.Code)
	}
	rec := doPluginReq(m, "GET", "/api/plugins/routes/_csrf", "")
	if rec.Code != 200 {
		t.Fatalf("csrf = %d", rec.Code)
	}
	var tok struct{ Token string }
	_ = json.Unmarshal(rec.Body.Bytes(), &tok)
	cookies := rec.Result().Cookies()
	if tok.Token == "" || len(cookies) == 0 {
		t.Fatalf("csrf token/cookie missing: %+v %v", tok, cookies)
	}
	r := httptest.NewRequest("POST", "/api/plugins/routes/me", strings.NewReader(`{"x":1}`))
	r.Host = "skin.test"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", tok.Token)
	r.Header.Set("Origin", "http://skin.test")
	r.AddCookie(user)
	r.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Fatalf("csrf post = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"x":1`) {
		t.Fatalf("body not passed: %s", rec.Body.String())
	}
}

func TestRouteRateLimit(t *testing.T) {
	m := newRoutesHost(t)
	for i := 0; i < 2; i++ {
		if rec := doPluginReq(m, "GET", "/api/plugins/routes/ping", ""); rec.Code != 200 {
			t.Fatalf("ping %d = %d", i, rec.Code)
		}
	}
	rec := doPluginReq(m, "GET", "/api/plugins/routes/ping", "")
	if rec.Code != 429 {
		t.Fatalf("rate = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRouteBodyLimitAndMethod(t *testing.T) {
	m := newRoutesHost(t)
	user := &http.Cookie{Name: "skin_session", Value: "user-token"}
	// 64 KiB default; 80 KiB rejected before CSRF (CSRF comes first, so use csrf).
	rec := doPluginReq(m, "GET", "/api/plugins/routes/_csrf", "")
	cookies := rec.Result().Cookies()
	var tok struct{ Token string }
	_ = json.Unmarshal(rec.Body.Bytes(), &tok)
	r := httptest.NewRequest("POST", "/api/plugins/routes/me", strings.NewReader(strings.Repeat("x", 80<<10)))
	r.Host = "skin.test"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", tok.Token)
	r.AddCookie(user)
	r.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, r)
	if rec.Code != 413 {
		t.Fatalf("body = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doPluginReq(m, "POST", "/api/plugins/routes/ping", ""); rec.Code != 405 {
		t.Fatalf("method = %d", rec.Code)
	}
}
