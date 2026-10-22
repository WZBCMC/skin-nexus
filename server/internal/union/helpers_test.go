package union

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- self-generated RSA fixtures (no network, no checked-in secrets) ----

// genRSA generates a test key pair. Tests use 2048 for speed and 4096 only
// where the chunked-encryption boundary needs it.
func genRSA(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("generate %d-bit key: %v", bits, err)
	}
	return key
}

// privPEM renders a private key as PKCS#8 PEM (hub /privatekey format).
func privPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// privPEMPKCS1 renders a private key as PKCS#1 PEM ("BEGIN RSA PRIVATE KEY").
func privPEMPKCS1(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}

// pubPEM renders a public key as strict PKIX PEM.
func pubPEM(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// signMessageRSA is RSA-SHA256 (PKCS#1 v1.5) over msg, Base64 out — the hub
// message signature algorithm (§4.3).
func signMessageRSA(t *testing.T, key *rsa.PrivateKey, msg string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// ---- audit capture ----

type auditEvent struct {
	event  string
	detail map[string]any
}

type auditLog struct {
	mu     sync.Mutex
	events []auditEvent
}

func (a *auditLog) fn() AuditFunc {
	return func(event string, detail map[string]any) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.events = append(a.events, auditEvent{event: event, detail: detail})
	}
}

func (a *auditLog) count(event string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, e := range a.events {
		if e.event == event {
			n++
		}
	}
	return n
}

func (a *auditLog) last(event string) (auditEvent, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := len(a.events) - 1; i >= 0; i-- {
		if a.events[i].event == event {
			return a.events[i], true
		}
	}
	return auditEvent{}, false
}

// ---- signed hub request builder ----

// signedRequest builds a hub -> member request with the three frozen headers;
// the signed payload is body + timestamp + nonce (string concatenation).
func signedRequest(t *testing.T, hubKey *rsa.PrivateKey, method, target, body string, at time.Time, nonce string) *http.Request {
	t.Helper()
	req := newRequest(t, method, target, body)
	ts := strconv.FormatInt(at.Unix(), 10)
	payload := body + ts + nonce
	req.Header.Set(HeaderMessageSignature, signMessageRSA(t, hubKey, payload))
	req.Header.Set(HeaderMessageTimestamp, ts)
	req.Header.Set(HeaderMessageNonce, nonce)
	return req
}

func newRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	return req
}

// signedRequestTweak builds a request then mutates one header/body after
// signing, to prove tampering is caught.
func signedRequestTweak(t *testing.T, hubKey *rsa.PrivateKey, method, target, body string, at time.Time, nonce string, tweak func(*http.Request)) *http.Request {
	t.Helper()
	req := signedRequest(t, hubKey, method, target, body, at, nonce)
	if tweak != nil {
		tweak(req)
	}
	return req
}
