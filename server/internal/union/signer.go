package union

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	mrand "math/rand"
	"sync"
	"sync/atomic"
	"time"

	"skin-nexus/server/internal/ygg"
)

// UnionSigner is the hub-distributed texture signing key (analysis/
// yggdrasil-connect.md §4.2 "GET /privatekey" + §7.1-5). It implements
// ygg.Signer so the yggdrasil surface can swap LocalSigner for it without
// touching the compatibility layer.
//
// Dual state:
//
//	union-managed  a hub-issued key is active (Mode() == ModeUnionManaged)
//	local          no hub key yet: delegate to the injected fallback signer
//
// Rotation policy (task H2): poll GET /privatekey on a 10-15 minute base
// interval plus jitter; switch only when the key fingerprint actually
// changes; after a switch every new signature uses the new key only (old
// signatures stay valid, no re-signing). More than 3 switches in one day is
// treated as an anomaly: automatic rotation suspends and an audit alert is
// emitted until an operator calls ResumeRotation.
type UnionSigner struct {
	out      *Outbound
	fallback ygg.Signer
	audit    AuditFunc
	now      func() time.Time
	enabled  bool

	cur atomic.Pointer[signingKey] // nil => fallback mode; lock-free reads

	mu          sync.Mutex
	loopRunning bool
	suspended   bool
	day         time.Time // UTC day bucket of the change counter
	changes     int
	rotCtx      context.Context
	rotBase     time.Duration
}

// signingKey is one immutable key state; swaps are atomic.
type signingKey struct {
	key     *rsa.PrivateKey
	pubPEM  string
	fp      string
	version string
}

// Mode() values for the admin panel.
const (
	ModeDisabled      = "disabled"
	ModeLocalFallback = "local"
	ModeUnionManaged  = "union-managed"
)

// Rotation tuning. Vars (not consts) so tests can drive the loop fast; the
// defaults are the approved policy window (task H2: 10-15min + jitter).
var (
	rotationIntervalMin = 10 * time.Minute
	rotationIntervalMax = 15 * time.Minute
	rotationJitterMax   = 90 * time.Second
)

// maxKeyChangesPerDay is the automatic-rotation anomaly budget
// ("日变更 > 3 次 → 停自动轮换").
const maxKeyChangesPerDay = 3

// Compile-time interface check.
var _ ygg.Signer = (*UnionSigner)(nil)

// NewUnionSigner builds the dual-state signer. fallback may be nil (then a
// missing hub key is a hard error at signing time). out may be nil: rotation
// then reports ErrNotConfigured while SetKey keeps working.
func NewUnionSigner(cfg Config, out *Outbound, fallback ygg.Signer, audit AuditFunc) *UnionSigner {
	return &UnionSigner{
		out:      out,
		fallback: fallback,
		audit:    audit,
		now:      time.Now,
		enabled:  cfg.Enabled,
	}
}

// Enabled reports whether the signer does real work (nil-safe).
func (s *UnionSigner) Enabled() bool { return s != nil && s.enabled }

// Mode reports the active key source for the admin panel.
func (s *UnionSigner) Mode() string {
	if !s.Enabled() {
		return ModeDisabled
	}
	if s.cur.Load() != nil {
		return ModeUnionManaged
	}
	return ModeLocalFallback
}

// PrivateKeyVersion reports the hub-issued key version ("0"/empty when none).
func (s *UnionSigner) PrivateKeyVersion() string {
	if s == nil {
		return ""
	}
	if k := s.cur.Load(); k != nil {
		return k.version
	}
	return ""
}

// SignValue signs a textures property value (SHA1withRSA, base64) exactly
// like LocalSigner — the algorithm is fixed by the Yggdrasil compatibility
// surface (§7.1-5); only the key source differs.
func (s *UnionSigner) SignValue(value string) (string, error) {
	if !s.Enabled() {
		return "", ErrDisabled
	}
	if k := s.cur.Load(); k != nil { // one-way switch: new signatures use the new key
		return signSHA1(k.key, value)
	}
	if s.fallback != nil {
		return s.fallback.SignValue(value)
	}
	return "", ErrNoSigningKey
}

// PublicKeyPEM returns the X.509 SubjectPublicKeyInfo PEM of the active key.
func (s *UnionSigner) PublicKeyPEM() string {
	if !s.Enabled() {
		return ""
	}
	if k := s.cur.Load(); k != nil {
		return k.pubPEM
	}
	if s.fallback != nil {
		return s.fallback.PublicKeyPEM()
	}
	return ""
}

// KeyFingerprint identifies the active key (same SHA256-prefix format as
// internal/ygg so panel/audit output is comparable across signers).
func (s *UnionSigner) KeyFingerprint() string {
	if !s.Enabled() {
		return ""
	}
	if k := s.cur.Load(); k != nil {
		return k.fp
	}
	if s.fallback != nil {
		return s.fallback.KeyFingerprint()
	}
	return ""
}

// SetKey installs a hub-issued (or operator-supplied) key manually. The key
// may be PKCS#8 or PKCS#1 PEM. Manual switches do not consume the automatic
// rotation budget — an operator acting deliberately is the human review the
// suspension asks for — but they do not lift a suspension either; call
// ResumeRotation to re-arm polling.
func (s *UnionSigner) SetKey(pemStr, version string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	key, err := parsePrivateKeyPEM(pemStr)
	if err != nil {
		return fmt.Errorf("union: signing key: %w", err)
	}
	pub, der, err := publicPEM(&key.PublicKey)
	if err != nil {
		return err
	}
	sk := &signingKey{key: key, pubPEM: pub, fp: fingerprint(der), version: version}
	s.cur.Store(sk)
	s.audit.call("union.signer.key.set", map[string]any{"version": version, "fingerprint": sk.fp})
	return nil
}

// RotationSuspended reports whether automatic rotation is halted (panel).
func (s *UnionSigner) RotationSuspended() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.suspended
}

// StartRotation launches the polling loop. intervalBase must sit inside the
// approved 10-15 minute window (task H2). Idempotence: a second call while
// running is refused.
func (s *UnionSigner) StartRotation(ctx context.Context, intervalBase time.Duration) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	if intervalBase < rotationIntervalMin || intervalBase > rotationIntervalMax {
		return fmt.Errorf("union: rotation interval %s outside [%s,%s]",
			intervalBase, rotationIntervalMin, rotationIntervalMax)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loopRunning {
		return errors.New("union: rotation loop already running")
	}
	s.rotCtx, s.rotBase = ctx, intervalBase
	s.loopRunning = true
	go s.rotationLoop(ctx, intervalBase)
	return nil
}

// ResumeRotation lifts a suspension after human review and re-arms the
// polling loop when one is configured and its context is still alive.
func (s *UnionSigner) ResumeRotation() error {
	if !s.Enabled() {
		return ErrDisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.suspended {
		return nil
	}
	s.suspended = false
	s.changes = 0
	s.audit.call("union.signer.rotation.resumed", nil)
	if s.rotCtx != nil && s.rotCtx.Err() == nil && s.rotBase > 0 && !s.loopRunning {
		s.loopRunning = true
		go s.rotationLoop(s.rotCtx, s.rotBase)
	}
	return nil
}

// rotationLoop polls with jitter until the context ends or rotation suspends.
func (s *UnionSigner) rotationLoop(ctx context.Context, base time.Duration) {
	defer func() {
		s.mu.Lock()
		s.loopRunning = false
		s.mu.Unlock()
	}()
	for {
		wait := base
		if rotationJitterMax > 0 {
			wait += time.Duration(mrand.Int63n(int64(rotationJitterMax)))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if _, err := s.rotateOnce(ctx); err != nil {
			switch {
			case errors.Is(err, ErrRotationSuspended), errors.Is(err, ErrDisabled), errors.Is(err, ErrNotConfigured):
				return // halt: needs human action or reconfiguration
			}
			s.audit.call("union.signer.rotation.error", map[string]any{"error": err.Error()})
		}
	}
}

// rotateOnce performs one poll/compare/switch cycle (also the unit under
// test). Reports whether the key changed.
func (s *UnionSigner) rotateOnce(ctx context.Context) (bool, error) {
	if !s.Enabled() {
		return false, ErrDisabled
	}
	s.mu.Lock()
	suspended := s.suspended
	s.mu.Unlock()
	if suspended {
		return false, ErrRotationSuspended
	}
	if s.out == nil {
		return false, ErrNotConfigured
	}
	doc, err := s.out.Privatekey(ctx)
	if err != nil {
		return false, err
	}
	key, err := parsePrivateKeyPEM(doc.PrivateKey)
	if err != nil {
		return false, fmt.Errorf("union: hub private key rejected: %w", err)
	}
	pub, der, err := publicPEM(&key.PublicKey)
	if err != nil {
		return false, err
	}
	fp := fingerprint(der)

	s.mu.Lock()
	defer s.mu.Unlock()
	if cur := s.cur.Load(); cur != nil && cur.fp == fp {
		return false, nil // switch only on fingerprint difference
	}
	now := s.now()
	if s.changesTodayLocked(now) >= maxKeyChangesPerDay {
		s.suspended = true
		s.audit.call("union.signer.rotation.suspended", map[string]any{
			"reason":   "daily_change_budget_exceeded",
			"changes":  s.changes,
			"limit":    maxKeyChangesPerDay,
			"previous": s.KeyFingerprint(),
			"incoming": fp,
		})
		return false, ErrRotationSuspended
	}
	s.changes++
	s.cur.Store(&signingKey{key: key, pubPEM: pub, fp: fp, version: doc.PrivateKeyVersion})
	s.audit.call("union.signer.rotation.switched", map[string]any{
		"version":     doc.PrivateKeyVersion,
		"fingerprint": fp,
		"changes":     s.changes,
	})
	return true, nil
}

// changesTodayLocked resets the day bucket when the UTC day rolled over and
// returns the current change count. Caller holds s.mu.
func (s *UnionSigner) changesTodayLocked(now time.Time) int {
	day := now.UTC().Truncate(24 * time.Hour)
	if !day.Equal(s.day) {
		s.day = day
		s.changes = 0
	}
	return s.changes
}

// signSHA1 is RSA-SHA1 (PKCS#1 v1.5) over the value, Base64 out — identical
// to ygg.LocalSigner.SignValue.
func signSHA1(key *rsa.PrivateKey, value string) (string, error) {
	digest := sha1.Sum([]byte(value))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// fingerprint is the SHA-256 prefix of the DER public key — byte-compatible
// with ygg.fingerprint (kept local: unexported there).
func fingerprint(der []byte) string {
	h := sha256.Sum256(der)
	return fmt.Sprintf("SHA256:%x", h[:8])
}
