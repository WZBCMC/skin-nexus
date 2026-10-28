package adminapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarketInstallAuthAndOrigin(t *testing.T) {
	b, _ := newPluginBackend(t)
	body := strings.NewReader(`{"name":"demo"}`)

	// token path is refused even though b.auth accepts it
	stubSession(t, func(*http.Request) (int64, bool) { return 0, false })
	r := httptest.NewRequest(http.MethodPost, "/api/admin/v1/market/install", body)
	r.Header.Set("X-Admin-Token", "sekret")
	r.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("token path = %d", rec.Code)
	}

	stubSession(t, func(*http.Request) (int64, bool) { return 5, true })
	stubPerm(t, 1)
	r = httptest.NewRequest(http.MethodPost, "/api/admin/v1/market/install", strings.NewReader(`{"name":"demo"}`))
	r.Header.Set("Origin", "http://example.com")
	rec = httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin path = %d", rec.Code)
	}

	stubPerm(t, 2)
	r = httptest.NewRequest(http.MethodPost, "/api/admin/v1/market/install", strings.NewReader(`{"name":"demo"}`))
	rec = httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing origin = %d", rec.Code)
	}

	r = httptest.NewRequest(http.MethodPost, "/api/admin/v1/market/install", strings.NewReader(`{"name":"demo"}`))
	r.Header.Set("Origin", "http://example.com")
	rec = httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, r)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("same-origin rejected: %s", rec.Body.String())
	}
}
