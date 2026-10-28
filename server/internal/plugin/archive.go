package plugin

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxZipEntries    = 4096
	maxZipEntryBytes = 32 << 20
	maxZipTotalBytes = 64 << 20
	maxZipRatio      = 200
	maxZipNameBytes  = 512
)

var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// extractZip safely extracts a plugin bundle into an empty staging directory.
// It rejects traversal, absolute paths, backslashes, symlinks, Windows
// reserved names, case collisions, oversized entries and zip bombs.
func extractZip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	if len(zr.File) > maxZipEntries {
		return fmt.Errorf("zip has too many entries (%d)", len(zr.File))
	}
	var total int64
	seen := map[string]bool{}
	for _, f := range zr.File {
		name := f.Name
		if err := validateZipName(name); err != nil {
			return err
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			return fmt.Errorf("duplicate/case-colliding zip entry %q", name)
		}
		seen[lower] = true
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filepath.Join(dest, filepath.FromSlash(name)), 0o755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry %q is a symlink", name)
		}
		if !f.Mode().IsRegular() {
			return fmt.Errorf("zip entry %q is not a regular file", name)
		}
		if f.UncompressedSize64 > maxZipEntryBytes {
			return fmt.Errorf("zip entry %q exceeds %d bytes", name, maxZipEntryBytes)
		}
		if f.CompressedSize64 > 0 && f.UncompressedSize64 > 1<<20 &&
			f.UncompressedSize64/f.CompressedSize64 > maxZipRatio {
			return fmt.Errorf("zip entry %q has suspicious compression ratio", name)
		}
		total += int64(f.UncompressedSize64)
		if total > maxZipTotalBytes {
			return fmt.Errorf("zip exceeds %d uncompressed bytes", maxZipTotalBytes)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, maxZipEntryBytes+1))
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if fi, err := os.Stat(target); err != nil || fi.Size() > maxZipEntryBytes {
			return fmt.Errorf("zip entry %q exceeded limits while extracting", name)
		}
	}
	return nil
}

func validateZipName(name string) error {
	if name == "" || len(name) > maxZipNameBytes || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid zip entry name %q", name)
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) || strings.Contains(name, "\x00") {
		return fmt.Errorf("absolute/unsafe zip entry %q", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("traversal zip entry %q", name)
		}
		if strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
			return fmt.Errorf("unsafe zip entry %q", name)
		}
		base := strings.ToLower(strings.TrimSuffix(seg, filepath.Ext(seg)))
		if windowsReserved[base] {
			return fmt.Errorf("reserved zip entry %q", name)
		}
		if strings.Contains(seg, ":") {
			return fmt.Errorf("unsafe zip entry %q", name)
		}
	}
	return nil
}

// findPluginRoot accepts either a bundle with plugin.json at the root or one
// single top-level directory containing plugin.json.
func findPluginRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		root := filepath.Join(dir, dirs[0])
		if _, err := os.Stat(filepath.Join(root, "plugin.json")); err == nil {
			return root, nil
		}
	}
	return "", errors.New("bundle must contain plugin.json at its root (or one top-level directory)")
}
