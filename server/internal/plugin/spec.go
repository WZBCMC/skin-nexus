package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// ConfigSpec is the declarative config block. Schema is a JSON Schema subset
// validated by config.go; keeping it raw lets the admin surface round-trip
// unknown-but-valid annotations (description/title) without a giant struct.
type ConfigSpec struct {
	Schema json.RawMessage `json:"schema"`
}

// StoreSpec declares the namespaced data a plugin may use. There is no raw
// SQL/table access: only these collections (plus KV) are addressable.
type StoreSpec struct {
	SchemaVersion int                       `json:"schemaVersion,omitempty"`
	Collections   map[string]CollectionSpec `json:"collections,omitempty"`
}

type CollectionSpec struct {
	MaxRows  int `json:"maxRows,omitempty"`
	MaxBytes int `json:"maxBytes,omitempty"`
}

// RouteSpec is a manifest-declared route. The runtime registration must match
// this declaration exactly (method + path); the host applies the tier/limits.
type RouteSpec struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Tier      string `json:"tier,omitempty"` // public (default) | session | admin | superadmin
	RateLimit string `json:"rateLimit,omitempty"`
	BodyLimit int    `json:"bodyLimit,omitempty"`
	CSRF      bool   `json:"csrf,omitempty"`
}

// UISpec is the zero-build UI declaration: fixed slots and optional pages.
type UISpec struct {
	Pages  []PageSpec                   `json:"pages,omitempty"`
	Slots  []SlotSpec                   `json:"slots,omitempty"`
	Assets string                       `json:"assets,omitempty"`
	I18n   map[string]map[string]string `json:"i18n,omitempty"`
}

type PageSpec struct {
	Plugin string     `json:"plugin,omitempty"` // filled by Manager.UI, ignored in manifests
	ID     string     `json:"id"`
	Path   string     `json:"path"` // /ext/<name>/<slug>
	Area   string     `json:"area"` // user | admin
	Nav    *NavSpec   `json:"nav,omitempty"`
	Render RenderSpec `json:"render"`
	Perm   int        `json:"perm,omitempty"`
}

type NavSpec struct {
	Group string `json:"group,omitempty"` // user nav group; admin uses "admin"
	Label string `json:"label"`
}

type SlotSpec struct {
	Plugin string     `json:"plugin,omitempty"` // filled by Manager.UI
	ID     string     `json:"id"`
	Render RenderSpec `json:"render"`
	Height int        `json:"height,omitempty"`
}

type RenderSpec struct {
	Type   string `json:"type"` // iframe | schema
	Src    string `json:"src,omitempty"`
	Schema string `json:"schema,omitempty"`
}

type RequiresSpec struct {
	Plugins map[string]string `json:"plugins,omitempty"` // name -> semver constraint
}

var (
	pageIDRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	collectionRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	rateLimitRe  = regexp.MustCompile(`^[1-9][0-9]*/(s|m|h)$`)
)

// fixedSlotIDs is the versioned list of UI surfaces plugins may target. It is
// intentionally closed: an arbitrary slot name would let a plugin inject into
// pages the host never reviewed.
var fixedSlotIDs = map[string]bool{
	"app.topbar.after":       true,
	"app.nav.user":           true,
	"app.nav.admin":          true,
	"auth.login.after":       true,
	"dashboard.cards":        true,
	"skinlib.list.sidebar":   true,
	"skinlib.detail.sidebar": true,
	"profile.card.footer":    true,
	"player.card.footer":     true,
	"footer.before":          true,
}

var allowedRouteMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
}

func validateV2Manifest(m *Manifest, dir string) error {
	if m.Runtime == "" {
		m.Runtime = "goja"
	}
	if m.Runtime != "goja" {
		return fmt.Errorf("runtime %q unsupported (only goja in this release)", m.Runtime)
	}
	if err := validateConfigSpec(m.Config); err != nil {
		return err
	}
	if err := validateStoreSpec(m.Store); err != nil {
		return err
	}
	if err := validateRoutes(m); err != nil {
		return err
	}
	if err := validateUI(m, dir); err != nil {
		return err
	}
	if err := validateRequires(m); err != nil {
		return err
	}
	return nil
}

func validateConfigSpec(c *ConfigSpec) error {
	if c == nil {
		return nil
	}
	if len(c.Schema) == 0 || len(c.Schema) > maxManifestBytes {
		return errors.New("config.schema must be a non-empty JSON object <= manifest size")
	}
	var obj map[string]any
	if err := json.Unmarshal(c.Schema, &obj); err != nil {
		return fmt.Errorf("config.schema: %w", err)
	}
	if obj["type"] != "object" {
		return errors.New("config.schema.type must be object")
	}
	if _, err := parseConfigSchema(c.Schema); err != nil {
		return err
	}
	return nil
}

func validateStoreSpec(s *StoreSpec) error {
	if s == nil {
		return nil
	}
	if s.SchemaVersion < 0 {
		return errors.New("store.schemaVersion must be >= 0")
	}
	if len(s.Collections) > 20 {
		return errors.New("too many store collections (max 20)")
	}
	for name, c := range s.Collections {
		if !collectionRe.MatchString(name) {
			return fmt.Errorf("invalid collection name %q", name)
		}
		if c.MaxRows < 0 || c.MaxRows > 10000 {
			return fmt.Errorf("collection %q maxRows out of range", name)
		}
		if c.MaxBytes < 0 || c.MaxBytes > 20<<20 {
			return fmt.Errorf("collection %q maxBytes out of range", name)
		}
	}
	return nil
}

func validateRoutes(m *Manifest) error {
	seen := map[string]bool{}
	for i := range m.Routes {
		r := &m.Routes[i]
		r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
		if !allowedRouteMethods[r.Method] {
			return fmt.Errorf("route method %q not allowed", r.Method)
		}
		clean := path.Clean("/" + strings.TrimPrefix(r.Path, "/"))
		if clean == "/" || hasDotDotSegment(clean) {
			return fmt.Errorf("invalid route path %q", r.Path)
		}
		if clean == "/_csrf" {
			return errors.New("route path /_csrf is reserved")
		}
		if err := validatePathParams(clean); err != nil {
			return fmt.Errorf("route %s: %w", clean, err)
		}
		r.Path = clean
		if r.Tier == "" {
			r.Tier = string(RouteTierPublic)
		}
		tier := RouteTier(r.Tier)
		switch tier {
		case RouteTierPublic, RouteTierSession, RouteTierAdmin, RouteTierSuperAdmin:
		default:
			return fmt.Errorf("route %s %s: invalid tier %q", r.Method, r.Path, r.Tier)
		}
		if r.RateLimit != "" && !rateLimitRe.MatchString(r.RateLimit) {
			return fmt.Errorf("route %s %s: invalid rateLimit %q", r.Method, r.Path, r.RateLimit)
		}
		if r.BodyLimit < 0 || r.BodyLimit > 1<<20 {
			return fmt.Errorf("route %s %s: bodyLimit out of range", r.Method, r.Path)
		}
		if (tier == RouteTierSession || tier == RouteTierAdmin || tier == RouteTierSuperAdmin) && r.Method != "GET" && r.Method != "HEAD" && !r.CSRF {
			return fmt.Errorf("route %s %s: non-GET %s routes must enable csrf", r.Method, r.Path, tier)
		}
		if err := requireTierPermission(m, tier); err != nil {
			return fmt.Errorf("route %s %s: %w", r.Method, r.Path, err)
		}
		key := r.Method + " " + r.Path
		if seen[key] {
			return fmt.Errorf("duplicate route %s", key)
		}
		seen[key] = true
	}
	if len(m.Routes) > maxRoutesPerPlugin {
		return fmt.Errorf("too many routes (%d > %d)", len(m.Routes), maxRoutesPerPlugin)
	}
	return nil
}

func requireTierPermission(m *Manifest, tier RouteTier) error {
	need := ""
	switch tier {
	case RouteTierPublic:
		need = PermRegisterRoute
	case RouteTierSession:
		need = PermHTTPRoutesSession
	case RouteTierAdmin:
		need = PermHTTPRoutesAdmin
	case RouteTierSuperAdmin:
		need = PermHTTPRoutesSuper
	}
	if need == "" {
		return nil
	}
	for _, p := range m.Permissions {
		if p == need {
			return nil
		}
	}
	return fmt.Errorf("missing permission %q", need)
}

func validateUI(m *Manifest, dir string) error {
	u := m.UI
	if u == nil {
		return nil
	}
	if len(u.Pages) > 32 || len(u.Slots) > 32 {
		return errors.New("too many UI pages/slots (max 32 each)")
	}
	assets := strings.TrimSpace(u.Assets)
	if assets != "" {
		assets = strings.TrimPrefix(assets, "/")
		clean := path.Clean(assets)
		if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, "..") {
			return fmt.Errorf("invalid ui.assets %q", u.Assets)
		}
		u.Assets = strings.TrimSuffix(clean, "/") + "/"
	}
	if assets == "" && (hasIframe(u.Pages, u.Slots)) {
		return errors.New("ui.assets is required when a page/slot uses iframe")
	}
	pageIDs := map[string]bool{}
	pagePaths := map[string]bool{}
	for _, p := range u.Pages {
		if !pageIDRe.MatchString(p.ID) {
			return fmt.Errorf("invalid page id %q", p.ID)
		}
		if pageIDs[p.ID] {
			return fmt.Errorf("duplicate page id %q", p.ID)
		}
		pageIDs[p.ID] = true
		prefix := "/ext/" + m.Name + "/"
		if !strings.HasPrefix(p.Path, prefix) || len(p.Path) == len(prefix) {
			return fmt.Errorf("page %q path must start with %s", p.ID, prefix)
		}
		if pagePaths[p.Path] {
			return fmt.Errorf("duplicate page path %q", p.Path)
		}
		pagePaths[p.Path] = true
		if p.Area != "user" && p.Area != "admin" {
			return fmt.Errorf("page %q area must be user or admin", p.ID)
		}
		if p.Nav != nil && (p.Nav.Label == "" || len(p.Nav.Label) > 128) {
			return fmt.Errorf("page %q nav label invalid", p.ID)
		}
		if err := validateRender(m, p.Render, assets); err != nil {
			return fmt.Errorf("page %q: %w", p.ID, err)
		}
		if p.Perm < -1 || p.Perm > 2 {
			return fmt.Errorf("page %q perm out of range", p.ID)
		}
	}
	slotSeen := map[string]bool{}
	for _, s := range u.Slots {
		if !fixedSlotIDs[s.ID] {
			return fmt.Errorf("unknown UI slot %q", s.ID)
		}
		if slotSeen[s.ID] {
			return fmt.Errorf("duplicate UI slot %q", s.ID)
		}
		slotSeen[s.ID] = true
		if s.Height < 0 || s.Height > 2000 {
			return fmt.Errorf("slot %q height out of range", s.ID)
		}
		if err := validateRender(m, s.Render, assets); err != nil {
			return fmt.Errorf("slot %q: %w", s.ID, err)
		}
	}
	totalI18n := 0
	for loc, kv := range u.I18n {
		if loc != "zh-CN" && loc != "en" {
			return fmt.Errorf("unsupported UI locale %q", loc)
		}
		for k, v := range kv {
			if k == "" || len(k) > 128 || len(v) > 2048 {
				return fmt.Errorf("invalid UI i18n entry %q", k)
			}
			totalI18n += len(k) + len(v)
		}
	}
	if totalI18n > 64<<10 {
		return errors.New("UI i18n exceeds 64 KiB")
	}
	return nil
}

func hasIframe(pages []PageSpec, slots []SlotSpec) bool {
	for _, p := range pages {
		if p.Render.Type == "iframe" {
			return true
		}
	}
	for _, s := range slots {
		if s.Render.Type == "iframe" {
			return true
		}
	}
	return false
}

func validateRender(m *Manifest, r RenderSpec, assets string) error {
	switch r.Type {
	case "iframe":
		if r.Src == "" || strings.HasPrefix(r.Src, "/") || strings.Contains(r.Src, "..") {
			return fmt.Errorf("invalid iframe src %q", r.Src)
		}
		if assets == "" {
			return errors.New("iframe requires ui.assets")
		}
	case "schema":
		if r.Schema == "" || !pageIDRe.MatchString(r.Schema) {
			return fmt.Errorf("invalid schema id %q", r.Schema)
		}
	default:
		return fmt.Errorf("render.type must be iframe or schema (got %q)", r.Type)
	}
	return nil
}

func validateRequires(m *Manifest) error {
	if m.Requires != nil {
		for name, constraint := range m.Requires.Plugins {
			if !nameRe.MatchString(name) {
				return fmt.Errorf("invalid required plugin %q", name)
			}
			if strings.TrimSpace(constraint) == "" {
				return fmt.Errorf("required plugin %q has empty constraint", name)
			}
		}
	}
	for _, name := range m.Conflicts {
		if !nameRe.MatchString(name) {
			return fmt.Errorf("invalid conflict plugin %q", name)
		}
	}
	return nil
}

var paramNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validatePathParams(p string) error {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	for i, seg := range segs {
		if !strings.HasPrefix(seg, "{") {
			continue
		}
		if !strings.HasSuffix(seg, "}") {
			return fmt.Errorf("invalid path parameter %q", seg)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
		catchAll := strings.HasSuffix(inner, "...")
		if catchAll {
			inner = strings.TrimSuffix(inner, "...")
			if i != len(segs)-1 {
				return errors.New("catch-all parameter must be last")
			}
		}
		if !paramNameRe.MatchString(inner) {
			return fmt.Errorf("invalid path parameter name %q", inner)
		}
	}
	return nil
}

func hasDotDotSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}
