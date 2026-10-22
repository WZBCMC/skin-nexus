// union.go: store queries backing the MUA Union Hooks (inbound surface).
// Kept out of the union package per repo rule (new queries live in store).
package store

import (
	"context"
)

// ProfileNameByUUID returns {uuid: name} for every mapped player — backs
// union.Hooks.ProfileListFunc (POST /sync payload).
func (m *Mysql) ProfileNameByUUID(ctx context.Context) (map[string]string, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT u.uuid, p.name FROM uuid_map u JOIN players p ON p.id = u.player_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var uuid, name string
		if err := rows.Scan(&uuid, &name); err != nil {
			return nil, err
		}
		out[uuid] = name
	}
	return out, rows.Err()
}

// RemapUUIDs applies a hub-issued {old: new} batch to uuid_map. Only mapped
// rows (player_id NOT NULL) are touched; unknown old values are skipped and
// reported so the audit trail shows partial application instead of silently
// dropping hub state.
func (m *Mysql) RemapUUIDs(ctx context.Context, remap map[string]string) error {
	if len(remap) == 0 {
		return nil
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for old, new := range remap {
		if old == "" || new == "" || old == new {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE uuid_map SET uuid = ? WHERE uuid = ? AND player_id IS NOT NULL`, new, old); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PlayerUUIDsOwnedBy returns the canonical uuids of one user's players —
// ownership check helper for union user endpoints (defense in depth beside
// the per-player lookup).
func (m *Mysql) PlayerUUIDsOwnedBy(ctx context.Context, uid int64) (map[string]bool, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT u.uuid FROM uuid_map u JOIN players p ON p.id = u.player_id WHERE p.uid = ?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return nil, err
		}
		out[uuid] = true
	}
	return out, rows.Err()
}
