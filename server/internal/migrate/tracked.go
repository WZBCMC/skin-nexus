// tracked.go: apply migrations/ with per-file markers so EXISTING databases
// receive new migration files too (option A,拍板 2026-10-23 — the empty-DB-only
// path left 0006 (invitation_codes) unapplied on the live test instance).
//
// Semantics:
//   - schema_migrations(file PK, applied_at) records completed files;
//   - unmarked files run on every SKIN_AUTO_MIGRATE boot, in filename order;
//   - a statement failing with a duplicate-OBJECT collision (ER1050 table /
//     ER1060 column / ER1061 index already exists) is tolerated — pre-tracking
//     databases self-bootstrap — and the file still gets marked;
//   - any other error fails loud (log.Fatal at the caller);
//   - migration files stay DDL-only: no data INSERTs (a re-run on a
//     pre-tracking DB would otherwise double-apply).
package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplySQLDirTracked applies pending *.sql files in dir (filename order),
// skipping files already recorded in schema_migrations.
func ApplySQLDirTracked(db *sql.DB, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		file       VARCHAR(191) NOT NULL,
		applied_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		PRIMARY KEY (file)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("migrate: schema_migrations bootstrap: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("migrate: no *.sql found in %s", dir)
	}
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		var one int
		switch err := db.QueryRow(`SELECT 1 FROM schema_migrations WHERE file = ?`, base).Scan(&one); {
		case err == nil:
			continue // already applied
		case err != sql.ErrNoRows:
			return fmt.Errorf("migrate: marker read %s: %w", base, err)
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, stmt := range SplitSQL(string(raw)) {
			if _, err := db.Exec(stmt); err != nil {
				if isDuplicateCollision(err) {
					continue // already took effect (pre-tracking database)
				}
				return fmt.Errorf("migrate: %s: %w\nstmt: %s", base, err, stmt)
			}
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (file) VALUES (?)`, base); err != nil {
			return fmt.Errorf("migrate: mark %s: %w", base, err)
		}
	}
	return nil
}

// isDuplicateCollision matches the MySQL duplicate-object errors that mean
// "this statement already took effect". go-sql-driver formats them as
// "Error 1050 (42S01): Table 'users' already exists" (SQLSTATE included —
// the pre-fix matcher required "Error 1050:" and crash-looped the deploy).
// ER1062 (duplicate data entry) is deliberately NOT tolerated — data
// collisions must fail loud.
func isDuplicateCollision(err error) bool {
	msg := err.Error()
	for _, code := range []string{"Error 1050", "Error 1060", "Error 1061"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}
