package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Password reset tokens (A1 找回密码): digest-only storage, single-use,
// DB-clock TTL. Every time comparison happens in SQL (NOW(3)) — the Go and
// MySQL clocks never need to agree, and a replayed link finds nothing.

// CreatePasswordResetToken stores the SHA-256 digest of a fresh reset token.
// Two housekeeping rules ride along:
//   - a new request invalidates the user's previous unused tokens (the
//     latest email wins, and the active-token surface stays at one);
//   - rows that expired or were already consumed more than a day ago are
//     purged, so the table stays bounded without a cron job.
func (m *Mysql) CreatePasswordResetToken(ctx context.Context, uid int64, tokenHash string, ttl time.Duration) error {
	secs := int64(ttl / time.Second)
	if secs <= 0 {
		secs = 1800
	}
	// Best-effort housekeeping: a purge failure must not block the insert.
	_, _ = m.db.ExecContext(ctx,
		`DELETE FROM password_reset_tokens
		  WHERE used_at IS NOT NULL OR expires_at < NOW(3) - INTERVAL 1 DAY`)
	if _, err := m.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW(3)
		  WHERE uid = ? AND used_at IS NULL`, uid); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (uid, token_hash, expires_at)
		 VALUES (?, ?, DATE_ADD(NOW(3), INTERVAL ? SECOND))`, uid, tokenHash, secs)
	return err
}

// ConsumePasswordResetToken atomically claims a token: the UPDATE matches
// only an unused, unexpired row, so a double-click or an attacker replaying
// a leaked link finds nothing. Every failure mode — unknown digest, expired,
// already used — collapses into ErrNotFound: the endpoint must not leak
// which one it was.
func (m *Mysql) ConsumePasswordResetToken(ctx context.Context, tokenHash string) (int64, error) {
	res, err := m.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW(3)
		  WHERE token_hash = ? AND used_at IS NULL AND expires_at > NOW(3)`,
		tokenHash)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return 0, ErrNotFound
	}
	// The row exists and is now claimed; read the owner back (the digest is
	// unique-indexed, and the row is never deleted by this path).
	var uid int64
	if err := m.db.QueryRowContext(ctx,
		`SELECT uid FROM password_reset_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&uid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return uid, nil
}

// UserByEmail resolves an email address to a uid (case-insensitive). The
// COLLATE sits on the column expression — `? COLLATE …` is a syntax error
// on MySQL 5.7 (same trap UserByIdentifier documents).
func (m *Mysql) UserByEmail(ctx context.Context, email string) (int64, error) {
	var uid int64
	err := m.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email COLLATE utf8mb4_general_ci = ? LIMIT 1`,
		email).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return uid, err
}
