package store

import "context"

// System facts for the admin status page (B1, 2026-10-24): read-only,
// MySQL 5.7-safe (no window functions, no CTEs).

// MySQLVersion reports the server version string (e.g. "5.7.44-log").
func (m *Mysql) MySQLVersion(ctx context.Context) (string, error) {
	var v string
	err := m.db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&v)
	return v, err
}

// MigrationVersion reports the highest applied migration file marker
// (schema_migrations.file is lexicographically ordered: 0001_… < 0009_…).
func (m *Mysql) MigrationVersion(ctx context.Context) (string, error) {
	var v string
	err := m.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(file), '') FROM schema_migrations`).Scan(&v)
	return v, err
}
