package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePlugin creates <root>/<name>/plugin.json (+ optional entrypoint body).
func writePlugin(t *testing.T, root, name, manifest, script string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const goodManifest = `{
  "name": "demo",
  "version": "1.0.0",
  "hookApi": 1,
  "entrypoint": "index.js",
  "permissions": ["textures:read"],
  "title": "Demo",
  "author": "YMZS",
  "homepage": "https://example.com",
  "description": "demo plugin"
}`

func TestLoadManifestValid(t *testing.T) {
	root := t.TempDir()
	dir := writePlugin(t, root, "demo", goodManifest, "skin.log('hi');")
	info, err := loadManifest(dir, "demo")
	if err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
	if info.Name != "demo" || info.Version != "1.0.0" || info.HookAPI != 1 {
		t.Fatalf("parsed = %+v", info.Manifest)
	}
	if info.Title != "Demo" || info.Author != "YMZS" || info.Homepage != "https://example.com" {
		t.Fatalf("metadata = %+v", info)
	}
	if len(info.Permissions) != 1 || info.Permissions[0] != PermReadTextures {
		t.Fatalf("permissions = %v", info.Permissions)
	}
	if info.ScriptPath != filepath.Join(dir, "index.js") || info.ScriptSize == 0 {
		t.Fatalf("script = %s/%d", info.ScriptPath, info.ScriptSize)
	}
}

func TestLoadManifestRejects(t *testing.T) {
	cases := []struct {
		name     string
		dirName  string
		manifest string
		script   string // "" = omit entrypoint file
		setup    func(t *testing.T, dir string)
	}{
		{"name mismatch", "other", goodManifest, "x", nil},
		{"bad name", "demo", strings.Replace(goodManifest, `"name": "demo"`, `"name": "Demo!"`, 1), "x", nil},
		{"empty version", "demo", strings.Replace(goodManifest, `"1.0.0"`, `""`, 1), "x", nil},
		{"hookApi mismatch", "demo", strings.Replace(goodManifest, `"hookApi": 1`, `"hookApi": 3`, 1), "x", nil},
		{"entrypoint traversal", "demo", strings.Replace(goodManifest, `"index.js"`, `"../evil.js"`, 1), "x", nil},
		{"entrypoint not js", "demo", strings.Replace(goodManifest, `"index.js"`, `"index.txt"`, 1), "x", nil},
		{"unknown permission", "demo", strings.Replace(goodManifest, `"textures:read"`, `"kernel:panic"`, 1), "x", nil},
		{"bad homepage", "demo", strings.Replace(goodManifest, `"https://example.com"`, `"ftp://example.com"`, 1), "x", nil},
		{"missing entrypoint file", "demo", goodManifest, "", nil},
		{"title too long", "demo", strings.Replace(goodManifest, `"Demo"`, `"`+strings.Repeat("t", 200)+`"`, 1), "x", nil},
		{"symlink entrypoint", "demo", goodManifest, "x", func(t *testing.T, dir string) {
			target := filepath.Join(dir, "real.js")
			if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "index.js")
			_ = os.Remove(link)
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unsupported: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := writePlugin(t, root, tc.dirName, tc.manifest, tc.script)
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			if _, err := loadManifest(dir, tc.dirName); err == nil {
				t.Fatalf("expected rejection for %s", tc.name)
			}
		})
	}
}

func TestLoadManifestDedupsPermissions(t *testing.T) {
	root := t.TempDir()
	m := strings.Replace(goodManifest, `["textures:read"]`, `["textures:read","textures:read"]`, 1)
	dir := writePlugin(t, root, "demo", m, "x")
	info, err := loadManifest(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Permissions) != 1 {
		t.Fatalf("permissions = %v", info.Permissions)
	}
}

func TestLoadManifestOversized(t *testing.T) {
	root := t.TempDir()
	dir := writePlugin(t, root, "demo", goodManifest, "")
	big := filepath.Join(dir, "index.js")
	if err := os.WriteFile(big, make([]byte, maxScriptBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(dir, "demo"); err == nil {
		t.Fatal("expected oversize rejection")
	}

	huge := filepath.Join(root, "huge")
	if err := os.MkdirAll(huge, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(huge, "plugin.json"), make([]byte, maxManifestBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(huge, "huge"); err == nil {
		t.Fatal("expected manifest size rejection")
	}
}
