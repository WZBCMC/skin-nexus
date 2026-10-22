package store

import (
	"context"
	"crypto/md5" //nolint:gosec // legacy-compatible UUID derivation
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)
// ---- registration & web sessions ----

// CreateUser registers a user. passwordHash is already modern PHC.
func (m *Mysql) CreateUser(ctx context.Context, email, nickname, passwordHash string, playerLimit int) (int64, error) {
	res, err := m.db.ExecContext(ctx,
		`INSERT INTO users (email, nickname, password, password_algo, player_limit, email_verified, can_join_server)
		 VALUES (?,?,?,'argon2id',?,0,1)`,
		nullableStr(email), nickname, passwordHash, playerLimit)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateSession stores an opaque web session token.
func (m *Mysql) CreateSession(ctx context.Context, token string, uid int64, expires time.Time) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO web_sessions (token, user_id, expires_at) VALUES (?,?,?)`, token, uid, expires)
	return err
}

// SessionUser resolves a session token to a user (nil-safe: ErrNotFound if invalid).
func (m *Mysql) SessionUser(ctx context.Context, token string) (int64, error) {
	var uid int64
	err := m.db.QueryRowContext(ctx,
		`SELECT user_id FROM web_sessions WHERE token = ? AND expires_at > NOW(3)`, token).
		Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return uid, err
}

func (m *Mysql) DeleteSession(ctx context.Context, token string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE token = ?`, token)
	return err
}

// RehashPassword updates to modern digest after successful legacy login.
func (m *Mysql) RehashPassword(ctx context.Context, uid int64, newHash string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE users SET password = ?, password_algo = 'argon2id' WHERE id = ?`, newHash, uid)
	return err
}

// UserPasswordAlgo exposes the password algorithm marker (for lazy rehash
// decisions in webapi after a successful verify).
func (m *Mysql) UserPasswordAlgo(ctx context.Context, uid int64) (string, error) {
	var algo string
	err := m.db.QueryRowContext(ctx, `SELECT password_algo FROM users WHERE id = ?`, uid).Scan(&algo)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return algo, err
}

// WeakHashCount powers the 「弱哈希余量」 dashboard.
func (m *Mysql) WeakHashCount(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE password_algo LIKE 'legacy%'`).Scan(&n)
	return n, err
}

// ---- textures ----

// AddTexture inserts a texture record and returns its id.
func (m *Mysql) AddTexture(ctx context.Context, uid int64, typ, hash, model, desc string) (int64, error) {
	res, err := m.db.ExecContext(ctx,
		`INSERT INTO textures (uid, type, hash, model, description) VALUES (?,?,?,?,?)`,
		uid, typ, hash, nullableStr(model), nullableStr(desc))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TextureByID returns texture row (for serving raw bytes later).
func (m *Mysql) TextureByHash(ctx context.Context, hash string) (id, uid int64, typ string, err error) {
	err = m.db.QueryRowContext(ctx,
		`SELECT id, uid, type FROM textures WHERE hash = ? LIMIT 1`, hash).
		Scan(&id, &uid, &typ)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

// ListTextures returns a user's textures.
func (m *Mysql) ListTextures(ctx context.Context, uid int64) ([]TextureRow, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, type, hash, COALESCE(model,''), COALESCE(description,''), upload_at
		 FROM textures WHERE uid = ? ORDER BY id DESC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TextureRow
	for rows.Next() {
		var t TextureRow
		if err := rows.Scan(&t.ID, &t.Type, &t.Hash, &t.Model, &t.Description, &t.UploadAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type TextureRow struct {
	ID          int64
	Type        string
	Hash        string
	Model       string
	Description string
	UploadAt    time.Time
}

// TextureOwner returns the owning user id of a texture.
func (m *Mysql) TextureOwner(ctx context.Context, tid int64) (int64, error) {
	var uid int64
	err := m.db.QueryRowContext(ctx, `SELECT uid FROM textures WHERE id = ?`, tid).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return uid, err
}

// PlayerTextureIDs returns current skin/cape texture ids (0 = unset).
func (m *Mysql) PlayerTextureIDs(ctx context.Context, pid int64) (skin, cape int64, err error) {
	var s, c sql.NullInt64
	err = m.db.QueryRowContext(ctx,
		`SELECT tid_skin, tid_cape FROM players WHERE id = ?`, pid).Scan(&s, &c)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return s.Int64, c.Int64, err
}

// PlayerTextureHashes returns the raw texture hashes for legacy-compat
// endpoints (CustomSkinAPI / USM / legacy-api need hashes, not URLs).
func (m *Mysql) PlayerTextureHashes(ctx context.Context, pid int64) (skinHash, skinModel, capeHash string, err error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT COALESCE(ts.hash,''), COALESCE(ts.model,'default'), COALESCE(tc.hash,'')
		 FROM players p
		 LEFT JOIN textures ts ON ts.id = p.tid_skin
		 LEFT JOIN textures tc ON tc.id = p.tid_cape
		 WHERE p.id = ?`, pid)
	err = row.Scan(&skinHash, &skinModel, &capeHash)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

// ---- admin queries ----

// Counters for the admin overview.
func (m *Mysql) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (m *Mysql) CountPlayers(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&n)
	return n, err
}

func (m *Mysql) CountTextures(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM textures`).Scan(&n)
	return n, err
}

// AdminUsers returns a page of users with weak-hash marker.
func (m *Mysql) AdminUsers(ctx context.Context, page, size int) ([]map[string]interface{}, int, error) {
	var total int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, COALESCE(email,''), nickname, password_algo, player_limit, created_at
		 FROM users ORDER BY id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, limit int64
		var email, nick, algo string
		var created time.Time
		if err := rows.Scan(&id, &email, &nick, &algo, &limit, &created); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "email": email, "nickname": nick,
			"passwordAlgo": algo, "playerLimit": limit,
			"createdAt": created.Format(time.RFC3339),
		})
	}
	return out, total, rows.Err()
}

// AdminTextures returns a page of texture records.
func (m *Mysql) AdminTextures(ctx context.Context, page, size int) ([]map[string]interface{}, int, error) {
	var total int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM textures`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := m.db.QueryContext(ctx,
		`SELECT t.id, t.uid, t.type, t.hash, COALESCE(t.model,''), t.upload_at
		 FROM textures t ORDER BY t.id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, uid int64
		var typ, hash, model string
		var up time.Time
		if err := rows.Scan(&id, &uid, &typ, &hash, &model, &up); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "uid": uid, "type": typ, "hash": hash, "model": model,
			"uploadAt": up.Format(time.RFC3339),
		})
	}
	return out, total, rows.Err()
}

// RecentAudit returns latest audit entries.
func (m *Mysql) RecentAudit(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, COALESCE(actor_uid,0), action, COALESCE(detail,'{}'), created_at
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, actor int64
		var action, detail string
		var created time.Time
		if err := rows.Scan(&id, &actor, &action, &detail, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "actorUid": actor, "action": action, "detail": detail,
			"createdAt": created.Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}

// ---- players ----

// CreatePlayer adds a character. Generates a UUID per the legacy v3 rule
// (hex2bin(md5("OfflinePlayer:"+name)), byte6 |= 0x30, byte8 |= 0x80) and
// records it in uuid_map so renames never recompute it.
func (m *Mysql) CreatePlayer(ctx context.Context, uid int64, name string) (int64, string, error) {
	uuid := OfflineUUID(name)
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO players (uid, name) VALUES (?,?)`, uid, name)
	if err != nil {
		return 0, "", err
	}
	pid, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO uuid_map (name, uuid, player_id) VALUES (?,?,?)`, name, uuid, pid); err != nil {
		return 0, "", err
	}
	return pid, uuid, tx.Commit()
}

// SetPlayerTextures points a player at texture ids (0 = clear).
func (m *Mysql) SetPlayerTextures(ctx context.Context, pid, skinTid, capeTid int64) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE players SET
		 tid_skin = IF(? = 0, NULL, ?),
		 tid_cape = IF(? = 0, NULL, ?)
		 WHERE id = ?`, skinTid, skinTid, capeTid, capeTid, pid)
	return err
}

// OfflineUUID reproduces the legacy v3 UUID rule byte-exactly
// (analysis/blessing-skin-plugins.md §2 — hex2bin(md5(...)) + variant bits).
func OfflineUUID(name string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + name)) //nolint:gosec
	b := sum[:]
	b[6] = (b[6] & 0x0f) | 0x30 // version 3
	b[8] = (b[8] & 0x3f) | 0x80 // RFC4122 variant
	return hex.EncodeToString(b)
}

func nullableStr(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
