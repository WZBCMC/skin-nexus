package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/signedpkg"
)

func runPack(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "plugin directory")
	out := fs.String("out", "", "output zip or directory")
	key := fs.String("key", "", "Ed25519 PKCS#8 PEM private key")
	keyID := fs.String("key-id", "", "key id")
	metaPath := fs.String("meta", "", "market.json path (default <dir>/market.json if present)")
	force := fs.Bool("force", false, "overwrite existing output")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" || *out == "" || *key == "" || *keyID == "" {
		fmt.Fprintln(stderr, "pack: --dir --out --key --key-id are required")
		return 2
	}
	info, err := plugin.InspectDir(*dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	metaFile := *metaPath
	if metaFile == "" {
		metaFile = filepath.Join(*dir, "market.json")
	}
	meta, err := readMarketMeta(metaFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := validateMarketMeta(info, meta); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	priv, err := loadPrivateKey(*key)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	entries, err := collectEntries(*dir, metaFile, *metaPath != "")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !hasEntry(entries, "plugin.json") {
		fmt.Fprintln(stderr, "pack: plugin.json missing")
		return 1
	}
	rel, _ := filepath.Rel(*dir, info.ScriptPath)
	if !hasEntry(entries, filepath.ToSlash(rel)) {
		fmt.Fprintf(stderr, "pack: entrypoint %s missing\n", rel)
		return 1
	}
	if info.UI != nil && info.UI.Assets != "" {
		if _, err := os.Stat(filepath.Join(*dir, filepath.FromSlash(info.UI.Assets))); err != nil {
			fmt.Fprintf(stderr, "pack: ui.assets %s missing\n", info.UI.Assets)
			return 1
		}
	}

	outPath := *out
	if fi, err := os.Stat(outPath); err == nil && fi.IsDir() {
		outPath = filepath.Join(outPath, info.Name+"-"+info.Version+".zip")
	}
	if !strings.HasSuffix(strings.ToLower(outPath), ".zip") {
		outPath += ".zip"
	}
	if !*force {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Fprintf(stderr, "pack: %s exists (use --force)\n", outPath)
			return 1
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	digest, sha, err := signedpkg.WriteNamedBundle(outPath, info.Name, info.Version, entries, priv, *keyID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fi, _ := os.Stat(outPath)
	result := map[string]any{
		"name": info.Name, "version": info.Version, "keyId": *keyID,
		"sha256": sha, "size": fi.Size(), "digest": digest, "path": outPath,
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(result)
	} else {
		fmt.Fprintf(stdout, "%s %s\nsha256=%s size=%d\n%s\n", info.Name, info.Version, sha, fi.Size(), outPath)
	}
	return 0
}

func collectEntries(dir, metaFile string, externalMeta bool) ([]signedpkg.Entry, error) {
	var entries []signedpkg.Entry
	var total int64
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (name == ".git" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", path)
		}
		name := d.Name()
		if name == "bundle.json" || name == ".DS_Store" {
			return nil
		}
		if externalMeta && sameFile(path, metaFile) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err := signedpkg.ValidateEntryName(rel); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data = normalizeText(rel, data)
		if len(data) > signedpkg.MaxEntryBytes {
			return fmt.Errorf("%s exceeds entry size cap", rel)
		}
		total += int64(len(data))
		if total > signedpkg.MaxTotalBytes {
			return fmt.Errorf("plugin exceeds total size cap")
		}
		entries = append(entries, signedpkg.Entry{Name: rel, Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if externalMeta {
		raw, err := os.ReadFile(metaFile)
		if err != nil {
			return nil, err
		}
		entries = append(entries, signedpkg.Entry{Name: "market.json", Data: normalizeText("market.json", raw)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// normalizeText makes bundles byte-identical across Windows and Linux
// checkouts: text entries are always stored with LF newlines.
func normalizeText(name string, data []byte) []byte {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json", ".js", ".mjs", ".cjs", ".html", ".css", ".md", ".txt", ".svg", ".yml", ".yaml":
		return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	return data
}

func hasEntry(entries []signedpkg.Entry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && aa == bb
}
