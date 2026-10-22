package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"skin-nexus/server/internal/ygg"
)

// ScopeYggdrasilJoin is the scope that authorises a token to be used as a
// Yggdrasil session token (game join). Launcher scope set is fixed to
// openid/offline_access/PlayerProfiles.Select/Server.Join (both launchers).
const ScopeYggdrasilJoin = "Yggdrasil.Server.Join"

// ErrNotJOSE reports that the presented credential is not a JWT (callers fall
// back to the opaque yggdrasil token lookup).
var ErrNotJOSE = errors.New("oidc: not a JWT")

// NewYggTokenResolver returns a resolver for ygg.Service.SetTokenResolver.
// It accepts OIDC-issued RS256 JWT access tokens as Yggdrasil tokens — the
// dual-state requirement proven by both launchers: the OAuth access_token is
// passed to the game verbatim and used against /sessionserver/join and
// /authserver/validate.
//
// Acceptance rules (analysis/oidc-client-contract-synthesis.md §4):
//   - signature RS256 against the provider signing key;
//   - iss == configured issuer, exp within 60s skew;
//   - jti present in the ledger and not revoked (revocation covers password
//     change / signout);
//   - scope contains Yggdrasil.Server.Join (a token without join scope is not
//     a Yggdrasil credential at all);
//   - the returned ygg.Token uses the normalised key "jti:<jti>" as
//     AccessToken so join sessions stay short-keyed.
func NewYggTokenResolver(backend Backend, issuer string, pub *rsa.PublicKey) func(context.Context, string) (ygg.Token, error) {
	return func(ctx context.Context, raw string) (ygg.Token, error) {
		return resolveJWT(ctx, backend, issuer, pub, raw, true)
	}
}

// VerifyBearer verifies an access token for the userinfo endpoint (no join
// scope requirement) and returns its ledger record.
func VerifyBearer(ctx context.Context, backend Backend, issuer string, pub *rsa.PublicKey, raw string) (AccessTokenRecord, error) {
	rec, _, err := verifyJWT(ctx, backend, issuer, pub, raw, false)
	if err != nil {
		return AccessTokenRecord{}, err
	}
	return rec, nil
}

func resolveJWT(ctx context.Context, backend Backend, issuer string, pub *rsa.PublicKey, raw string, requireJoinScope bool) (ygg.Token, error) {
	rec, _, err := verifyJWT(ctx, backend, issuer, pub, raw, requireJoinScope)
	if err != nil {
		return ygg.Token{}, err
	}
	return ygg.Token{
		AccessToken: JWTKey(rec.JTI),
		UserID:      rec.UserID,
		SelectedPID: rec.SelectedPID,
		CreatedAt:   rec.CreatedAt,
		ExpiresAt:   rec.ExpiresAt,
	}, nil
}

type jwtClaims struct {
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`
	Exp     int64  `json:"exp"`
	Iat     int64  `json:"iat"`
	JTI     string `json:"jti"`
	Scope   string `json:"scope"`
}

// verifyJWT is the single verification path: signature first, then claims,
// then the jti ledger. There is intentionally no "skip signature" fallback.
func verifyJWT(ctx context.Context, backend Backend, issuer string, pub *rsa.PublicKey, raw string, requireJoinScope bool) (AccessTokenRecord, jwtClaims, error) {
	var zero AccessTokenRecord
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return zero, jwtClaims{}, ErrNotJOSE
	}
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, jwtClaims{}, ErrNotJOSE
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(header, &hdr); err != nil || hdr.Alg != "RS256" {
		return zero, jwtClaims{}, ErrNotJOSE
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	h := crypto.SHA256.New()
	_, _ = h.Write([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h.Sum(nil), sig); err != nil {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	if claims.Issuer != issuer || claims.JTI == "" || claims.Subject == "" {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	now := time.Now()
	if now.After(time.Unix(claims.Exp, 0).Add(60 * time.Second)) {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}

	rec, err := backend.GetAccessToken(ctx, claims.JTI)
	if err != nil {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	if !rec.RevokedAt.IsZero() || now.After(rec.ExpiresAt) {
		return zero, jwtClaims{}, ygg.ErrTokenInvalid
	}
	// Join-scope gate. The ledger record is authoritative: zitadel v3 JWT
	// access tokens may omit the "scope" claim entirely (CreateJWT never sets
	// Scopes), so the claim is only a fallback.
	if requireJoinScope {
		granted := scopeGranted(claims.Scope, ScopeYggdrasilJoin) ||
			scopeGranted(strings.Join(rec.Scopes, " "), ScopeYggdrasilJoin)
		if !granted {
			return zero, jwtClaims{}, ygg.ErrTokenInvalid
		}
	}
	return rec, claims, nil
}

// JWTKey normalises a JWT to the short key used in join-session rows.
func JWTKey(jti string) string { return "jti:" + jti }

func scopeGranted(scopeList, want string) bool {
	for _, s := range strings.Fields(scopeList) {
		if s == want {
			return true
		}
	}
	return false
}
