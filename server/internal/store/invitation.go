package store

import (
	"context"
	"crypto/md5" //nolint:gosec // legacy-compatible code shape: md5 hex, like the blessing plugin
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// InvitationCode mirrors the live blessing invitation-codes table 1:1
// (invitation_codes(id,code,generated_at,used_by,used_at,description); see
// analysis/invitation-codes-live-delta.md). used_by=0 means unused.
type InvitationCode struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	GeneratedAt time.Time  `json:"generated_at"`
	UsedBy      int64      `json:"used_by"`
	UsedAt      *time.Time `json:"used_at"`
	Description string     `json:"description"`
}

// NewInvitationCodeValue returns a fresh code in the legacy shape: md5 hex
// over random bytes, 32 lowercase hex chars (md5(Str::random()) upstream).
func NewInvitationCodeValue() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	sum := md5.Sum(buf) //nolint:gosec
	return hex.EncodeToString(sum[:])
}

// CreateInvitationCodes generates n codes. description is optional and
// normally empty here — the live admin flow generates single codes carrying
// a description (e.g. the legacy "姓名+学号" convention).
func (m *Mysql) CreateInvitationCodes(ctx context.Context, n int, description string) ([]InvitationCode, error) {
	if n < 1 {
		n = 1
	}
	if n > 1000 {
		n = 1000
	}
	now := time.Now()
	out := make([]InvitationCode, 0, n)
	for i := 0; i < n; i++ {
		code := NewInvitationCodeValue()
		res, err := m.db.ExecContext(ctx,
			`INSERT INTO invitation_codes (code, generated_at, used_by, description) VALUES (?,?,0,?)`,
			code, now, nullableStr(description))
		if err != nil {
			return out, err
		}
		id, _ := res.LastInsertId()
		out = append(out, InvitationCode{ID: id, Code: code, GeneratedAt: now, Description: description})
	}
	return out, nil
}

// FindInvitationCode resolves one code (any state).
func (m *Mysql) FindInvitationCode(ctx context.Context, code string) (InvitationCode, bool, error) {
	var ic InvitationCode
	var usedAt sql.NullTime
	var desc sql.NullString
	err := m.db.QueryRowContext(ctx,
		`SELECT id, code, generated_at, used_by, used_at, description FROM invitation_codes WHERE code = ?`, code).
		Scan(&ic.ID, &ic.Code, &ic.GeneratedAt, &ic.UsedBy, &usedAt, &desc)
	if errors.Is(err, sql.ErrNoRows) {
		return InvitationCode{}, false, ErrNotFound
	}
	if err != nil {
		return InvitationCode{}, false, err
	}
	if usedAt.Valid {
		t := usedAt.Time
		ic.UsedAt = &t
	}
	ic.Description = desc.String
	return ic, true, nil
}

// ListInvitationCodes returns the legacy free/used split (admin list page).
func (m *Mysql) ListInvitationCodes(ctx context.Context) (free, used []InvitationCode, err error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, code, generated_at, used_by, used_at, description FROM invitation_codes ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ic InvitationCode
		var usedAt sql.NullTime
		var desc sql.NullString
		if err := rows.Scan(&ic.ID, &ic.Code, &ic.GeneratedAt, &ic.UsedBy, &usedAt, &desc); err != nil {
			return nil, nil, err
		}
		if usedAt.Valid {
			t := usedAt.Time
			ic.UsedAt = &t
		}
		ic.Description = desc.String
		if ic.UsedBy == 0 {
			free = append(free, ic)
		} else {
			used = append(used, ic)
		}
	}
	return free, used, rows.Err()
}

// SetInvitationCodeDescription updates the description (live plugin PUT edit).
func (m *Mysql) SetInvitationCodeDescription(ctx context.Context, id int64, description string) error {
	res, err := m.db.ExecContext(ctx,
		`UPDATE invitation_codes SET description = ? WHERE id = ?`, nullableStr(description), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateUserWithInvitation registers a user and burns the invitation code in
// one transaction. The atomic "used_by = 0" guard closes the double-use race
// the legacy plugin left open (session stash + after-the-fact update).
// ErrNotFound = the code is missing or already used (nothing is created).
func (m *Mysql) CreateUserWithInvitation(ctx context.Context, email, nickname, passwordHash string, playerLimit int, code string) (int64, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (email, nickname, password, password_algo, player_limit, email_verified, can_join_server)
		 VALUES (?,?,?,'argon2id',?,0,1)`,
		nullableStr(email), nickname, passwordHash, playerLimit)
	if err != nil {
		return 0, err
	}
	uid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	up, err := tx.ExecContext(ctx,
		`UPDATE invitation_codes SET used_by = ?, used_at = ? WHERE code = ? AND used_by = 0`,
		uid, time.Now(), code)
	if err != nil {
		return 0, err
	}
	if n, _ := up.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uid, nil
}
