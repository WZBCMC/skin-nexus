package union

import (
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestVerifier wires a verifier pinned to hubKey.
func newTestVerifier(t *testing.T, hubKey *rsa.PublicKey, audit AuditFunc) *Verifier {
	t.Helper()
	v, err := NewVerifier(Config{Enabled: true, HostPublicKeyPEM: pubPEM(t, hubKey)}, audit)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

// serve runs req through the middleware and reports whether the inner
// handler ran.
func serve(v *Verifier, req *http.Request) (*httptest.ResponseRecorder, bool) {
	called := false
	rec := httptest.NewRecorder()
	v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	return rec, called
}

func assertForbidden(t *testing.T, rec *httptest.ResponseRecorder, called bool) {
	t.Helper()
	if called {
		t.Fatal("inner handler must not run on verification failure")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %v", err)
	}
	if body["error"] != "ForbiddenOperationException" || body["errorMessage"] != "Union host verification failure." {
		t.Fatalf("error body = %v, want legacy {error,errorMessage} shape", body)
	}
}

func TestVerifierAcceptsValidRequest(t *testing.T) {
	hub := genRSA(t, 2048)
	log := &auditLog{}
	v := newTestVerifier(t, &hub.PublicKey, log.fn())

	body := `{"remapped_uuid":{"a":"b"}}`
	req := signedRequest(t, hub, http.MethodPost, "/api/union/member/remapuuid", body, time.Now(), "nonce-1")
	rec, called := serve(v, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("valid request rejected: status=%d called=%v body=%s", rec.Code, called, rec.Body)
	}
	if len(log.events) != 0 {
		t.Fatalf("audit must stay silent on success, got %v", log.events)
	}
	// The raw body must reach the handler untouched (signature is over it).
	if req.Body == nil {
		t.Fatal("body was not restored for the inner handler")
	}
}

func TestVerifierRejectsTampering(t *testing.T) {
	hub := genRSA(t, 2048)
	now := time.Now()
	body := `{"nonce":"abc"}`

	cases := []struct {
		name   string
		req    func() *http.Request
		reason string
	}{
		{
			name: "wrong signature key",
			req: func() *http.Request {
				other := genRSA(t, 2048)
				return signedRequest(t, other, http.MethodPost, "/x", body, now, "n1")
			},
			reason: "invalid_signature",
		},
		{
			name: "tampered body",
			req: func() *http.Request {
				req := signedRequest(t, hub, http.MethodPost, "/x", body, now, "n2")
				req.Body = io.NopCloser(strings.NewReader(`{"nonce":"evil"}`))
				req.ContentLength = int64(len(`{"nonce":"evil"}`))
				return req
			},
			reason: "invalid_signature",
		},
		{
			name: "expired timestamp",
			req: func() *http.Request {
				return signedRequest(t, hub, http.MethodPost, "/x", body, now.Add(-11*time.Second), "n3")
			},
			reason: "timestamp_out_of_window",
		},
		{
			name: "future timestamp beyond window",
			req: func() *http.Request {
				return signedRequest(t, hub, http.MethodPost, "/x", body, now.Add(31*time.Second), "n4")
			},
			reason: "timestamp_out_of_window",
		},
		{
			name:   "missing header",
			req:    func() *http.Request { return newRequest(t, http.MethodPost, "/x", body) },
			reason: "missing_header",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &auditLog{}
			v := newTestVerifier(t, &hub.PublicKey, log.fn())
			rec, called := serve(v, tc.req())
			assertForbidden(t, rec, called)
			ev, ok := log.last("union.verify." + tc.reason)
			if !ok {
				t.Fatalf("audit event union.verify.%s missing, got %v", tc.reason, log.events)
			}
			if ev.detail["reason"] != tc.reason {
				t.Fatalf("audit reason = %v", ev.detail["reason"])
			}
		})
	}
}

func TestVerifierRejectsReplayedNonce(t *testing.T) {
	hub := genRSA(t, 2048)
	log := &auditLog{}
	v := newTestVerifier(t, &hub.PublicKey, log.fn())
	now := time.Now()

	req := signedRequest(t, hub, http.MethodPost, "/x", `{"a":1}`, now, "replay-me")
	if rec, called := serve(v, req); rec.Code != http.StatusOK || !called {
		t.Fatalf("first request must pass, got %d", rec.Code)
	}
	// Byte-identical replay (same timestamp too).
	req2 := signedRequest(t, hub, http.MethodPost, "/x", `{"a":1}`, now, "replay-me")
	rec, called := serve(v, req2)
	assertForbidden(t, rec, called)
	if _, ok := log.last("union.verify.replayed_nonce"); !ok {
		t.Fatalf("replay audit missing, got %v", log.events)
	}
}

func TestVerifierTimestampWindowBoundaries(t *testing.T) {
	hub := genRSA(t, 2048)
	v := newTestVerifier(t, &hub.PublicKey, nil)
	base := time.Unix(1_800_000_000, 0)
	v.now = func() time.Time { return base }

	for _, tc := range []struct {
		name string
		at   time.Time
		ok   bool
	}{
		{"now-10s accepted", base.Add(-10 * time.Second), true},
		{"now+30s accepted", base.Add(30 * time.Second), true},
		{"now-11s rejected", base.Add(-11 * time.Second), false},
		{"now+31s rejected", base.Add(31 * time.Second), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := signedRequest(t, hub, http.MethodPost, "/x", "{}", tc.at, "b-"+tc.name)
			rec, called := serve(v, req)
			if tc.ok && (rec.Code != http.StatusOK || !called) {
				t.Fatalf("want pass, got %d", rec.Code)
			}
			if !tc.ok && rec.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d", rec.Code)
			}
		})
	}
}

func TestVerifierNonceTTL60s(t *testing.T) {
	hub := genRSA(t, 2048)
	v := newTestVerifier(t, &hub.PublicKey, nil)
	base := time.Unix(1_800_000_000, 0)
	now := base
	v.now = func() time.Time { return now }

	req := signedRequest(t, hub, http.MethodPost, "/x", "{}", now, "ttl-nonce")
	if rec, _ := serve(v, req); rec.Code != http.StatusOK {
		t.Fatalf("first request must pass, got %d", rec.Code)
	}
	// Still inside the 60s replay cache: fresh timestamp, fresh signature,
	// same nonce => rejected.
	now = base.Add(59 * time.Second)
	req = signedRequest(t, hub, http.MethodPost, "/x", "{}", now, "ttl-nonce")
	if rec, _ := serve(v, req); rec.Code != http.StatusForbidden {
		t.Fatalf("replay inside 60s must 403, got %d", rec.Code)
	}
	// After the nonce lifetime the same nonce is usable again.
	now = base.Add(61 * time.Second)
	req = signedRequest(t, hub, http.MethodPost, "/x", "{}", now, "ttl-nonce")
	if rec, _ := serve(v, req); rec.Code != http.StatusOK {
		t.Fatalf("nonce must expire after 60s, got %d", rec.Code)
	}
}

func TestVerifierDisabledIsInert(t *testing.T) {
	v, err := NewVerifier(Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("disabled verifier must construct: %v", err)
	}
	if v.Enabled() {
		t.Fatal("disabled verifier must report Enabled()==false")
	}
	rec, called := serve(v, signedRequest(t, genRSA(t, 2048), http.MethodPost, "/x", "{}", time.Now(), "n"))
	if called || rec.Code != http.StatusNotFound {
		t.Fatalf("disabled middleware must be a 404 handler, got %d called=%v", rec.Code, called)
	}
	if err := v.Verify([]byte("{}"), "sig", "1", "n"); err != ErrDisabled {
		t.Fatalf("Verify on disabled verifier = %v, want ErrDisabled", err)
	}
}

func TestVerifierRejectsBadPinnedKey(t *testing.T) {
	_, err := NewVerifier(Config{Enabled: true, HostPublicKeyPEM: "not a pem"}, nil)
	if err == nil {
		t.Fatal("garbage pinned key must fail construction")
	}
}
