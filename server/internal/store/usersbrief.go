package store

import (
	"context"

	"skin-nexus/server/internal/oidc"
)

// UserBrief supplies the OIDC claim builder with nickname/email.
func (m *Mysql) UserBrief(ctx context.Context, uid int64) (oidc.UserBrief, error) {
	var b oidc.UserBrief
	err := m.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(nickname,''), COALESCE(email,'') FROM users WHERE id = ?`, uid).
		Scan(&b.ID, &b.Nickname, &b.Email)
	if err != nil {
		return oidc.UserBrief{}, err
	}
	b.EmailVerified = true
	return b, nil
}
