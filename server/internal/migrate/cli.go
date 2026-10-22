package migrate

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// RunCLI implements the `skin-nexus migrate` subcommands:
//
//	migrate preflight --env legacy.env [--textures DIR] [--dump hashes.txt]
//	migrate uuid --rows uuid.tsv --players players.tsv [--out report.txt]
//
// preflight = 部署执行序第 1 步（phase1-dev-plan §5）: parse legacy .env,
// print positional method/salt table (salts masked), hash distribution when a
// dump is provided, and texture-tree audit when a dir is given.
// uuid = dry-run of the five-step cleaning over exported TSV snapshots;
// the apply path stays a SQL script at deployment (report only by design).
func RunCLI(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate preflight|uuid ...")
	}
	switch args[0] {
	case "preflight":
		return runPreflight(args[1:], out)
	case "uuid":
		return runUuidClean(args[1:], out)
	default:
		return fmt.Errorf("unknown migrate command %q (want preflight|uuid)", args[0])
	}
}

func runPreflight(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	envPath := fs.String("env", "", "path to legacy .env (required)")
	textures := fs.String("textures", "", "legacy TEXTURES_DIR to audit")
	dump := fs.String("dump", "", "optional hash dump file (one stored hash per line)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *envPath == "" {
		return fmt.Errorf("preflight: --env is required")
	}
	env, err := ParseLegacyEnv(*envPath)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "== legacy env ==")
	fmt.Fprintf(out, "APP_URL=%s\nPASSWORD_METHODS=%d (SALT slots aligned OK)\n",
		env.AppURL, len(env.PWDMethods))
	for i, ms := range env.MethodSalts() {
		fmt.Fprintf(out, "  [%d] %-16s salt=%s\n", i, ms.Method, mask(ms.Salt))
	}

	if *dump != "" {
		stored, err := readLines(*dump)
		if err != nil {
			return err
		}
		d := ClassifyHashes(stored)
		fmt.Fprintln(out, "== hash distribution ==")
		fmt.Fprintf(out, "bcrypt=%d argon2=%d authmeSHA=%d hex32=%d hex64=%d hex128=%d other=%d\n",
			d.Bcrypt, d.Argon2, d.AuthmeSHA, d.Hex32, d.Hex64, d.Hex128, d.Other)
		fmt.Fprintf(out, "weak-share=%.1f%% -> scenario %s\n", d.WeakShare*100,
			map[bool]string{true: "甲/乙 pending confirmation", false: "甲 (bcrypt/argon2i dominant)"}[d.WeakShare > 0])
		if d.WeakShare == 0 {
			fmt.Fprintln(out, "strategy: lazy rehash only (AI2 方案)")
		} else {
			fmt.Fprintln(out, "strategy: lazy rehash + 弱哈希余量看板 (AI2 方案)")
		}
	}

	if *textures != "" {
		s, err := ScanTextureTree(*textures)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "== texture tree ==")
		fmt.Fprintln(out, s.String())
		for _, n := range s.HashMismatch {
			fmt.Fprintf(out, "  HASH-MISMATCH %s\n", n)
		}
		for _, n := range s.BadIHDR {
			fmt.Fprintf(out, "  BAD-IHDR %s\n", n)
		}
		for _, n := range s.NotHexName {
			fmt.Fprintf(out, "  BAD-NAME %s\n", n)
		}
	}
	return nil
}

func runUuidClean(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("uuid", flag.ContinueOnError)
	rowsPath := fs.String("rows", "", "legacy uuid table TSV: id\tname\tuuid\tpid (id-ascending)")
	playersPath := fs.String("players", "", "legacy players TSV: id\tname")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *rowsPath == "" || *playersPath == "" {
		return fmt.Errorf("uuid: --rows and --players are required")
	}
	rows, err := parseUuidRows(*rowsPath)
	if err != nil {
		return err
	}
	players, err := parsePlayers(*playersPath)
	if err != nil {
		return err
	}
	cleaned, rep := CleanUuidTable(rows, players)
	fmt.Fprintln(out, rep.String())
	for _, r := range cleaned {
		fmt.Fprintf(out, "  KEEP id=%d name=%s uuid=%s pid=%d\n", r.ID, r.Name, r.UUID, r.PID)
	}
	return nil
}

// ---- input helpers ----

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			out = append(out, l)
		}
	}
	return out, sc.Err()
}

func parseUuidRows(path string) ([]UuidRow, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var out []UuidRow
	for _, l := range lines {
		p := strings.Split(l, "\t")
		if len(p) < 3 {
			return nil, fmt.Errorf("uuid row malformed: %q", l)
		}
		r := UuidRow{Name: p[1], UUID: p[2]}
		fmt.Sscanf(p[0], "%d", &r.ID)
		if len(p) > 3 {
			fmt.Sscanf(p[3], "%d", &r.PID)
		}
		out = append(out, r)
	}
	return out, nil
}

func parsePlayers(path string) (map[string]int64, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, l := range lines {
		p := strings.Split(l, "\t")
		if len(p) != 2 {
			return nil, fmt.Errorf("player row malformed: %q", l)
		}
		var id int64
		if _, err := fmt.Sscanf(p[0], "%d", &id); err != nil {
			return nil, err
		}
		out[p[1]] = id
	}
	return out, nil
}

func mask(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
