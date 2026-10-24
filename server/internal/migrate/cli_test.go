package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIPreflight(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	_ = os.WriteFile(envPath, []byte(
		"APP_URL=https://skin.example.com\nPASSWORD_METHODS=SALTED2MD5,BCRYPT\nSALT=ab12cd,unused\n"), 0o644)
	dump := filepath.Join(dir, "hashes.txt")
	// 2 weak (hex32) + 1 bcrypt
	_ = os.WriteFile(dump, []byte(
		"5d41402abc4b2a76b9719d911017c592\n2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n"+
			"$2y$04$abcdefghijklmnopqrstuv\n"), 0o644)

	var buf bytes.Buffer
	err := RunCLI([]string{"preflight", "--env", envPath, "--dump", dump}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"SALT slots aligned OK",
		"salt=ab**cd", // masked (6-char salt -> 2+2+2), not raw
		"bcrypt=1", "hex32=1", "hex64=1",
		"weak-share=66.7%",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "ab12cd") {
		t.Error("raw salt leaked in output")
	}
	// missing --env must error
	if err := RunCLI([]string{"preflight"}, &buf); err == nil {
		t.Error("missing --env must fail")
	}
}

func TestRunCLIUuidDryRun(t *testing.T) {
	dir := t.TempDir()
	rows := filepath.Join(dir, "uuid.tsv")
	// id-ascending: dup name, dup uuid, backfillable, orphan
	_ = os.WriteFile(rows, []byte(
		"1\tSteve\taaa\t10\n2\tSteve\tbbb\t0\n3\tAlex\taaa\t11\n4\tNoor\tccc\t0\n5\tGhost\tddd\t0\n"), 0o644)
	players := filepath.Join(dir, "players.tsv")
	_ = os.WriteFile(players, []byte("10\tSteve\n11\tAlex\n12\tNoor\n"), 0o644)

	var buf bytes.Buffer
	if err := RunCLI([]string{"uuid", "--rows", rows, "--players", players}, &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"dupName=1", "dupUUID=1", "pidBackfill=1", "orphans=1", "out=2",
		"KEEP id=4 name=Noor"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

func TestRunCLIUnknownCommand(t *testing.T) {
	if err := RunCLI([]string{"frobnicate"}, io.Discard); err == nil {
		t.Error("unknown command must fail")
	}
}
