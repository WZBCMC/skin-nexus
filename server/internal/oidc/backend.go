// Package oidc is the built-in OAuth2/OIDC provider (yggc/OIDC passwordless
// login, replacing the legacy Janus sidecar). Client contract is pinned in
// analysis/oidc-client-contract-synthesis.md; provider engine is zitadel/oidc
// v3 (selection record: analysis/oidc-lib-eval.md).
//
// backend.go defines the persistence contract shared by the DB implementation
// (internal/store) and in-memory test doubles. No zitadel types leak here.
package oidc

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Backend getters when a record does not exist.
var ErrNotFound = errors.New("oidc: not found")

// ClientRecord is a registered OAuth client. The shared public client used by
// launchers (discovery `shared_client_id`) is a row with IsPublic and no secret.
type ClientRecord struct {
	ID           string
	SecretHash   string // "" for public clients (verified via AuthorizeClientIDSecret hook)
	Name         string
	RedirectURIs []string // exact-match only (contract: redirect_uri 精确匹配)
	GrantTypes   []string // oidc grant type strings, e.g. "urn:ietf:params:oauth:grant-type:device_code"
	Scopes       []string // allowed scopes; empty = all supported
}

// AuthRequestRecord backs the authorization-code flow (P1) including PKCE.
type AuthRequestRecord struct {
	ID                  string
	ClientID            string
	Subject             string // set once the user authenticates (uid as decimal string)
	Scopes              []string
	RedirectURI         string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string // "S256" (plain rejected by policy)
	SelectedPID         int64  // profile chosen at the consent/selection step; 0 = none
	AuthTime            time.Time
	Done                bool
	Code                string
	CodeExpires         time.Time
	CreatedAt           time.Time
}

// AccessTokenRecord is the JWT access-token ledger entry (jti). Revocation is
// checked on every Bearer verification; user-wide revocation covers password
// change and legacy signout semantics.
type AccessTokenRecord struct {
	JTI         string
	UserID      int64
	ClientID    string
	Scopes      []string
	SelectedPID int64
	ExpiresAt   time.Time
	CreatedAt   time.Time
	RevokedAt   time.Time // zero = live
}

// RefreshTokenRecord stores opaque refresh tokens. Rotation revokes the old
// row and inserts the new one; token strings never appear in logs.
type RefreshTokenRecord struct {
	Token       string
	UserID      int64
	ClientID    string
	Scopes      []string
	SelectedPID int64
	AuthTime    time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
	Revoked     bool
}

// DeviceAuthRecord backs RFC 8628 device flow state. Done/Denied are flipped
// by the site's verification page (login + consent + profile select).
type DeviceAuthRecord struct {
	DeviceCode  string
	UserCode    string // XXXX-XXXX, base20 charset (matches live Janus shape)
	ClientID    string
	Scopes      []string
	ExpiresAt   time.Time
	Done        bool
	Denied      bool
	UserID      int64 // 0 until approved
	SelectedPID int64
	AuthTime    time.Time
	CreatedAt   time.Time
}

// Backend is the persistence contract. Implementations: internal/store (MySQL)
// and in-memory doubles in tests. All methods must be safe for concurrent use.
type Backend interface {
	// clients
	GetClient(ctx context.Context, id string) (ClientRecord, error)

	// authorization-code flow (P1; wired now so grants share one ledger)
	SaveAuthRequest(ctx context.Context, rec AuthRequestRecord) error
	GetAuthRequest(ctx context.Context, id string) (AuthRequestRecord, error)
	GetAuthRequestByCode(ctx context.Context, code string) (AuthRequestRecord, error)
	DeleteAuthRequest(ctx context.Context, id string) error

	// access tokens (jti ledger)
	SaveAccessToken(ctx context.Context, rec AccessTokenRecord) error
	GetAccessToken(ctx context.Context, jti string) (AccessTokenRecord, error)
	RevokeAccessToken(ctx context.Context, jti string) error
	RevokeUserAccessTokens(ctx context.Context, uid int64) error

	// refresh tokens
	SaveRefreshToken(ctx context.Context, rec RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, token string) (RefreshTokenRecord, error)
	// RotateRefreshToken revokes old and stores new atomically.
	RotateRefreshToken(ctx context.Context, old, new RefreshTokenRecord) error
	RevokeRefreshToken(ctx context.Context, token string) error

	// device flow
	// SaveDeviceAuth must return ErrDuplicateUserCode semantics via
	// errors.Is(err, ErrDuplicateUserCode) when UserCode collides.
	SaveDeviceAuth(ctx context.Context, rec DeviceAuthRecord) error
	GetDeviceAuthByDeviceCode(ctx context.Context, code string) (DeviceAuthRecord, error)
	GetDeviceAuthByUserCode(ctx context.Context, code string) (DeviceAuthRecord, error)
	UpdateDeviceAuth(ctx context.Context, rec DeviceAuthRecord) error

	// health probe
	Health(ctx context.Context) error
}

// ErrDuplicateUserCode tells the caller to regenerate a user code
// (mirrors op.ErrDuplicateUserCode semantics).
var ErrDuplicateUserCode = errors.New("oidc: user code already exists")
