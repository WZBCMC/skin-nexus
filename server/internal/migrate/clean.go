package migrate

import "fmt"

// UuidRow mirrors the legacy yggdrasil-connect `uuid` table
// (id, name, uuid, pid optional).
type UuidRow struct {
	ID   int64
	Name string
	UUID string
	PID  int64 // 0 = unlinked (legacy mojang-verification rows)
}

// CleanUuidTable runs the five steps of legacy `yggc:fix-uuid-table`
// (analysis/blessing-skin-plugins.md §2) WITHOUT touching the source:
// executed against a snapshot; the real run writes only the cleaned result
// to the new schema (uuid_map unique keys enforce steps 1-2 structurally).
//
// Step semantics preserved:
//  1. drop duplicate names  (keep lowest id)
//  2. drop duplicate uuids  (keep lowest id)
//  3. link pid by exact name match against known players (backfill)
//  4. drop orphans (rows whose name matches no player and pid is 0)
//  5. uniqueness is guaranteed by the target schema (uq_uuid_map_uuid, PK name)
func CleanUuidTable(rows []UuidRow, players map[string]int64) (cleaned []UuidRow, report CleanReport) {
	report.Input = len(rows)

	// Step 1: dedupe by name, keep lowest id.
	seenName := map[string]bool{}
	var s1 []UuidRow
	for _, r := range rows { // assume id-ascending input; sort responsibility on caller
		if seenName[r.Name] {
			report.DupNameDropped++
			continue
		}
		seenName[r.Name] = true
		s1 = append(s1, r)
	}

	// Step 2: dedupe by uuid, keep lowest id.
	seenUUID := map[string]bool{}
	var s2 []UuidRow
	for _, r := range s1 {
		if seenUUID[r.UUID] {
			report.DupUUIDDropped++
			continue
		}
		seenUUID[r.UUID] = true
		s2 = append(s2, r)
	}

	// Step 3: backfill pid from players by name.
	for _, r := range s2 {
		if r.PID == 0 {
			if pid, ok := players[r.Name]; ok {
				r.PID = pid
				report.PIDBackfilled++
			}
		}
		cleaned = append(cleaned, r)
	}

	// Step 4: drop orphans (no pid and no player match).
	var s4 []UuidRow
	for _, r := range cleaned {
		if r.PID == 0 {
			report.OrphansDropped++
			continue
		}
		s4 = append(s4, r)
	}
	report.Output = len(s4)
	return s4, report
}

// CleanReport is the dry-run/apply evidence for the uuid table.
type CleanReport struct {
	Input          int
	DupNameDropped int
	DupUUIDDropped int
	PIDBackfilled  int
	OrphansDropped int
	Output         int
}

func (r CleanReport) String() string {
	return fmt.Sprintf("uuid clean: in=%d dupName=%d dupUUID=%d pidBackfill=%d orphans=%d out=%d",
		r.Input, r.DupNameDropped, r.DupUUIDDropped, r.PIDBackfilled, r.OrphansDropped, r.Output)
}
