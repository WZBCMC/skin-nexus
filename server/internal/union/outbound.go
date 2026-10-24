package union

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Outbound is the member -> hub client (analysis/yggdrasil-connect.md §4.2,
// field names frozen). Every call carries the X-Union-Member-Key header, is
// bounded by a timeout and returns wrapped errors. It is safe for concurrent
// use; the member key is swappable at runtime (updatebackendkey rotation).
type Outbound struct {
	root   string // hub API root without trailing slash
	client *http.Client
	key    atomic.Value // string: union member key
	to     time.Duration
	enable bool
}

// keyStore semantics: the member key is read on every request and rewritten
// only by rotation, so an atomic value beats a mutex here.

// APIError is a non-2xx hub response (except proxy endpoints, which relay it).
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       json.RawMessage
}

func (e *APIError) Error() string {
	return fmt.Sprintf("union: %s %s: hub returned %d: %s", e.Method, e.Path, e.StatusCode, strings.TrimSpace(string(e.Body)))
}

// Hub wire documents (§4.2). JSON tags are the hub field names, verbatim.
type (
	// ServerEntry is one member entry of the hub server list:
	// {code,name,bs_root}.
	ServerEntry struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		BSRoot string `json:"bs_root"`
	}

	// ServerList is the GET /serverlist response: {servers:[...],version}.
	ServerList struct {
		Servers []ServerEntry `json:"servers"`
		Version string        `json:"version"`
	}

	// PrivateKeyDoc is the GET /privatekey response:
	// {privateKey,privateKeyVersion}.
	PrivateKeyDoc struct {
		PrivateKey        string `json:"privateKey"`
		PrivateKeyVersion string `json:"privateKeyVersion"`
	}

	// BindToken is the POST /profile/bind response: {token} (2 minute TTL).
	BindToken struct {
		Token string `json:"token"`
	}

	// ServerCode is the POST /code response: {code}.
	ServerCode struct {
		Code string `json:"code"`
	}

	// OAuth2BackendKey is the GET /oauth2/backend response: {publicKey} (the
	// hub RSA encryption key used by the grant token).
	OAuth2BackendKey struct {
		PublicKey string `json:"publicKey"`
	}

	// ProfileDetail is GET /profile/detail/{uuid}. Fields whose hub type is
	// not yet pinned (hub-side source is closed, analysis §3 待验证) stay as
	// raw JSON so nothing breaks on wire drift; names are verbatim.
	ProfileDetail struct {
		InternalID     json.RawMessage   `json:"internal_id"`
		Bind           []json.RawMessage `json:"bind"`
		BindMappedName string            `json:"bind_mapped_name"`
		UUID           string            `json:"uuid"`
		MappedUUID     string            `json:"mapped_uuid"`
		BindStatus     json.RawMessage   `json:"bind_status"`
		Backend        json.RawMessage   `json:"backend"`
	}

	// UnmappedProfile is one entry of GET /profile/unmapped/byname/{name}
	// (name-conflict candidates; legacy keys the list by internal_id).
	UnmappedProfile struct {
		InternalID json.RawMessage `json:"internal_id"`
		UUID       string          `json:"uuid"`
		Name       string          `json:"name"`
	}

	// ProxyResult relays a blacklist hub response untouched (status + body).
	ProxyResult struct {
		StatusCode int
		Body       json.RawMessage
	}
)

// flexString tolerates the hub encoding a spec-"string" field as a bare JSON
// number (observed 2026-10-24 on the live hub: /serverlist `version` and
// /privatekey `privateKeyVersion` arrive as numbers, e.g. 3 not "3"). Wire
// field NAMES and public struct fields stay frozen (string) — this is purely
// additive decode tolerance, same trap family as MS1 `{id}` narrowing.
type flexString string

// UnmarshalJSON accepts "x", 3, 3.0, true, and null (→ "").
func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(b) // bare number/bool literal as-is
	return nil
}

// UnmarshalJSON shadow-decodes ServerList so `version` tolerates number
// encoding while the exported field remains a plain string.
func (s *ServerList) UnmarshalJSON(b []byte) error {
	var raw struct {
		Servers []ServerEntry `json:"servers"`
		Version flexString    `json:"version"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.Servers = raw.Servers
	s.Version = string(raw.Version)
	return nil
}

// UnmarshalJSON shadow-decodes PrivateKeyDoc so `privateKeyVersion` tolerates
// number encoding while the exported field remains a plain string.
func (p *PrivateKeyDoc) UnmarshalJSON(b []byte) error {
	var raw struct {
		PrivateKey        flexString `json:"privateKey"`
		PrivateKeyVersion flexString `json:"privateKeyVersion"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.PrivateKey = string(raw.PrivateKey)
	p.PrivateKeyVersion = string(raw.PrivateKeyVersion)
	return nil
}

// NewOutbound builds the hub client. Disabled config yields an inert client
// (Enabled() == false, all calls return ErrDisabled) and never touches the
// network. client may be nil (default: plain http.Client, per-call timeouts).
func NewOutbound(cfg Config, client *http.Client) (*Outbound, error) {
	o := &Outbound{client: client, to: cfg.requestTimeout(), enable: cfg.Enabled}
	if o.client == nil {
		o.client = &http.Client{}
	}
	if !cfg.Enabled {
		o.root = cfg.apiRoot()
		o.key.Store("")
		return o, nil
	}
	root := cfg.apiRoot()
	if root == "" {
		return nil, errors.New("union: ApiRoot is required")
	}
	u, err := url.Parse(root)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("union: ApiRoot must be an absolute URL")
	}
	o.root = root
	o.key.Store(cfg.MemberKey)
	return o, nil
}

// Enabled reports whether the client performs real work (nil-safe).
func (o *Outbound) Enabled() bool { return o != nil && o.enable }

// SetMemberKey swaps the rotating member key (updatebackendkey channel).
func (o *Outbound) SetMemberKey(key string) {
	if o != nil {
		o.key.Store(key)
	}
}

// MemberKey returns the current member key.
func (o *Outbound) MemberKey() string {
	if o == nil {
		return ""
	}
	s, _ := o.key.Load().(string)
	return s
}

// ---- profile events (§4.2) ----

// SyncProfileCreate reports a new profile: POST /profile {id,name}. Note the
// hub field is "id" (not "uuid") — frozen.
func (o *Outbound) SyncProfileCreate(ctx context.Context, uuid, name string) error {
	body := map[string]string{"id": uuid, "name": name}
	_, _, err := o.doJSON(ctx, http.MethodPost, "/profile", body, nil)
	return err
}

// SyncProfileRename reports a rename: PUT /profile/{uuid} {name}.
func (o *Outbound) SyncProfileRename(ctx context.Context, uuid, name string) error {
	body := map[string]string{"name": name}
	_, _, err := o.doJSON(ctx, http.MethodPut, "/profile/"+url.PathEscape(uuid), body, nil)
	return err
}

// SyncProfileDelete reports a deletion: DELETE /profile/{uuid}.
func (o *Outbound) SyncProfileDelete(ctx context.Context, uuid string) error {
	_, _, err := o.doJSON(ctx, http.MethodDelete, "/profile/"+url.PathEscape(uuid), nil, nil)
	return err
}

// Sync uploads the local profile roster: POST /sync {profileList:{uuid:name}}.
func (o *Outbound) Sync(ctx context.Context, profileList map[string]string) error {
	body := map[string]any{"profileList": profileList}
	_, _, err := o.doJSON(ctx, http.MethodPost, "/sync", body, nil)
	return err
}

// ---- hub data pulls ----

// ServerList pulls GET /serverlist -> {servers:[{code,name,bs_root}],version}.
func (o *Outbound) ServerList(ctx context.Context) (*ServerList, error) {
	var doc ServerList
	if _, _, err := o.doJSON(ctx, http.MethodGet, "/serverlist", nil, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Privatekey pulls GET /privatekey -> {privateKey,privateKeyVersion}. Name
// spelling follows the spec/hub field ("privatekey"); the hub-issued key
// replaces the shared texture-signing key (§7.1-5).
func (o *Outbound) Privatekey(ctx context.Context) (*PrivateKeyDoc, error) {
	var doc PrivateKeyDoc
	if _, _, err := o.doJSON(ctx, http.MethodGet, "/privatekey", nil, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Diagnose pings POST /diagnose; the hub response shape is hub-private, so it
// is relayed raw (legacy triggerDiagnose wraps it the same way).
func (o *Outbound) Diagnose(ctx context.Context) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, diagnoseTimeout)
	defer cancel()
	_, body, err := o.doJSON(ctx, http.MethodPost, "/diagnose", map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// ServerCode exchanges the member key for the public server code:
// POST /code {token} -> {code} (§4.2 安全级别).
func (o *Outbound) ServerCode(ctx context.Context, token string) (*ServerCode, error) {
	body := map[string]string{"token": token}
	var doc ServerCode
	if _, _, err := o.doJSON(ctx, http.MethodPost, "/code", body, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// OAuth2BackendKey pulls the hub RSA encryption public key:
// GET /oauth2/backend -> {publicKey}.
func (o *Outbound) OAuth2BackendKey(ctx context.Context) (*OAuth2BackendKey, error) {
	var doc OAuth2BackendKey
	if _, _, err := o.doJSON(ctx, http.MethodGet, "/oauth2/backend", nil, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ---- binding family ----

// Bind starts binding: POST /profile/bind {uuid} -> {token} (2 min validity).
func (o *Outbound) Bind(ctx context.Context, uuid string) (*BindToken, error) {
	body := map[string]string{"uuid": uuid}
	var doc BindToken
	if _, _, err := o.doJSON(ctx, http.MethodPost, "/profile/bind", body, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// BindTo binds this profile to a primary profile token:
// POST /profile/bindto {uuid,token}.
func (o *Outbound) BindTo(ctx context.Context, uuid, token string) error {
	body := map[string]string{"uuid": uuid, "token": token}
	_, _, err := o.doJSON(ctx, http.MethodPost, "/profile/bindto", body, nil)
	return err
}

// Unbind unbinds: POST /profile/unbind {uuid}.
func (o *Outbound) Unbind(ctx context.Context, uuid string) error {
	body := map[string]string{"uuid": uuid}
	_, _, err := o.doJSON(ctx, http.MethodPost, "/profile/unbind", body, nil)
	return err
}

// RemapUUID requests a UUID remap across the union:
// POST /profile/remapuuid {me,target}.
func (o *Outbound) RemapUUID(ctx context.Context, me, target string) error {
	body := map[string]string{"me": me, "target": target}
	_, _, err := o.doJSON(ctx, http.MethodPost, "/profile/remapuuid", body, nil)
	return err
}

// UnmappedByName lists name-conflict candidates:
// GET /profile/unmapped/byname/{name}.
func (o *Outbound) UnmappedByName(ctx context.Context, name string) ([]UnmappedProfile, error) {
	var doc []UnmappedProfile
	path := "/profile/unmapped/byname/" + url.PathEscape(name)
	if _, _, err := o.doJSON(ctx, http.MethodGet, path, nil, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// DetailByUUID fetches hub-side binding detail: GET /profile/detail/{uuid}.
func (o *Outbound) DetailByUUID(ctx context.Context, uuid string) (*ProfileDetail, error) {
	var doc ProfileDetail
	path := "/profile/detail/" + url.PathEscape(uuid)
	if _, _, err := o.doJSON(ctx, http.MethodGet, path, nil, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// ---- blacklist proxy (原样透传) ----

// BlacklistQuery proxies GET /blacklist/query?<query>.
func (o *Outbound) BlacklistQuery(ctx context.Context, query url.Values) (*ProxyResult, error) {
	path := "/blacklist/query"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return o.doProxy(ctx, http.MethodGet, path, nil)
}

// BlacklistCreate proxies POST /blacklist/restful with the raw JSON body.
func (o *Outbound) BlacklistCreate(ctx context.Context, body json.RawMessage) (*ProxyResult, error) {
	return o.doProxy(ctx, http.MethodPost, "/blacklist/restful", body)
}

// BlacklistInvalidate proxies PUT /blacklist/invalidate/{id}.
func (o *Outbound) BlacklistInvalidate(ctx context.Context, id string) (*ProxyResult, error) {
	return o.doProxy(ctx, http.MethodPut, "/blacklist/invalidate/"+url.PathEscape(id), nil)
}

// BlacklistDelete proxies DELETE /blacklist/restful/{id}.
func (o *Outbound) BlacklistDelete(ctx context.Context, id string) (*ProxyResult, error) {
	return o.doProxy(ctx, http.MethodDelete, "/blacklist/restful/"+url.PathEscape(id), nil)
}

// ---- transport ----

// doJSON performs one authenticated JSON call. Non-2xx becomes *APIError; a
// nil out skips response decoding.
func (o *Outbound) doJSON(ctx context.Context, method, path string, body any, out any) (int, json.RawMessage, error) {
	if !o.Enabled() {
		return 0, nil, ErrDisabled
	}
	var payload []byte
	switch b := body.(type) {
	case nil:
	case json.RawMessage:
		payload = b
	default:
		enc, err := json.Marshal(b)
		if err != nil {
			return 0, nil, fmt.Errorf("union: encode %s %s body: %w", method, path, err)
		}
		payload = enc
	}

	ctx, cancel := context.WithTimeout(ctx, o.to)
	defer cancel()

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.root+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("union: build %s %s: %w", method, path, err)
	}
	req.Header.Set(HeaderMemberKey, o.MemberKey())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("union: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("union: %s %s: read response: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, raw, &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: raw}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, raw, fmt.Errorf("union: %s %s: decode response: %w", method, path, err)
		}
	}
	return resp.StatusCode, raw, nil
}

// doProxy relays a blacklist call untouched: transport errors are returned,
// hub non-2xx answers are passed through in ProxyResult (legacy behavior of
// UnionBlacklistController: response(body, status)).
func (o *Outbound) doProxy(ctx context.Context, method, path string, body json.RawMessage) (*ProxyResult, error) {
	if !o.Enabled() {
		return nil, ErrDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, o.to)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.root+path, reader)
	if err != nil {
		return nil, fmt.Errorf("union: build %s %s: %w", method, path, err)
	}
	req.Header.Set(HeaderMemberKey, o.MemberKey())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("union: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("union: %s %s: read response: %w", method, path, err)
	}
	return &ProxyResult{StatusCode: resp.StatusCode, Body: raw}, nil
}
