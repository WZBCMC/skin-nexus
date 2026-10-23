package adminapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"skin-nexus/server/internal/store"
)

// stubSession swaps the session-admin resolver for one test (restored via
// t.Cleanup) — the store-backed default needs MySQL and lives behind the
// same seam style as oidc.sessionCookie.
func stubSession(t *testing.T, fn func(*http.Request) (int64, bool)) {
	t.Helper()
	old := sessionAdmin
	sessionAdmin = func(r *http.Request, _ *store.Mysql) (int64, bool) { return fn(r) }
	t.Cleanup(func() { sessionAdmin = old })
}

func authServed(t *testing.T, b *Backend, r *http.Request) (served bool, actor int64, rec *httptest.ResponseRecorder) {
	t.Helper()
	rec = httptest.NewRecorder()
	h := b.auth(func(w http.ResponseWriter, r *http.Request) {
		served = true
		actor = actorUID(r)
		w.WriteHeader(http.StatusOK)
	})
	h.ServeHTTP(rec, r)
	return
}

func TestAuthTokenPasses(t *testing.T) {
	b := &Backend{AdminTok: "sekret"}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/overview", nil)
	r.Header.Set("X-Admin-Token", "sekret")
	served, actor, rec := authServed(t, b, r)
	if !served || rec.Code != http.StatusOK {
		t.Fatalf("valid token: served=%v code=%d", served, rec.Code)
	}
	if actor != 0 { // token path stays the system/API actor
		t.Fatalf("token path actor uid = %d, want 0", actor)
	}
}

func TestAuthTokenWrongForbidden(t *testing.T) {
	b := &Backend{AdminTok: "sekret"}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/overview", nil)
	r.Header.Set("X-Admin-Token", "nope")
	served, _, rec := authServed(t, b, r)
	if served || rec.Code != http.StatusForbidden {
		t.Fatalf("wrong token: served=%v code=%d, want 403", served, rec.Code)
	}
}

func TestAuthTokenPathLockedWithoutConfig(t *testing.T) {
	b := &Backend{AdminTok: ""} // empty SKIN_ADMIN_TOKEN = token path locked
	r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/overview", nil)
	served, _, rec := authServed(t, b, r)
	if served || rec.Code != http.StatusForbidden {
		t.Fatalf("empty token config: served=%v code=%d, want 403", served, rec.Code)
	}
}

func TestAuthSessionAdminPasses(t *testing.T) {
	stubSession(t, func(r *http.Request) (int64, bool) {
		if c, err := r.Cookie(sessionCookieName); err != nil || c.Value == "" {
			t.Fatalf("session lookup without %s cookie", sessionCookieName)
		}
		return 7, true
	})
	b := &Backend{AdminTok: "sekret"}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/overview", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	served, actor, rec := authServed(t, b, r)
	if !served || rec.Code != http.StatusOK {
		t.Fatalf("admin session: served=%v code=%d", served, rec.Code)
	}
	if actor != 7 {
		t.Fatalf("session actor uid = %d, want 7 (audit attribution)", actor)
	}
}

func TestAuthNonAdminSessionForbidden(t *testing.T) {
	stubSession(t, func(*http.Request) (int64, bool) { return 0, false })
	b := &Backend{AdminTok: "sekret"}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/v1/overview", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok"})
	served, _, rec := authServed(t, b, r)
	if served || rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin session: served=%v code=%d, want 403 (same as wrong token)", served, rec.Code)
	}
}
