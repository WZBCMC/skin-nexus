package store

import (
	"context"

	"skin-nexus/server/internal/oidc"
)

// UserBrief supplies the OIDC claim builder with nickname/email.
func (m *Mysql) UserBrief(ctx context.Context, uid int64) (oidc.UserBrief, error) {
	var b oidc.UserBrief
	// email_verified carries the real column (chain-trace P2: it was
	// hardcoded true, so the OIDC claim lied for unverified users).
	err := m.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(nickname,''), COALESCE(email,''), email_verified FROM users WHERE id = ?`, uid).
		Scan(&b.ID, &b.Nickname, &b.Email, &b.EmailVerified)
	if err != nil {
		return oidc.UserBrief{}, err
	}
	return b, nil
}
