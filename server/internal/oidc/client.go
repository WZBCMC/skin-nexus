package oidc

import (
	"time"

	oidclib "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// client adapts ClientRecord to the zitadel op.Client interface.
//
// Contract notes (analysis/oidc-client-contract-synthesis.md):
//   - launchers are public clients (discovery shared_client_id, no secret):
//     AuthMethodNone + device_code grant;
//   - access tokens are RS256 JWTs reused verbatim as Yggdrasil tokens, so
//     AccessTokenType must be JWT;
//   - id_token must carry custom claims (selectedProfile), hence
//     IDTokenUserinfoClaimsAssertion() is true;
//   - implicit grant is intentionally unsupported (2026-10-23 scope narrowing).
type client struct {
	rec      ClientRecord
	loginURL string
}

func newClient(rec ClientRecord, loginURL string) *client {
	return &client{rec: rec, loginURL: loginURL}
}

func (c *client) GetID() string { return c.rec.ID }

func (c *client) RedirectURIs() []string { return c.rec.RedirectURIs }

func (c *client) PostLogoutRedirectURIs() []string { return nil }

func (c *client) ApplicationType() op.ApplicationType {
	if c.rec.SecretHash == "" {
		// Public client (launcher): native app semantics allow loopback
		// redirect URIs for the P1 authorization-code flow.
		return op.ApplicationTypeNative
	}
	return op.ApplicationTypeWeb
}

func (c *client) AuthMethod() oidclib.AuthMethod {
	if c.rec.SecretHash == "" {
		return oidclib.AuthMethodNone
	}
	return oidclib.AuthMethodPost // legacy client_secret_post compatibility
}

func (c *client) ResponseTypes() []oidclib.ResponseType {
	return []oidclib.ResponseType{oidclib.ResponseTypeCode}
}

func (c *client) GrantTypes() []oidclib.GrantType {
	out := make([]oidclib.GrantType, 0, len(c.rec.GrantTypes))
	for _, g := range c.rec.GrantTypes {
		out = append(out, oidclib.GrantType(g))
	}
	return out
}

func (c *client) LoginURL(string) string { return c.loginURL }

func (c *client) AccessTokenType() op.AccessTokenType {
	return op.AccessTokenTypeJWT
}

func (c *client) IDTokenLifetime() time.Duration { return time.Hour }

func (c *client) DevMode() bool { return false }

func passthroughScopes(scopes []string) []string { return scopes }

func (c *client) RestrictAdditionalIdTokenScopes() func([]string) []string {
	return passthroughScopes
}

func (c *client) RestrictAdditionalAccessTokenScopes() func([]string) []string {
	return passthroughScopes
}

func (c *client) IsScopeAllowed(scope string) bool {
	if len(c.rec.Scopes) == 0 {
		return true // empty allow-list = all supported scopes
	}
	for _, s := range c.rec.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (c *client) IDTokenUserinfoClaimsAssertion() bool {
	// Merge userinfo claims (selectedProfile, availableProfiles, nickname,
	// picture, ...) into the id_token — SJMCL hard-requires selectedProfile.
	return true
}

func (c *client) ClockSkew() time.Duration { return 60 * time.Second }
