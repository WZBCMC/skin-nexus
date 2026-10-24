package union

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// hubCall is one recorded member -> hub request.
type hubCall struct {
	method    string
	path      string
	rawQuery  string
	body      string
	memberKey string
}

// newRecordingHub runs a fake hub that records every request and delegates
// the response to respond.
func newRecordingHub(t *testing.T, respond func(w http.ResponseWriter, call hubCall)) (*httptest.Server, *[]hubCall) {
	t.Helper()
	var mu sync.Mutex
	calls := &[]hubCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		call := hubCall{
			method:    r.Method,
			path:      r.URL.Path,
			rawQuery:  r.URL.RawQuery,
			body:      string(b),
			memberKey: r.Header.Get(HeaderMemberKey),
		}
		mu.Lock()
		*calls = append(*calls, call)
		mu.Unlock()
		if respond != nil {
			respond(w, call)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func jsonRespond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newTestOutbound(t *testing.T, root, memberKey string) *Outbound {
	t.Helper()
	o, err := NewOutbound(Config{Enabled: true, ApiRoot: root, MemberKey: memberKey}, nil)
	if err != nil {
		t.Fatalf("NewOutbound: %v", err)
	}
	return o
}

func bodyKeys(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("request body is not JSON object: %v (%s)", err, body)
	}
	return m
}

func TestOutboundProfileEvents(t *testing.T) {
	srv, calls := newRecordingHub(t, nil)
	o := newTestOutbound(t, srv.URL, "mk-1")
	ctx := context.Background()

	if err := o.SyncProfileCreate(ctx, "uuid-1", "Steve"); err != nil {
		t.Fatalf("SyncProfileCreate: %v", err)
	}
	if err := o.SyncProfileRename(ctx, "uuid-1", "Alex"); err != nil {
		t.Fatalf("SyncProfileRename: %v", err)
	}
	if err := o.SyncProfileDelete(ctx, "uuid-1"); err != nil {
		t.Fatalf("SyncProfileDelete: %v", err)
	}
	if err := o.Sync(ctx, map[string]string{"uuid-1": "Steve", "uuid-2": "Alex"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	want := []hubCall{
		{method: http.MethodPost, path: "/profile"},
		{method: http.MethodPut, path: "/profile/uuid-1"},
		{method: http.MethodDelete, path: "/profile/uuid-1"},
		{method: http.MethodPost, path: "/sync"},
	}
	if len(*calls) != len(want) {
		t.Fatalf("hub saw %d calls, want %d", len(*calls), len(want))
	}
	for i, w := range want {
		got := (*calls)[i]
		if got.method != w.method || got.path != w.path {
			t.Fatalf("call %d = %s %s, want %s %s", i, got.method, got.path, w.method, w.path)
		}
		if got.memberKey != "mk-1" {
			t.Fatalf("call %d missing %s header, got %q", i, HeaderMemberKey, got.memberKey)
		}
	}

	// Field names are frozen (§4.2): create uses {id,name}, NOT {uuid,name}.
	create := bodyKeys(t, (*calls)[0].body)
	if _, ok := create["id"]; !ok {
		t.Fatalf("POST /profile body must use field \"id\": %v", create)
	}
	if create["id"] != "uuid-1" || create["name"] != "Steve" {
		t.Fatalf("POST /profile body = %v", create)
	}
	if len(create) != 2 {
		t.Fatalf("POST /profile body has extra fields: %v", create)
	}
	rename := bodyKeys(t, (*calls)[1].body)
	if _, ok := rename["name"]; !ok || len(rename) != 1 {
		t.Fatalf("PUT /profile body must be {name}: %v", rename)
	}
	sync := bodyKeys(t, (*calls)[3].body)
	pl, ok := sync["profileList"].(map[string]any)
	if !ok || len(sync) != 1 {
		t.Fatalf("POST /sync body must be {profileList:{uuid:name}}: %v", sync)
	}
	if pl["uuid-1"] != "Steve" || pl["uuid-2"] != "Alex" {
		t.Fatalf("profileList = %v", pl)
	}
}

func TestOutboundDataPulls(t *testing.T) {
	srv, calls := newRecordingHub(t, func(w http.ResponseWriter, call hubCall) {
		switch call.path {
		case "/serverlist":
			jsonRespond(w, 200, map[string]any{
				"servers": []map[string]any{{"code": "WZB", "name": "WZBC", "bs_root": "https://skin.example/"}},
				"version": "7",
			})
		case "/privatekey":
			jsonRespond(w, 200, map[string]any{"privateKey": "PEM", "privateKeyVersion": "3"})
		case "/oauth2/backend":
			jsonRespond(w, 200, map[string]any{"publicKey": "HUBPEM"})
		case "/code":
			jsonRespond(w, 200, map[string]any{"code": "WZB"})
		case "/diagnose":
			jsonRespond(w, 200, []any{map[string]any{"nonce": "x"}, map[string]any{"timestamp": 1.5}})
		}
	})
	o := newTestOutbound(t, srv.URL, "mk-2")
	ctx := context.Background()

	list, err := o.ServerList(ctx)
	if err != nil {
		t.Fatalf("ServerList: %v", err)
	}
	if list.Version != "7" || len(list.Servers) != 1 ||
		list.Servers[0].Code != "WZB" || list.Servers[0].Name != "WZBC" || list.Servers[0].BSRoot != "https://skin.example/" {
		t.Fatalf("ServerList decoded wrong: %+v", list)
	}

	pk, err := o.Privatekey(ctx)
	if err != nil {
		t.Fatalf("Privatekey: %v", err)
	}
	if pk.PrivateKey != "PEM" || pk.PrivateKeyVersion != "3" {
		t.Fatalf("Privatekey decoded wrong: %+v", pk)
	}

	key, err := o.OAuth2BackendKey(ctx)
	if err != nil {
		t.Fatalf("OAuth2BackendKey: %v", err)
	}
	if key.PublicKey != "HUBPEM" {
		t.Fatalf("OAuth2BackendKey decoded wrong: %+v", key)
	}

	code, err := o.ServerCode(ctx, "mk-2")
	if err != nil {
		t.Fatalf("ServerCode: %v", err)
	}
	if code.Code != "WZB" {
		t.Fatalf("ServerCode decoded wrong: %+v", code)
	}

	rep, err := o.Diagnose(ctx)
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("Diagnose status = %q, want ok", rep.Status)
	}
	if len(rep.Data) == 0 {
		t.Fatal("Diagnose must relay the hub body")
	}

	// Wire shapes: methods and JSON field names.
	byPath := map[string]hubCall{}
	for _, c := range *calls {
		byPath[c.path] = c
	}
	if c := byPath["/serverlist"]; c.method != http.MethodGet {
		t.Fatalf("serverlist method = %s", c.method)
	}
	if c := byPath["/privatekey"]; c.method != http.MethodGet {
		t.Fatalf("privatekey method = %s", c.method)
	}
	if c := byPath["/oauth2/backend"]; c.method != http.MethodGet {
		t.Fatalf("oauth2/backend method = %s", c.method)
	}
	if c := byPath["/code"]; c.method != http.MethodPost {
		t.Fatalf("code method = %s", c.method)
	}
	codeBody := bodyKeys(t, byPath["/code"].body)
	if codeBody["token"] != "mk-2" || len(codeBody) != 1 {
		t.Fatalf("POST /code body must be {token}: %v", codeBody)
	}
}

func TestOutboundBindingFamily(t *testing.T) {
	srv, calls := newRecordingHub(t, func(w http.ResponseWriter, call hubCall) {
		switch call.path {
		case "/profile/bind":
			jsonRespond(w, 200, map[string]any{"token": "tok-1"})
		case "/profile/unmapped/byname/Steve A": // r.URL.Path is decoded
			jsonRespond(w, 200, []map[string]any{{"internal_id": 5, "uuid": "u", "name": "Steve A"}})
		case "/profile/detail/uuid-1":
			jsonRespond(w, 200, map[string]any{
				"internal_id": 5, "bind": []any{}, "bind_mapped_name": "Steve@WZB",
				"uuid": "uuid-1", "mapped_uuid": "uuid-9", "bind_status": 0, "backend": "WZB",
			})
		}
	})
	o := newTestOutbound(t, srv.URL, "mk-3")
	ctx := context.Background()

	tok, err := o.Bind(ctx, "uuid-1")
	if err != nil || tok.Token != "tok-1" {
		t.Fatalf("Bind = %+v, %v", tok, err)
	}
	if err := o.BindTo(ctx, "uuid-1", "tok-1"); err != nil {
		t.Fatalf("BindTo: %v", err)
	}
	if err := o.Unbind(ctx, "uuid-1"); err != nil {
		t.Fatalf("Unbind: %v", err)
	}
	if err := o.RemapUUID(ctx, "uuid-1", "uuid-9"); err != nil {
		t.Fatalf("RemapUUID: %v", err)
	}
	unmapped, err := o.UnmappedByName(ctx, "Steve A")
	if err != nil || len(unmapped) != 1 || unmapped[0].Name != "Steve A" {
		t.Fatalf("UnmappedByName = %+v, %v", unmapped, err)
	}
	detail, err := o.DetailByUUID(ctx, "uuid-1")
	if err != nil || detail.UUID != "uuid-1" || detail.MappedUUID != "uuid-9" || detail.BindMappedName != "Steve@WZB" {
		t.Fatalf("DetailByUUID = %+v, %v", detail, err)
	}

	byPath := map[string]hubCall{}
	for _, c := range *calls {
		byPath[c.path] = c
	}
	if c := byPath["/profile/bind"]; c.method != http.MethodPost {
		t.Fatalf("bind method = %s", c.method)
	}
	bindBody := bodyKeys(t, byPath["/profile/bind"].body)
	if bindBody["uuid"] != "uuid-1" || len(bindBody) != 1 {
		t.Fatalf("POST /profile/bind body must be {uuid}: %v", bindBody)
	}
	if c := byPath["/profile/bindto"]; c.method != http.MethodPost {
		t.Fatalf("bindto method = %s", c.method)
	}
	bindToBody := bodyKeys(t, byPath["/profile/bindto"].body)
	if bindToBody["uuid"] != "uuid-1" || bindToBody["token"] != "tok-1" || len(bindToBody) != 2 {
		t.Fatalf("POST /profile/bindto body must be {uuid,token}: %v", bindToBody)
	}
	unbindBody := bodyKeys(t, byPath["/profile/unbind"].body)
	if unbindBody["uuid"] != "uuid-1" {
		t.Fatalf("POST /profile/unbind body must be {uuid}: %v", unbindBody)
	}
	remapBody := bodyKeys(t, byPath["/profile/remapuuid"].body)
	if remapBody["me"] != "uuid-1" || remapBody["target"] != "uuid-9" || len(remapBody) != 2 {
		t.Fatalf("POST /profile/remapuuid body must be {me,target}: %v", remapBody)
	}
	// The name goes out percent-escaped; httptest reports the decoded path.
	if _, ok := byPath["/profile/unmapped/byname/Steve A"]; !ok {
		t.Fatalf("byname path not found in %v", byPath)
	}
	if c := byPath["/profile/detail/uuid-1"]; c.method != http.MethodGet {
		t.Fatalf("detail method = %s", c.method)
	}
}

func TestOutboundBlacklistProxy(t *testing.T) {
	srv, calls := newRecordingHub(t, func(w http.ResponseWriter, call hubCall) {
		// Deliberately non-2xx: proxy must relay it untouched.
		jsonRespond(w, 422, map[string]any{"error": "upstream", "errorMessage": "nope"})
	})
	o := newTestOutbound(t, srv.URL, "mk-4")
	ctx := context.Background()

	q := url.Values{"email": {"a@example.com"}}
	res, err := o.BlacklistQuery(ctx, q)
	if err != nil {
		t.Fatalf("BlacklistQuery transport error: %v", err)
	}
	if res.StatusCode != 422 || string(res.Body) == "" {
		t.Fatalf("BlacklistQuery must relay status+body: %+v", res)
	}

	if _, err := o.BlacklistCreate(ctx, json.RawMessage(`{"email":"a@example.com","reason":"x"}`)); err != nil {
		t.Fatalf("BlacklistCreate: %v", err)
	}
	if _, err := o.BlacklistInvalidate(ctx, "42"); err != nil {
		t.Fatalf("BlacklistInvalidate: %v", err)
	}
	if _, err := o.BlacklistDelete(ctx, "42"); err != nil {
		t.Fatalf("BlacklistDelete: %v", err)
	}

	byPath := map[string]hubCall{}
	for _, c := range *calls {
		byPath[c.path] = c
	}
	if c := byPath["/blacklist/query"]; c.method != http.MethodGet || c.rawQuery != "email=a%40example.com" {
		t.Fatalf("blacklist/query wire = %+v", c)
	}
	create := byPath["/blacklist/restful"]
	if create.method != http.MethodPost || create.body != `{"email":"a@example.com","reason":"x"}` {
		t.Fatalf("blacklist create must pass JSON through verbatim: %+v", create)
	}
	if c := byPath["/blacklist/invalidate/42"]; c.method != http.MethodPut {
		t.Fatalf("invalidate wire = %+v", c)
	}
	if c := byPath["/blacklist/restful/42"]; c.method != http.MethodDelete {
		t.Fatalf("delete wire = %+v", c)
	}
	for _, c := range *calls {
		if c.memberKey != "mk-4" {
			t.Fatalf("blacklist call missing member key: %+v", c)
		}
	}
}

func TestOutboundErrorWrapping(t *testing.T) {
	srv, _ := newRecordingHub(t, func(w http.ResponseWriter, call hubCall) {
		jsonRespond(w, 500, map[string]any{"error": "boom"})
	})
	o := newTestOutbound(t, srv.URL, "mk-5")

	_, err := o.ServerList(context.Background())
	if err == nil {
		t.Fatal("non-2xx must be an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Fatalf("want *APIError with status 500, got %v", err)
	}
}

func TestOutboundMemberKeyRotation(t *testing.T) {
	srv, calls := newRecordingHub(t, nil)
	o := newTestOutbound(t, srv.URL, "old-key")
	o.SetMemberKey("new-key")
	if err := o.SyncProfileDelete(context.Background(), "u"); err != nil {
		t.Fatalf("SyncProfileDelete: %v", err)
	}
	if (*calls)[0].memberKey != "new-key" {
		t.Fatalf("rotated key not used: %+v", (*calls)[0])
	}
}

func TestOutboundDisabled(t *testing.T) {
	o, err := NewOutbound(Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("disabled outbound must construct: %v", err)
	}
	if o.Enabled() {
		t.Fatal("disabled outbound must report Enabled()==false")
	}
	if _, err := o.ServerList(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("ServerList on disabled = %v, want ErrDisabled", err)
	}
	if err := o.SyncProfileCreate(context.Background(), "u", "n"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("SyncProfileCreate on disabled = %v", err)
	}
	if _, err := o.BlacklistQuery(context.Background(), nil); !errors.Is(err, ErrDisabled) {
		t.Fatalf("BlacklistQuery on disabled = %v", err)
	}
}
