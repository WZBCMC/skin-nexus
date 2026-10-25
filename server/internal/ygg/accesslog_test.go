package ygg

// Handler-level observability tests (2026-10-26 观测面): the traffic
// logger must record the right kind/outcome/source on the frozen
// endpoints WITHOUT changing any wire response.

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"skin-nexus/server/internal/accesslog"
)

type memSink struct{ entries []accesslog.Entry }

func (m *memSink) put(_ context.Context, e accesslog.Entry) error {
	m.entries = append(m.entries, e)
	return nil
}

func (m *memSink) find(kind string) *accesslog.Entry {
	for i := range m.entries {
		if m.entries[i].Kind == kind {
			return &m.entries[i]
		}
	}
	return nil
}

func newLoggedAPI(t *testing.T) (*API, *memSink) {
	t.Helper()
	api, _ := newTestAPI(t)
	sink := &memSink{}
	api.SetAccess(accesslog.New(sink.put))
	return api, sink
}

func TestAccessLogProfileAndHasJoined(t *testing.T) {
	api, sink := newLoggedAPI(t)

	// Unknown valid-hex profile (fallback off) → miss / src=none.
	rec := do(t, api.Handler(), "GET",
		"/api/yggdrasil/sessionserver/session/minecraft/profile/000000000000000000000000000000ff",
		"", map[string]string{"User-Agent": "TestLauncher/9.9"})
	if rec.Code != 204 {
		t.Fatalf("profile miss must 204, got %d", rec.Code)
	}
	e := sink.find(accesslog.YggProfile)
	if e == nil || e.Outcome != accesslog.OutcomeMiss || !strings.Contains(e.Detail, "src=none") {
		t.Fatalf("profile miss entry = %+v", e)
	}
	if e.Who != "000000000000000000000000000000ff" || e.UA != "TestLauncher/9.9" || e.IP == "" {
		t.Errorf("profile entry identity wrong: %+v", e)
	}

	// Local profile hit → ok / src=local.
	sink.entries = nil
	rec = do(t, api.Handler(), "GET",
		"/api/yggdrasil/sessionserver/session/minecraft/profile/"+testUUID, "", nil)
	if rec.Code != 200 {
		t.Fatalf("local profile must 200, got %d", rec.Code)
	}
	e = sink.find(accesslog.YggProfile)
	if e == nil || e.Outcome != accesslog.OutcomeOK || !strings.Contains(e.Detail, "src=local") {
		t.Fatalf("profile local entry = %+v", e)
	}

	// hasJoined with no session → miss / src=none (and no crash).
	sink.entries = nil
	rec = do(t, api.Handler(), "GET",
		"/api/yggdrasil/sessionserver/session/minecraft/hasJoined?username=Notch&serverId=abc", "", nil)
	if rec.Code != 204 {
		t.Fatalf("hasJoined miss must 204, got %d", rec.Code)
	}
	e = sink.find(accesslog.YggHasJoined)
	if e == nil || e.Outcome != accesslog.OutcomeMiss || e.Who != "Notch" ||
		!strings.Contains(e.Detail, "src=none") || !strings.Contains(e.Detail, "serverId=abc") {
		t.Fatalf("hasJoined entry = %+v", e)
	}
}

func TestAccessLogAuthenticateOutcomes(t *testing.T) {
	api, sink := newLoggedAPI(t)

	// Bad credentials → fail / invalid_credentials (who = attempted name).
	do(t, api.Handler(), "POST", "/api/yggdrasil/authserver/authenticate",
		`{"username":"ghost","password":"wrong","clientToken":"ct-1"}`, nil)
	e := sink.find(accesslog.YggAuthenticate)
	if e == nil || e.Outcome != accesslog.OutcomeFail ||
		e.Who != "ghost" || !strings.Contains(e.Detail, "invalid_credentials") {
		t.Fatalf("authenticate fail entry = %+v", e)
	}

	// Malformed body →400: client error, no log line (not operator signal).
	sink.entries = nil
	do(t, api.Handler(), "POST", "/api/yggdrasil/authserver/authenticate", `{`, nil)
	if sink.find(accesslog.YggAuthenticate) != nil {
		t.Error("400 malformed body must not produce a traffic line")
	}
}

func TestAccessLogJoinOutcomes(t *testing.T) {
	api, sink := newLoggedAPI(t)

	// Garbage token → fail / invalid_token with uuid + serverId in detail.
	do(t, api.Handler(), "POST", "/api/yggdrasil/sessionserver/session/minecraft/join",
		`{"accessToken":"bogus","selectedProfile":"`+testUUID+`","serverId":"srv-1"}`, nil)
	e := sink.find(accesslog.YggJoin)
	if e == nil || e.Outcome != accesslog.OutcomeFail ||
		!strings.Contains(e.Detail, "invalid_token") ||
		!strings.Contains(e.Detail, "serverId=srv-1") {
		t.Fatalf("join entry = %+v", e)
	}
}

func TestAccessLogNilSafeOnPlainAPI(t *testing.T) {
	// An API without SetAccess (the default everywhere tests run) must
	// behave exactly as before — this very request proves it.
	api, _ := newTestAPI(t)
	rec := do(t, api.Handler(), "GET",
		"/api/yggdrasil/sessionserver/session/minecraft/profile/"+testUUID, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unlogged profile = %d", rec.Code)
	}
}
