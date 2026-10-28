package main

// pluginbearer.go bridges machine-client Bearer tokens to plugin route
// identities. Both OIDC access tokens and opaque Yggdrasil tokens resolve
// through the same ledger the game endpoints use.
import (
	"context"
	"time"

	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/ygg"
)

type pluginBearerResolver struct {
	ygg *ygg.Service
	st  *store.Mysql
}

func (r pluginBearerResolver) ResolveBearer(ctx context.Context, token string) (*plugin.SessionIdentity, error) {
	t, err := r.ygg.ResolveToken(ctx, token)
	if err != nil || t.UserID == 0 || t.Invalidated || t.ExpiresAt.IsZero() || time.Now().After(t.ExpiresAt) {
		return nil, &plugin.BearerError{
			Status: 401, Code: "auth.invalid_token", Message: "invalid or expired token",
		}
	}
	u, err := r.st.UserByID(ctx, t.UserID)
	if err != nil {
		return nil, &plugin.BearerError{Status: 401, Code: "auth.invalid_token", Message: "unknown account"}
	}
	perm, err := r.st.UserPermission(ctx, t.UserID)
	if err != nil {
		return nil, &plugin.BearerError{Status: 401, Code: "auth.invalid_token", Message: "unknown account"}
	}
	if perm < 0 {
		return nil, &plugin.BearerError{Status: 403, Code: "auth.account_disabled", Message: "account disabled"}
	}
	return &plugin.SessionIdentity{ID: u.ID, Nickname: u.Nickname, Permission: perm}, nil
}
