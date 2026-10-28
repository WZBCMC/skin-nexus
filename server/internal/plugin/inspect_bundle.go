package plugin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skin-nexus/server/internal/signedpkg"
)

// Trust levels recorded in plugin_meta.trust and shown in the admin list.
const (
	TrustVerified   = "verified"
	TrustUnknownKey = "unknown-key"
	TrustUnsigned   = "unsigned"
	TrustInvalid    = "invalid"
)

// BundleInspection is the pre-install trust report shown to a super admin.
type BundleInspection struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Permissions []string `json:"permissions,omitempty"`
	Conflicts   []string `json:"conflicts,omitempty"`
	Status      string   `json:"status"`
	KeyID       string   `json:"keyId,omitempty"`
	Digest      string   `json:"digest,omitempty"`
	ZipSHA      string   `json:"zipSha"`
	Reason      string   `json:"reason,omitempty"`
}

// InspectBytes inspects a bundle held in memory.
func (m *Manager) InspectBytes(ctx context.Context, data []byte) (*BundleInspection, error) {
	_ = ctx
	if err := m.ensureInstallDirs(); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Join(m.stateDir(), "staging"), "inspect-*.zip")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return m.InspectBundle(path)
}

// InspectBundle classifies a bundle and reads its manifest without extracting.
func (m *Manager) InspectBundle(zipPath string) (*BundleInspection, error) {
	res, err := signedpkg.Inspect(zipPath, m.cfg.Keyring)
	if err != nil {
		return nil, err
	}
	mf, err := readZipManifest(zipPath)
	if err != nil {
		return nil, err
	}
	out := &BundleInspection{
		Name:        mf.Name,
		Version:     mf.Version,
		Permissions: mf.Permissions,
		Conflicts:   mf.Conflicts,
		Status:      string(res.Status),
		KeyID:       res.Meta.KeyID,
		Digest:      res.Digest,
		ZipSHA:      res.ZipSHA,
		Reason:      res.Reason,
	}
	if res.HasMeta && (res.Meta.Name != mf.Name || res.Meta.Version != mf.Version) {
		out.Status = TrustInvalid
		out.Reason = "bundle.json name/version does not match plugin.json"
	}
	return out, nil
}

func readZipManifest(zipPath string) (manifestFile, error) {
	raw, err := signedpkg.ReadEntry(zipPath, "plugin.json", maxManifestBytes)
	if errors.Is(err, os.ErrNotExist) {
		name, ferr := findZipManifestEntry(zipPath)
		if ferr != nil {
			return manifestFile{}, ferr
		}
		raw, err = signedpkg.ReadEntry(zipPath, name, maxManifestBytes)
	}
	if err != nil {
		return manifestFile{}, fmt.Errorf("plugin.json: %w", err)
	}
	var mf manifestFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		return manifestFile{}, fmt.Errorf("plugin.json: %w", err)
	}
	if mf.Name == "" || mf.Version == "" {
		return manifestFile{}, errors.New("plugin.json must declare name and version")
	}
	return mf, nil
}

func findZipManifestEntry(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	found := ""
	for _, f := range zr.File {
		if f.Name != "plugin.json" && !(strings.HasSuffix(f.Name, "/plugin.json") && strings.Count(f.Name, "/") == 1) {
			continue
		}
		if found != "" {
			return "", errors.New("bundle has multiple plugin.json files")
		}
		found = f.Name
	}
	if found == "" {
		return "", errors.New("bundle is missing plugin.json")
	}
	return found, nil
}
