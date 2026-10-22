package ygg

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Metadata is the ALI discovery document (live-capture shape,
// docs/phase1-dev-plan.md §1.1).
type Metadata struct {
	Meta struct {
		ServerName           string            `json:"serverName"`
		ImplementationName   string            `json:"implementationName"`
		ImplementationVersion string           `json:"implementationVersion"`
		Links                map[string]string `json:"links"`
		Features             map[string]bool   `json:"-"` // serialized flat below
	} `json:"meta"`
	SkinDomains        []string `json:"skinDomains"`
	SignaturePublicKey string   `json:"signaturePublickey"`
}

// MetadataConfig is deployment configuration for the metadata document.
type MetadataConfig struct {
	ServerName string
	Version    string
	Homepage   string
	Register   string
	SkinDomains []string
}

// API serves the Yggdrasil HTTP surface mounted at /api/yggdrasil.
type API struct {
	svc *Service
	cfg MetadataConfig
}

func NewAPI(svc *Service, cfg MetadataConfig) *API {
	return &API{svc: svc, cfg: cfg}
}

// Handler returns the route table. Routes are rooted at the /api/yggdrasil
// prefix — the compatibility surface itself (legacy clients hardcode it).
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/yggdrasil/{$}", a.handleMetadata)
	mux.HandleFunc("POST /api/yggdrasil/authserver/authenticate", a.handleAuthenticate)
	mux.HandleFunc("POST /api/yggdrasil/authserver/refresh", a.handleRefresh)
	mux.HandleFunc("POST /api/yggdrasil/authserver/validate", a.handleValidate)
	mux.HandleFunc("POST /api/yggdrasil/authserver/invalidate", a.handleInvalidate)
	mux.HandleFunc("POST /api/yggdrasil/authserver/signout", a.handleSignout)
	mux.HandleFunc("POST /api/yggdrasil/sessionserver/session/minecraft/join", a.handleJoin)
	mux.HandleFunc("GET /api/yggdrasil/sessionserver/session/minecraft/hasJoined", a.handleHasJoined)
	mux.HandleFunc("GET /api/yggdrasil/sessionserver/session/minecraft/profile/{uuid}", a.handleProfile)
	mux.HandleFunc("POST /api/yggdrasil/api/profiles/minecraft", a.handleLookupBulk)
	mux.HandleFunc("GET /api/yggdrasil/api/users/profiles/minecraft/{name}", a.handleLookupName)
	return mux
}

// ---- responses ----

// writeError emits the legacy error body {error,errorMessage,cause?}.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"error": code, "errorMessage": msg}
	_ = json.NewEncoder(w).Encode(body)
}

// noContent emits 204 — the "not found / done" signal clients expect.
// Callers must never turn these paths into 4xx/5xx (clients treat those as
// network errors, docs/phase1-dev-plan.md §1.3).
func noContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// checkJSON enforces the legacy content-type behavior: a non-JSON POST gets
// 200 + error body (yggdrasil-connect CheckContentType semantics).
func checkJSON(r *http.Request, w http.ResponseWriter) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		writeError(w, http.StatusOK, "UnsupportedMediaType", "Content-Type must be application/json")
		return false
	}
	return true
}

// ---- metadata ----

func (a *API) handleMetadata(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	b.WriteString(`{"meta":{"serverName":`)
	writeJSONString(&b, a.cfg.ServerName)
	b.WriteString(`,"implementationName":"SkinNexus","implementationVersion":`)
	writeJSONString(&b, a.cfg.Version)
	b.WriteString(`,"links":{"homepage":`)
	writeJSONString(&b, a.cfg.Homepage)
	b.WriteString(`,"register":`)
	writeJSONString(&b, a.cfg.Register)
	b.WriteString(`},"feature.non_email_login":true},"skinDomains":[`)
	for i, d := range a.cfg.SkinDomains {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSONString(&b, d)
	}
	b.WriteString(`],"signaturePublickey":`)
	writeJSONString(&b, a.svc.PublicKeyPEM())
	b.WriteString(`}`)
	w.Header().Set("Content-Type", "application/json")
	// Authlib-injector discovery: relative or absolute location of this API.
	w.Header().Set("X-Authlib-Injector-API-Location", apiLocation(r))
	_, _ = w.Write([]byte(b.String()))
}

// apiLocation derives the ALI value from the request so the deployment
// works under any host (matches the live legacy header shape).
func apiLocation(r *http.Request) string {
	base := "https://" + r.Host
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		base = "http://" + r.Host
	}
	prefix := strings.TrimSuffix(r.URL.Path, "/")
	return base + prefix
}

// writeJSONString appends a quoted JSON string without allocating an encoder.
func writeJSONString(b *strings.Builder, s string) {
	enc, _ := json.Marshal(s)
	b.Write(enc)
}

// ---- authserver ----

func (a *API) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		ClientToken string `json:"clientToken"`
		RequestUser bool   `json:"requestUser"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "IllegalArgumentException", "invalid JSON body")
		return
	}
	t, players, err := a.svc.Authenticate(r.Context(), req.Username, req.Password, req.ClientToken, req.RequestUser)
	if err != nil {
		writeError(w, http.StatusForbidden, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
		return
	}
	writeTokenResponse(w, t, players, req.RequestUser)
}

func (a *API) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		AccessToken  string `json:"accessToken"`
		ClientToken  string `json:"clientToken"`
		RequestUser  bool   `json:"requestUser"`
		SelectedProfile struct {
			ID string `json:"id"`
		} `json:"selectedProfile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "IllegalArgumentException", "invalid JSON body")
		return
	}
	var pid int64
	t, players, err := a.svc.Refresh(r.Context(), req.AccessToken, req.ClientToken, pid)
	if err != nil {
		writeError(w, http.StatusForbidden, "ForbiddenOperationException", "Invalid token.")
		return
	}
	writeTokenResponse(w, t, players, req.RequestUser)
}

func (a *API) handleValidate(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "IllegalArgumentException", "invalid JSON body")
		return
	}
	if err := a.svc.Validate(r.Context(), req.AccessToken, req.ClientToken); err != nil {
		writeError(w, http.StatusForbidden, "ForbiddenOperationException", "Invalid token.")
		return
	}
	noContent(w)
}

func (a *API) handleInvalidate(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	a.svc.Invalidate(r.Context(), req.AccessToken)
	noContent(w) // always 204
}

func (a *API) handleSignout(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	a.svc.SignOut(r.Context(), req.Username, req.Password)
	noContent(w) // always 204
}

// ---- sessionserver ----

func (a *API) handleJoin(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var req struct {
		AccessToken    string `json:"accessToken"`
		SelectedProfile struct {
			ID string `json:"id"`
		} `json:"selectedProfile"`
		ServerID string `json:"serverId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "IllegalArgumentException", "invalid JSON body")
		return
	}
	err := a.svc.Join(r.Context(), req.AccessToken, req.SelectedProfile.ID, req.ServerID, clientIP(r))
	if err != nil {
		// Legacy behavior: join failures are silent (204); the game then
		// fails the hasJoined wait. Only parameter errors are reported.
		noContent(w)
		return
	}
	noContent(w)
}

func (a *API) handleHasJoined(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := a.svc.HasJoined(r.Context(), q.Get("username"), q.Get("serverId"), q.Get("ip"))
	if err != nil {
		noContent(w) // unknown / expired / mismatch: 204, never 4xx
		return
	}
	writeJSON(w, p)
}

func (a *API) handleProfile(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	withSig := r.URL.Query().Get("unsigned") == "false"
	p, err := a.svc.ProfileByUUID(r.Context(), uuid, withSig)
	if err != nil {
		noContent(w) // unknown profile: empty body
		return
	}
	if !withSig {
		for i := range p.Properties {
			p.Properties[i].Signature = ""
		}
	}
	writeJSON(w, p)
}

// ---- lookup ----

func (a *API) handleLookupBulk(w http.ResponseWriter, r *http.Request) {
	if !checkJSON(r, w) {
		return
	}
	var names []string
	if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
		writeError(w, http.StatusBadRequest, "IllegalArgumentException", "invalid JSON body")
		return
	}
	if len(names) > 100 { // legacy bound
		names = names[:100]
	}
	players := a.svc.LookupBulk(r.Context(), names)
	type entry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]entry, 0, len(players))
	for _, p := range players {
		out = append(out, entry{ID: p.UUID, Name: p.Name})
	}
	writeJSON(w, out)
}

func (a *API) handleLookupName(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.LookupByName(r.Context(), r.PathValue("name"))
	if err != nil {
		noContent(w) // legacy: 204 for unknown player
		return
	}
	writeJSON(w, map[string]string{"id": p.UUID, "name": p.Name})
}

// ---- shared writers ----

func writeTokenResponse(w http.ResponseWriter, t Token, players []Player, withUser bool) {
	type prof struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	available := make([]prof, 0, len(players))
	var selected *prof
	for _, p := range players {
		available = append(available, prof{ID: p.UUID, Name: p.Name})
		if p.ID == t.SelectedPID {
			sel := prof{ID: p.UUID, Name: p.Name}
			selected = &sel
		}
	}
	resp := map[string]interface{}{
		"accessToken":       t.AccessToken,
		"clientToken":       t.ClientToken,
		"availableProfiles": available,
	}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		return host[:i]
	}
	return host
}

var _ = errors.Is // keep errors import for future sentinel mapping
