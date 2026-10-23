package store

import (
	"context"
	"database/sql"
	"errors"
)

// Admin user management (P1 管理面余项, 2026-10-23).

// AdminSetPassword overwrites a user's hash (argon2id) — admin reset path.
// The caller (adminapi) fires credential-change side effects (ygg + OIDC
// revocation) like the self-service change does.
func (m *Mysql) AdminSetPassword(ctx context.Context, uid int64, hash string) error {
	res, err := m.db.ExecContext(ctx,
		`UPDATE users SET password = ?, password_algo = 'argon2id' WHERE id = ?`, hash, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// row exists with identical hash → still success (changed-rows)
		var one int
		if err := m.db.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ?`, uid).Scan(&one); err != nil {
			return ErrNotFound
		}
	}
	return nil
}

// DeleteUser removes a user and every credential/session row that would let
// the account keep working, in one transaction:
//
//	web_sessions / ygg_tokens / oidc refresh+access / user_closet /
//	uuid_map of their players / players / the users row last.
//
// textures.uid (uploaded library files) and audit_log.actor_uid are
// deliberately KEPT — shared-library + audit-history semantics (legacy had
// no cascades either; orphan visibility beats silent data loss).
// ygg_join_sessions has no uid column (120s TTL, unusable once tokens die).
func (m *Mysql) DeleteUser(ctx context.Context, uid int64) error {
	var one int
	if err := m.db.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ?`, uid).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []struct {
		q string
	}{
		{`DELETE FROM web_sessions WHERE user_id = ?`},
		{`DELETE FROM ygg_tokens WHERE user_id = ?`},
		{`DELETE FROM oidc_refresh_tokens WHERE uid = ?`},
		{`DELETE FROM oidc_access_tokens WHERE uid = ?`},
		{`DELETE FROM user_closet WHERE user_uid = ?`},
		{`DELETE FROM uuid_map WHERE player_id IN (SELECT id FROM players WHERE uid = ?)`},
		{`DELETE FROM players WHERE uid = ?`},
		{`DELETE FROM users WHERE id = ?`},
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s.q, uid); err != nil {
			return err
		}
	}
	return tx.Commit()
}
