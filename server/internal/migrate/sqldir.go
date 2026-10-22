// sqldir.go: apply a migrations directory to a live *sql.DB. Used by the
// empty-database auto-init at boot (docker first-run) and by the MySQL
// integration suite. Statements are split on ';' — the shipped DDL contains
// no routines/DELIMITER blocks. Comments ('-- …') are skipped.
package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplySQLDir executes every *.sql in dir, in filename order.
func ApplySQLDir(db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("migrate: no *.sql found in %s", dir)
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, stmt := range SplitSQL(string(raw)) {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate: %s: %w\nstmt: %s", filepath.Base(f), err, stmt)
			}
		}
	}
	return nil
}

// SplitSQL splits a DDL script into executable statements.
func SplitSQL(script string) []string {
	var out []string
	var sb strings.Builder
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(sb.String())
			if stmt != "" && stmt != ";" {
				out = append(out, strings.TrimSuffix(stmt, ";"))
			}
			sb.Reset()
		}
	}
	if s := strings.TrimSpace(sb.String()); s != "" {
		out = append(out, s)
	}
	return out
}
