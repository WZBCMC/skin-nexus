package plugin

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func makeArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
}

func TestExtractZipValidAndTopDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.zip")
	makeArchive(t, path, map[string]string{"plugin.json": "{}", "index.js": "1"})
	dest := t.TempDir()
	if err := extractZip(path, dest); err != nil {
		t.Fatal(err)
	}
	root, err := findPluginRoot(dest)
	if err != nil || root != dest {
		t.Fatalf("root=%s err=%v", root, err)
	}

	top := t.TempDir()
	makeArchive(t, path, map[string]string{"demo/plugin.json": "{}", "demo/index.js": "1"})
	if err := extractZip(path, top); err != nil {
		t.Fatal(err)
	}
	root, err = findPluginRoot(top)
	if err != nil || filepath.Base(root) != "demo" {
		t.Fatalf("top root=%s err=%v", root, err)
	}
}

func TestExtractZipRejectsUnsafe(t *testing.T) {
	cases := map[string]string{
		"traversal":    "../evil.js",
		"absolute":     "/etc/passwd",
		"backslash":    `a\b.js`,
		"reserved":     "CON.txt",
		"double dot":   "a/../../b",
		"root no name": "",
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.zip")
			makeArchive(t, path, map[string]string{entry: "x"})
			if err := extractZip(path, t.TempDir()); err == nil {
				t.Fatalf("accepted %q", entry)
			}
		})
	}
}

func TestExtractZipRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sym.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(h)
	_, _ = w.Write([]byte("target"))
	_ = zw.Close()
	_ = f.Close()
	if err := extractZip(path, t.TempDir()); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestExtractZipRejectsBombRatio(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bomb.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("big.js")
	chunk := make([]byte, 1<<20)
	for i := 0; i < 2; i++ {
		_, _ = w.Write(chunk)
	}
	_ = zw.Close()
	_ = f.Close()
	if err := extractZip(path, t.TempDir()); err == nil {
		t.Fatal("high-compression bomb accepted")
	}
}
