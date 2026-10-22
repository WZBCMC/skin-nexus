package union

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Verifier authenticates hub -> member requests (analysis/yggdrasil-connect.md
// §4.3, wire-exact):
//
//	headers: X-Message-Signature (base64), X-Message-Timestamp (Unix seconds),
//	         X-Message-Nonce
//	payload: rawBody + timestamp + nonce   (string concatenation, order frozen)
//	verify:  RSA-SHA256 (PKCS#1 v1.5) over payload with the hub public key
//	window:  timestamp in [now-10s, now+30s]; nonce cached 60s against replay
//
// Security delta vs the legacy plugin (plan §5, "固定 pin"): the hub public key
// is a constructor parameter, never fetched per request. The legacy middleware
// pulled it from GET union_api_root on every call (UnionHostVerify.php:67),
// which let anyone who hijacks the transport replace the trust root.
type Verifier struct {
	pub   *rsa.PublicKey
	audit AuditFunc
	now   func() time.Time // injectable clock for tests

	mu        sync.Mutex
	nonces    map[string]time.Time // nonce -> expiry
	lastSweep time.Time
}

// Verification failure sentinels; the middleware maps all of them to the same
// 403 but keeps the reason apart for the audit trail.
var (
	errVerify          = errors.New("union: hub message verification failed")
	errVerifyHeader    = errors.New("missing signature header")
	errVerifyReplay    = errors.New("nonce already used")
	errVerifyTimeFmt   = errors.New("malformed timestamp")
	errVerifyTimeSkew  = errors.New("timestamp outside window")
	errVerifySignature = errors.New("invalid signature")
)

// NewVerifier builds the pinned-key verifier. When the module is disabled an
// inert verifier is returned without parsing anything. The pinned key must be
// a PKIX PEM ("BEGIN PUBLIC KEY"); a PKCS#1 PEM ("BEGIN RSA PUBLIC KEY") is
// accepted as well for robustness against hub key-format drift.
func NewVerifier(cfg Config, audit AuditFunc) (*Verifier, error) {
	if !cfg.Enabled {
		return &Verifier{audit: audit, now: time.Now, nonces: map[string]time.Time{}}, nil
	}
	pub, err := parsePublicKeyPEM(cfg.HostPublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("union: pinned host public key: %w", err)
	}
	return &Verifier{
		pub:    pub,
		audit:  audit,
		now:    time.Now,
		nonces: map[string]time.Time{},
	}, nil
}

// Enabled reports whether the verifier performs real work (nil-safe).
func (v *Verifier) Enabled() bool { return v != nil && v.pub != nil }

// Verify checks one message: the exact middleware core, usable outside HTTP.
// Every failure maps to 403 in the middleware (legacy semantics).
func (v *Verifier) Verify(rawBody []byte, signature, timestamp, nonce string) error {
	if !v.Enabled() {
		return ErrDisabled
	}
	if signature == "" || timestamp == "" || nonce == "" {
		return fmt.Errorf("%w: %w", errVerify, errVerifyHeader)
	}
	// Replay cache first, then timestamp window, then signature — the same
	// check order as the legacy middleware (UnionHostVerify.php:57-72).
	if v.seen(nonce) {
		return fmt.Errorf("%w: %w", errVerify, errVerifyReplay)
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %w", errVerify, errVerifyTimeFmt)
	}
	now := v.now()
	at := time.Unix(ts, 0)
	if at.Before(now.Add(-VerifyPastSkew)) || at.After(now.Add(VerifyFutureSkew)) {
		return fmt.Errorf("%w: %w [%s,%s]", errVerify, errVerifyTimeSkew, -VerifyPastSkew, VerifyFutureSkew)
	}
	// openssl_verify(body+timestamp+nonce, base64_decode(sig), pub, SHA256)
	payload := string(rawBody) + timestamp + nonce
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("%w: signature is not base64: %w", errVerify, errVerifySignature)
	}
	digest := crypto.SHA256.New()
	digest.Write([]byte(payload))
	if err := rsa.VerifyPKCS1v15(v.pub, crypto.SHA256, digest.Sum(nil), sig); err != nil {
		return fmt.Errorf("%w: %w", errVerify, errVerifySignature)
	}
	// Cache the nonce only after full success, mirroring the legacy
	// Cache::put after openssl_verify (UnionHostVerify.php:75).
	v.remember(nonce)
	return nil
}

// Middleware wraps next with full request authentication. On any failure it
// answers 403 with the legacy error body and reports through AuditFunc.
// When disabled it is a 404 handler: nothing leaks even if mounted by mistake.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	if !v.Enabled() {
		return http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			v.fail(w, r, "body_unreadable", "")
			return
		}
		// Restore the raw body for the wrapped handler.
		r.Body = io.NopCloser(bytes.NewReader(raw))

		signature := r.Header.Get(HeaderMessageSignature)
		timestamp := r.Header.Get(HeaderMessageTimestamp)
		nonce := r.Header.Get(HeaderMessageNonce)

		if err := v.Verify(raw, signature, timestamp, nonce); err != nil {
			v.fail(w, r, verifyReason(err), nonce)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// verifyReason maps a Verify error to a stable audit reason token.
func verifyReason(err error) string {
	switch {
	case errors.Is(err, ErrDisabled):
		return "disabled"
	case errors.Is(err, errVerifyHeader):
		return "missing_header"
	case errors.Is(err, errVerifyReplay):
		return "replayed_nonce"
	case errors.Is(err, errVerifyTimeFmt):
		return "malformed_timestamp"
	case errors.Is(err, errVerifyTimeSkew):
		return "timestamp_out_of_window"
	default:
		return "invalid_signature"
	}
}

// fail writes the legacy 403 body and records the audit event.
func (v *Verifier) fail(w http.ResponseWriter, r *http.Request, reason, nonce string) {
	detail := map[string]any{"reason": reason}
	if r != nil {
		detail["path"] = r.URL.Path
		detail["remote"] = r.RemoteAddr
	}
	if nonce != "" {
		detail["nonce"] = nonce
	}
	v.audit.call("union.verify."+reason, detail)
	writeError(w, http.StatusForbidden, codeForbidden, verifyFailureMsg)
}

// StartJanitor runs periodic nonce-cache cleanup until ctxDone is closed.
// Cleanup also happens lazily on writes, so this is optional hygiene for
// long-lived processes. Nil-safe and a no-op when disabled.
func (v *Verifier) StartJanitor(ctxDone <-chan struct{}, interval time.Duration) {
	if !v.Enabled() || interval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctxDone:
				return
			case <-t.C:
				v.sweep(v.now())
			}
		}
	}()
}

// seen reports whether nonce was already used (and still cached).
func (v *Verifier) seen(nonce string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	exp, ok := v.nonces[nonce]
	return ok && exp.After(v.now())
}

// remember stores nonce for NonceTTL and lazily sweeps the cache.
func (v *Verifier) remember(nonce string) {
	now := v.now()
	v.mu.Lock()
	defer v.mu.Unlock()
	v.nonces[nonce] = now.Add(NonceTTL)
	if now.Sub(v.lastSweep) > NonceTTL/2 || len(v.nonces) > 256 {
		v.sweepLocked(now)
	}
}

// sweep drops expired nonce entries.
func (v *Verifier) sweep(now time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sweepLocked(now)
}

func (v *Verifier) sweepLocked(now time.Time) {
	for k, exp := range v.nonces {
		if !exp.After(now) {
			delete(v.nonces, k)
		}
	}
	v.lastSweep = now
}

// ---- PEM helpers (shared by signer/grantor) ----

// parsePublicKeyPEM parses a PKIX (or PKCS#1 fallback) RSA public key PEM.
func parsePublicKeyPEM(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaPub, nil
	}
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("not a PKIX/PKCS#1 RSA public key")
	}
	return rsaPub, nil
}

// parsePrivateKeyPEM parses a PKCS#8 or PKCS#1 RSA private key PEM (the hub
// hands out /privatekey in either format through openssl-compatible tooling).
func parsePrivateKeyPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return rsaKey, nil
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("not a PKCS#8/PKCS#1 RSA private key")
	}
	return rsaKey, nil
}

// publicPEM renders an RSA public key as strict X.509 SubjectPublicKeyInfo
// PEM (authlib-injector exits the game on malformed PEM).
func publicPEM(pub *rsa.PublicKey) (string, []byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", nil, err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), der, nil
}

// writeError emits the legacy error body {error,errorMessage,cause?}.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "errorMessage": msg})
}

// writeJSON emits a JSON response body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
