package main

import (
	"context"

	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/store"
)

type pluginSessionResolver struct{ st *store.Mysql }

func (r pluginSessionResolver) ResolveSession(ctx context.Context, token string) (*plugin.SessionIdentity, error) {
	if token == "" {
		return nil, store.ErrNotFound
	}
	uid, err := r.st.SessionUser(ctx, token)
	if err != nil || uid <= 0 {
		return nil, store.ErrNotFound
	}
	perm, err := r.st.UserPermission(ctx, uid)
	if err != nil {
		return nil, err
	}
	brief, err := r.st.UserBrief(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &plugin.SessionIdentity{ID: uid, Nickname: brief.Nickname, Permission: perm}, nil
}

var _ plugin.SessionResolver = pluginSessionResolver{}
