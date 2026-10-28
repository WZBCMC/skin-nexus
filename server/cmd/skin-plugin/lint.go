package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skin-nexus/server/internal/marketfmt"
	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/signedpkg"
)

// runLint validates a plugin source tree without a signing key. CI uses it on
// pull requests so contributors never need (or receive) the private key.
func runLint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "plugin source directory")
	registry := fs.String("registry", "", "published registry to enforce version monotonicity")
	hostVersion := fs.String("host-version", "", "host version to check minServer against")
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "lint: --dir is required")
		return 2
	}
	info, err := plugin.InspectDir(*dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	meta, err := readMarketMeta(filepath.Join(*dir, "market.json"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if meta == nil {
		fmt.Fprintln(stderr, "lint: market.json is required for publishing")
		return 1
	}
	if err := validateMarketMeta(info, meta); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *hostVersion != "" {
		if err := info.CheckMinServer(*hostVersion); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if err := lintSourceTree(*dir, info); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	latest := ""
	if *registry != "" {
		latest, err = publishedVersion(*registry, info.Name)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if latest != "" {
			cmp, err := marketfmt.CompareVersions(info.Version, latest)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			if cmp <= 0 {
				fmt.Fprintf(stderr, "lint: version %s must be greater than published %s\n", info.Version, latest)
				return 1
			}
		}
	}
	result := map[string]any{
		"name": info.Name, "version": info.Version,
		"permissions": info.Permissions, "published": latest,
	}
	if *asJSON {
		_ = json.NewEncoder(stdout).Encode(result)
	} else {
		fmt.Fprintf(stdout, "%s %s ok (%d permissions)\n", info.Name, info.Version, len(info.Permissions))
	}
	return 0
}

func publishedVersion(registryPath, name string) (string, error) {
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		return "", fmt.Errorf("registry: %w", err)
	}
	var reg marketfmt.Registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return "", fmt.Errorf("registry JSON: %w", err)
	}
	for _, p := range reg.Packages {
		if p.Name == name {
			return p.Version, nil
		}
	}
	return "", nil
}

// lintSourceTree mirrors the packer's safety checks without producing a zip.
func lintSourceTree(dir string, info plugin.Info) error {
	if info.UI != nil && info.UI.Assets != "" {
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(info.UI.Assets))); err != nil || !st.IsDir() {
			return fmt.Errorf("ui.assets %s is missing", info.UI.Assets)
		}
	}
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
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", rel)
		}
		if d.Name() == "bundle.json" {
			return fmt.Errorf("bundle.json must not be committed: %s", rel)
		}
		if err := signedpkg.ValidateEntryName(rel); err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".exe", ".dll", ".so", ".dylib":
			return fmt.Errorf("native binary not allowed: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > signedpkg.MaxEntryBytes {
			return fmt.Errorf("%s exceeds the per-file size cap", rel)
		}
		total += int64(len(data))
		if total > signedpkg.MaxTotalBytes {
			return fmt.Errorf("plugin exceeds the total size cap")
		}
		if strings.Contains(string(data), "BEGIN PRIVATE KEY") {
			return fmt.Errorf("private key material found in %s", rel)
		}
		return nil
	})
	return err
}
