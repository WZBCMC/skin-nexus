package plugin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeBearerResolver struct{}

func (fakeBearerResolver) ResolveBearer(_ context.Context, token string) (*SessionIdentity, error) {
	switch token {
	case "good":
		return &SessionIdentity{ID: 9, Nickname: "launcher", Permission: 0}, nil
	case "admin":
		return &SessionIdentity{ID: 10, Nickname: "admin", Permission: 2}, nil
	case "disabled":
		return nil, &BearerError{Status: 403, Code: "auth.account_disabled", Message: "disabled"}
	}
	return nil, errors.New("bad token")
}

const bearerManifest = `{
  "name": "bearer",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["http:routes:bearer"],
  "routes": [{"method": "POST", "path": "/write", "tier": "bearer"}]
}`

const bearerScript = `skin.route("POST", "/write", function (r) { return {user: r.user, body: r.body}; });`

func newBearerHost(t *testing.T, resolver BearerResolver) *Manager {
	t.Helper()
	root := t.TempDir()
	writePlugin(t, root, "bearer", bearerManifest, bearerScript)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0", BearerResolver: resolver})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "bearer"); err != nil {
		t.Fatal(err)
	}
	return m
}

func bearerReq(m *Manager, auth, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/plugins/bearer/write", strings.NewReader(body))
	r.Host = "skin.test"
	r.Header.Set("Content-Type", "application/json")
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, r)
	return rec
}

func TestBearerRouteAuth(t *testing.T) {
	m := newBearerHost(t, fakeBearerResolver{})

	rec := bearerReq(m, "Bearer good", `{"x":1}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":9`) || !strings.Contains(rec.Body.String(), `"x":1`) {
		t.Fatalf("bearer route = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReq(m, "bearer good", `{}`)
	if rec.Code != 200 {
		t.Fatalf("lowercase scheme = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReq(m, "", `{}`)
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") == "" || !strings.Contains(rec.Body.String(), "auth.required") {
		t.Fatalf("missing token = %d %q %s", rec.Code, rec.Header().Get("WWW-Authenticate"), rec.Body.String())
	}

	rec = bearerReq(m, "Bearer nope", `{}`)
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "auth.invalid_token") {
		t.Fatalf("bad token = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReq(m, "Basic good", `{}`)
	if rec.Code != 401 {
		t.Fatalf("wrong scheme = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReq(m, "Bearer disabled", `{}`)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "auth.account_disabled") {
		t.Fatalf("disabled = %d %s", rec.Code, rec.Body.String())
	}
}

func TestBearerRouteWithoutResolver(t *testing.T) {
	m := newBearerHost(t, nil)
	rec := bearerReq(m, "Bearer good", `{}`)
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "auth.unavailable") {
		t.Fatalf("no resolver = %d %s", rec.Code, rec.Body.String())
	}
}
