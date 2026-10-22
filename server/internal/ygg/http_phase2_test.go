package ygg

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Phase-2 additive tests (frozen contract assertions in http_test.go stay
// untouched): metadata exposes feature.openid_configuration_url only when
// configured (analysis/oidc-janus-contract.md §6 item 1).

func TestMetadataOpenIDConfigURL(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := do(t, api.Handler(), "GET", "/api/yggdrasil/", "", nil)
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	meta, _ := doc["meta"].(map[string]any)
	if _, ok := meta["feature.openid_configuration_url"]; ok {
		t.Fatalf("feature.openid_configuration_url must be absent when unconfigured")
	}

	api2, _ := newTestAPI(t)
	api2.cfg.OpenIDConfigurationURL = "https://skin.test/api/yggdrasil/.well-known/openid-configuration"
	rec2 := do(t, api2.Handler(), "GET", "/api/yggdrasil/", "", nil)
	var doc2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &doc2); err != nil {
		t.Fatal(err)
	}
	meta2, _ := doc2["meta"].(map[string]any)
	got, _ := meta2["feature.openid_configuration_url"].(string)
	if !strings.HasSuffix(got, "/.well-known/openid-configuration") {
		t.Fatalf("feature.openid_configuration_url = %q", got)
	}
}

// TestJoinRejectsForeignPlayer pins the ownership gate added with the
// dual-state token work: a token may only join as a player it owns and that
// matches its selected profile (legacy player.not-match semantics).
func TestJoinRejectsForeignPlayer(t *testing.T) {
	api, st := newTestAPI(t)
	// Issue a token for user 1 (player "Steve" id 1 per memStore fixture).
	rec := do(t, api.Handler(), "POST", "/api/yggdrasil/authserver/authenticate",
		`{"username":"u@example.com","password":"pw"}`, map[string]string{"Content-Type": "application/json"})
	var tok struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil {
		t.Fatal(err)
	}
	_ = st

	reqBody := `{"accessToken":"` + tok.AccessToken + `","selectedProfile":{"id":"ffffccccddddeeeebbbbaaaa00001111"},"serverId":"x"}`
	jr := do(t, api.Handler(), "POST", "/api/yggdrasil/sessionserver/session/minecraft/join",
		reqBody, map[string]string{"Content-Type": "application/json"})
	if jr.Code != 204 {
		t.Fatalf("join status %d (silent 204 semantics preserved)", jr.Code)
	}
	// A join as a foreign uuid must not create a live session.
	hr := do(t, api.Handler(), "GET", "/api/yggdrasil/sessionserver/session/minecraft/hasJoined?username=Steve&serverId=x", "", nil)
	if hr.Code != 204 {
		t.Fatalf("hasJoined after foreign join = %d (want 204)", hr.Code)
	}
}

var _ = httptest.NewRecorder
