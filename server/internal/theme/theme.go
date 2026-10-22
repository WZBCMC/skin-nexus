// Package theme loads SkinNexus theme packs from a themes directory
// (default data/themes/<id>/, configurable via SKIN_THEMES_DIR).
//
// Theme packs are pure CSS + optional JS + theme.json — no build step, so an
// AI (or a human) can generate one from docs/theme-spec.md alone. The active
// theme id lives in options (key "active_theme"); custom CSS/JS stored in
// options (custom_css / custom_js) are layered on top by the HTML wrapper.
//
// Security: pack ids and asset file names are whitelist-validated
// ([a-z0-9][a-z0-9_-]*, asset files also allow dots) so a malicious pack
// cannot escape its directory. Theme JS runs with the same trust level as
// admin-supplied custom JS — the admin panel states this explicitly.
package theme

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Manifest is the theme.json schema (see docs/theme-spec.md).
type Manifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Author      string            `json:"author,omitempty"`
	Description string            `json:"description,omitempty"`
	Extends     string            `json:"extends,omitempty"`
	Inject      Inject            `json:"inject"`
	Slots       map[string]string `json:"slots,omitempty"`
	Palette     map[string]string `json:"palette,omitempty"`
	Requires    Requires          `json:"requires,omitempty"`
}

// Inject lists pack-relative files to load, in order.
type Inject struct {
	CSS []string `json:"css,omitempty"`
	JS  []string `json:"js,omitempty"`
}

// Requires gates theme loading on the SkinNexus version ("requires": {"skin_nexus": ">=0.1"}).
type Requires struct {
	SkinNexus string `json:"skin_nexus,omitempty"`
}

// Theme is a loaded pack: manifest + resolved absolute base dir.
type Theme struct {
	Manifest Manifest
	BaseDir  string
}

var (
	idRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	fileRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

// ErrBadID / ErrNotFound signal pack validation failures.
var (
	ErrBadID    = errors.New("theme: invalid id")
	ErrNotFound = errors.New("theme: not found")
)

// Registry is the in-memory catalog of installed packs.
type Registry struct {
	dir string

	mu     sync.RWMutex
	themes map[string]*Theme
}

// NewRegistry scans dir. A missing directory is not an error (no themes yet).
func NewRegistry(dir string) (*Registry, error) {
	r := &Registry{dir: dir, themes: map[string]*Theme{}}
	if err := r.Rescan(); err != nil {
		return nil, err
	}
	return r, nil
}

// Dir returns the themes root directory.
func (r *Registry) Dir() string { return r.dir }

// Rescan re-reads the themes directory (called at boot and after installs).
func (r *Registry) Rescan() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.mu.Lock()
			r.themes = map[string]*Theme{}
			r.mu.Unlock()
			return nil
		}
		return err
	}
	found := map[string]*Theme{}
	for _, e := range entries {
		if !e.IsDir() || !idRe.MatchString(e.Name()) {
			continue // unknown ids are skipped, never trusted
		}
		base := filepath.Join(r.dir, e.Name())
		raw, err := os.ReadFile(filepath.Join(base, "theme.json"))
		if err != nil {
			continue // packs without a manifest are ignored
		}
		var m Manifest
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		// manifest id must match the directory name (no aliasing surprises)
		if m.ID != e.Name() {
			continue
		}
		found[m.ID] = &Theme{Manifest: m, BaseDir: base}
	}
	r.mu.Lock()
	r.themes = found
	r.mu.Unlock()
	return nil
}

// List returns manifests sorted by id.
func (r *Registry) List() []Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Manifest, 0, len(r.themes))
	for _, t := range r.themes {
		out = append(out, t.Manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns one pack or ErrNotFound.
func (r *Registry) Get(id string) (*Theme, error) {
	if !idRe.MatchString(id) {
		return nil, ErrBadID
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.themes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

// ResolveAsset validates a pack-relative asset request and returns the
// absolute path. Anything outside the pack dir is rejected.
func (r *Registry) ResolveAsset(id, name string) (string, error) {
	t, err := r.Get(id)
	if err != nil {
		return "", err
	}
	if !fileRe.MatchString(name) {
		return "", ErrBadID
	}
	full := filepath.Join(t.BaseDir, name)
	// defense in depth: resolved path must stay inside the pack directory
	if !strings.HasPrefix(full, t.BaseDir+string(os.PathSeparator)) {
		return "", ErrBadID
	}
	if st, err := os.Stat(full); err != nil || st.IsDir() {
		return "", ErrNotFound
	}
	return full, nil
}

// HeadHTML renders the active theme's inject list as HTML tags, served at
// /theme-assets/<id>/<file> with long-lived caching.
func (t *Theme) HeadHTML() string {
	var sb strings.Builder
	for _, f := range t.Manifest.Inject.CSS {
		if !fileRe.MatchString(f) {
			continue
		}
		sb.WriteString(`<link rel="stylesheet" href="/theme-assets/` + t.Manifest.ID + `/` + f + `">` + "\n")
	}
	for _, f := range t.Manifest.Inject.JS {
		if !fileRe.MatchString(f) {
			continue
		}
		sb.WriteString(`<script src="/theme-assets/` + t.Manifest.ID + `/` + f + `"></script>` + "\n")
	}
	return sb.String()
}
