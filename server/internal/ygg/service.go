package ygg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Errors map to the legacy response semantics (docs/phase1-dev-plan.md §1.3).
var (
	ErrBadCredentials   = errors.New("ygg: invalid credentials")     // 403 ForbiddenOperationException
	ErrTokenInvalid     = errors.New("ygg: invalid token")           // 403
	ErrProfileNotFound  = errors.New("ygg: profile not found")       // 204
	ErrSessionNotFound  = errors.New("ygg: session not found")       // 204
	ErrEmailUnverified  = errors.New("ygg: email not verified")      // 403 (join gate, fork behavior)
)

// ---- storage ports (DB-backed impls land in M1; memory impls in tests) ----

// User is the auth-side user record.
type User struct {
	ID            int64
	Nickname      string
	EmailVerified bool
	CanJoinServer bool
}

// Player is one profile (character) owned by a user.
type Player struct {
	ID       int64
	UID      int64
	Name     string
	UUID     string // 32 lowercase hex
	SkinURL  string
	SkinModel string
	CapeURL  string
}

// Token is an issued access token (opaque string; format intentionally
// differs from legacy Passport JWT — all tokens die at cutover).
type Token struct {
	AccessToken  string
	ClientToken  string
	UserID       int64
	SelectedPID  int64 // 0 = none selected
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Invalidated  bool
}

// JoinSession is a join/hasJoined session (TTL 120s, legacy behavior).
type JoinSession struct {
	AccessToken string
	UUID        string
	ServerID    string
	ServerIP    string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type Store interface {
	UserByID(ctx context.Context, id int64) (User, error)
	UserByIdentifier(ctx context.Context, ident string) (User, error) // email or nickname
	VerifyPassword(ctx context.Context, userID int64, password string) (bool, error)
	PlayersByUID(ctx context.Context, uid int64) ([]Player, error)
	PlayerByUUID(ctx context.Context, uuid string) (Player, error)
	PlayerByName(ctx context.Context, name string) (Player, error)
	SaveToken(ctx context.Context, t Token) error
	TokenByAccess(ctx context.Context, access string) (Token, error)
	InvalidateToken(ctx context.Context, access string) error
	InvalidateUserTokens(ctx context.Context, uid int64) error
	SaveJoin(ctx context.Context, j JoinSession) error
	JoinByTokenServer(ctx context.Context, access, serverID string) (JoinSession, error)
}

// Service implements Yggdrasil session logic. Join-session TTL follows the
// legacy deployment: 120s (not the 30s spec suggestion — see §1.7).
type Service struct {
	store     Store
	signer    Signer
	joinTTL   time.Duration
	tokenTTL  time.Duration
	now       func() time.Time
}

const (
	DefaultJoinTTL  = 120 * time.Second
	DefaultTokenTTL = 30 * 24 * time.Hour
)

func NewService(store Store, signer Signer) *Service {
	return &Service{
		store:    store,
		signer:   signer,
		joinTTL:  DefaultJoinTTL,
		tokenTTL: DefaultTokenTTL,
		now:      time.Now,
	}
}

// Authenticate issues a token pair (authserver/authenticate).
func (s *Service) Authenticate(ctx context.Context, username, password, clientToken string, withProfiles bool) (Token, []Player, error) {
	u, err := s.store.UserByIdentifier(ctx, username)
	if err != nil {
		return Token{}, nil, ErrBadCredentials
	}
	ok, err := s.store.VerifyPassword(ctx, u.ID, password)
	if err != nil || !ok {
		return Token{}, nil, ErrBadCredentials
	}
	if clientToken == "" {
		clientToken = randomHex(16)
	}
	players, _ := s.store.PlayersByUID(ctx, u.ID)
	t := Token{
		AccessToken: randomHex(32),
		ClientToken: clientToken,
		UserID:      u.ID,
		CreatedAt:   s.now(),
		ExpiresAt:   s.now().Add(s.tokenTTL),
	}
	if err := s.store.SaveToken(ctx, t); err != nil {
		return Token{}, nil, err
	}
	return t, players, nil
}

// Refresh re-issues an access token (authserver/refresh).
func (s *Service) Refresh(ctx context.Context, accessToken, clientToken string, selectPID int64) (Token, []Player, error) {
	old, err := s.store.TokenByAccess(ctx, accessToken)
	if err != nil || old.Invalidated || s.now().After(old.ExpiresAt) {
		return Token{}, nil, ErrTokenInvalid
	}
	if clientToken != "" && clientToken != old.ClientToken {
		return Token{}, nil, ErrTokenInvalid
	}
	// Legacy semantics: a refresh invalidates the previous token.
	_ = s.store.InvalidateToken(ctx, accessToken)
	t := Token{
		AccessToken: randomHex(32),
		ClientToken: old.ClientToken,
		UserID:      old.UserID,
		SelectedPID: selectPID,
		CreatedAt:   s.now(),
		ExpiresAt:   s.now().Add(s.tokenTTL),
	}
	if t.SelectedPID == 0 {
		t.SelectedPID = old.SelectedPID
	}
	if err := s.store.SaveToken(ctx, t); err != nil {
		return Token{}, nil, err
	}
	players, _ := s.store.PlayersByUID(ctx, t.UserID)
	return t, players, nil
}

// Validate reports whether a token is currently usable (204 vs 403).
func (s *Service) Validate(ctx context.Context, accessToken, clientToken string) error {
	t, err := s.store.TokenByAccess(ctx, accessToken)
	if err != nil || t.Invalidated || s.now().After(t.ExpiresAt) {
		return ErrTokenInvalid
	}
	if clientToken != "" && clientToken != t.ClientToken {
		return ErrTokenInvalid
	}
	return nil
}

// Invalidate kills one token (always 204 at the HTTP layer).
func (s *Service) Invalidate(ctx context.Context, accessToken string) {
	_ = s.store.InvalidateToken(ctx, accessToken)
}

// SignOut kills all tokens of a user after password check (always 204).
func (s *Service) SignOut(ctx context.Context, username, password string) {
	u, err := s.store.UserByIdentifier(ctx, username)
	if err != nil {
		return
	}
	if ok, _ := s.store.VerifyPassword(ctx, u.ID, password); ok {
		_ = s.store.InvalidateUserTokens(ctx, u.ID)
	}
}

// Join records a join session (sessionserver/join, 204 at HTTP layer).
// Fork behavior preserved: unverified email may not join (ErrEmailUnverified).
func (s *Service) Join(ctx context.Context, accessToken, playerUUID, serverID, serverIP string) error {
	t, err := s.store.TokenByAccess(ctx, accessToken)
	if err != nil || t.Invalidated || s.now().After(t.ExpiresAt) {
		return ErrTokenInvalid
	}
	u, err := s.store.UserByID(ctx, t.UserID)
	if err != nil {
		return ErrTokenInvalid
	}
	if !u.EmailVerified {
		return ErrEmailUnverified
	}
	if !u.CanJoinServer {
		return ErrTokenInvalid
	}
	return s.store.SaveJoin(ctx, JoinSession{
		AccessToken: accessToken,
		UUID:        strings.ToLower(playerUUID),
		ServerID:    serverID,
		ServerIP:    serverIP,
		CreatedAt:   s.now(),
		ExpiresAt:   s.now().Add(s.joinTTL),
	})
}

// HasJoined resolves a join session to a signed profile.
// Returns ErrSessionNotFound (HTTP 204) on: unknown session, expired,
// serverId mismatch, or IP mismatch (fork behavior).
func (s *Service) HasJoined(ctx context.Context, username, serverID, serverIP string) (Profile, error) {
	p, err := s.store.PlayerByName(ctx, username)
	if err != nil {
		return Profile{}, ErrProfileNotFound
	}
	// Find the most recent live join session for this player.
	// (Store scan is O(sessions); acceptable — TTL 120s bounds the table.)
	js, err := s.findLiveJoin(ctx, p, serverID, serverIP)
	if err != nil {
		return Profile{}, err
	}
	_ = js
	return s.buildProfile(ctx, p, true) // hasJoined always signs
}

func (s *Service) findLiveJoin(ctx context.Context, p Player, serverID, serverIP string) (JoinSession, error) {
	type scanner interface {
		FindLiveJoins(ctx context.Context, uuid string, now time.Time) ([]JoinSession, error)
	}
	sc, ok := s.store.(scanner)
	if !ok {
		return JoinSession{}, ErrSessionNotFound
	}
	joins, err := sc.FindLiveJoins(ctx, p.UUID, s.now())
	if err != nil {
		return JoinSession{}, ErrSessionNotFound
	}
	for _, j := range joins {
		if serverID != "" && j.ServerID != serverID {
			continue
		}
		if serverIP != "" && j.ServerIP != "" && j.ServerIP != serverIP {
			continue
		}
		return j, nil
	}
	return JoinSession{}, ErrSessionNotFound
}

// ProfileByUUID serves sessionserver/profile/{uuid}.
// unsigned=false triggers property signatures; missing profile = ErrProfileNotFound (204).
func (s *Service) ProfileByUUID(ctx context.Context, uuid string, withSig bool) (Profile, error) {
	p, err := s.store.PlayerByUUID(ctx, strings.ToLower(uuid))
	if err != nil {
		return Profile{}, ErrProfileNotFound
	}
	return s.buildProfile(ctx, p, withSig)
}

// LookupByName serves api/users/profiles/minecraft/{name} (no properties).
func (s *Service) LookupByName(ctx context.Context, name string) (Player, error) {
	return s.store.PlayerByName(ctx, name)
}

// LookupBulk serves api/profiles/minecraft (no properties).
func (s *Service) LookupBulk(ctx context.Context, names []string) []Player {
	out := make([]Player, 0, len(names))
	for _, n := range names {
		if p, err := s.store.PlayerByName(ctx, n); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *Service) buildProfile(ctx context.Context, p Player, withSig bool) (Profile, error) {
	props, err := AssembleTextures(p.UUID, p.Name, TextureSet{
		SkinURL: p.SkinURL, SkinModel: p.SkinModel, CapeURL: p.CapeURL,
	}, s.now(), withSig, s.signer)
	if err != nil {
		return Profile{}, err
	}
	return Profile{ID: p.UUID, Name: p.Name, Properties: props}, nil
}

// PublicKeyPEM exposes the metadata signing key.
func (s *Service) PublicKeyPEM() string { return s.signer.PublicKeyPEM() }

// SignerFingerprint exposes the active key fingerprint for admin UI/audit.
func (s *Service) SignerFingerprint() string { return s.signer.KeyFingerprint() }

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
