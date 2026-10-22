// Package plugin defines the phase-1 frozen hook contract and the PluginHost
// interface. The runtime (goja, frozen by the M0 spike) plugs in behind this
// contract; process-outside hosts can be added later without changing hooks.
//
// Contract versioning: hooks are additive-versioned (SkinHookV1...). Plugins
// declare the hook API version in manifest.json; the host refuses to load a
// plugin requiring a newer major. Permission manifest is DECLARATIVE +
// AUDITED, not enforced isolation (phase-1 trust model: reviewed plugins).
package plugin

import "context"

// HookAPIVersion is the current hook contract major.
const HookAPIVersion = 1

// Manifest is the plugin's plugin.json.
type Manifest struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	HookAPI      int      `json:"hookApi"`
	Permissions  []string `json:"permissions"` // declared capabilities, audited on load
	Entrypoint   string   `json:"entrypoint"`  // script file, e.g. "index.js"
	MinServer    string   `json:"minServer,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// Permission strings (declarative, audit-only in phase 1).
const (
	PermReadProfiles  = "profiles:read"
	PermEditProfiles  = "profiles:write"
	PermReadTextures  = "textures:read"
	PermEditTextures  = "textures:write"
	PermRegisterRoute = "http:routes"
	PermMenuEntry     = "admin:menu"
	PermAuthHooks     = "auth:hooks" // covers join/hasJoined-adjacent hooks
)

// ---- P0 hook contexts (frozen v1; 15-20 hooks, additive from here) ----

// UserProfileFilter mutates a profile payload before it is served
// (CustomSkinAPI / Yggdrasil profile). Filter = value transform.
type UserProfileFilter struct {
	PlayerID    int64
	PlayerName  string
	UUID        string
	TexturesURL map[string]string // "SKIN"/"CAPE" -> url
}

// TextureURLFilter rewrites texture URLs at response time (CDN, mirrors).
type TextureURLFilter struct {
	Hash string
	URL  string
	Type string // "skin" | "cape"
}

// TextureUploadedEvent fires after a texture is committed to the store.
type TextureUploadedEvent struct {
	UserID   int64
	PlayerID int64
	Hash     string
	Type     string
}

// UserLifecycleEvent fires for register/login/delete.
type UserLifecycleEvent struct {
	Kind   string // "registered" | "logged_in" | "deleted"
	UserID int64
}

// AuthPhaseEvent fires around Yggdrasil session issuance (fail-open unless
// the hook opts into fail-closed via HostOptions).
type AuthPhaseEvent struct {
	Phase  string // "pre_join" | "post_has_joined"
	UUID   string
	Player string
}

// RouteRegistration lets a plugin contribute read-only HTTP routes
// under /api/plugins/<name>/ (permission: http:routes).
type RouteRegistration struct {
	Method  string
	Path    string
	Handler func(ctx context.Context, req map[string]interface{}) (int, interface{})
}

// HostOptions carries per-hook policy decided by the host.
type HostOptions struct {
	TimeoutMS   int  // default 100; join/hasJoined hooks default 50
	FailClosed  bool // default false (fail-open): a crashing plugin must not block login
}

// HookBus is the extension surface plugins program against.
type HookBus interface {
	OnUserProfileFilter(fn func(context.Context, *UserProfileFilter) error, opts HostOptions) error
	OnTextureURLFilter(fn func(context.Context, *TextureURLFilter) error, opts HostOptions) error
	OnTextureUploaded(fn func(context.Context, TextureUploadedEvent) error, opts HostOptions) error
	OnUserLifecycle(fn func(context.Context, UserLifecycleEvent) error, opts HostOptions) error
	OnAuthPhase(fn func(context.Context, AuthPhaseEvent) error, opts HostOptions) error
	RegisterRoute(r RouteRegistration) error
}

// Host loads plugins and drives their scripts. The goja implementation lives
// in runtime_goja.go (phase-1 skeleton: interface frozen here).
type Host interface {
	Load(dir string) error
	Bus() HookBus
	Close() error
}
