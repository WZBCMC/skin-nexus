// Package config loads SkinNexus runtime configuration from environment
// variables (12-factor, .env-friendly for 宝塔 deployments).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Listen      string // e.g. ":8080"
	BaseURL     string // e.g. https://skin.wzbcmc.cn (used for texture URLs / ALI)
	DatabaseDSN string // user:pass@tcp(host:3306)/skin-nexus?parseTime=true&charset=utf8mb4

	TexturesDir string

	// Yggdrasil metadata
	ServerName  string
	SkinDomains []string // comma-separated

	SignKeyPath string // RSA private key PEM (generated on first boot)

	SiteName string

	// Phase 2: yggc/OIDC passwordless login (internal OIDC provider).
	OIDCEnabled    bool
	OIDCIssuer     string // e.g. https://skin.wzbcmc.cn
	OIDCSignKey    string // OIDC token signing key PEM — MUST differ from SignKeyPath
	SharedClientID string // discovery shared_client_id (launcher public client)

	// Join gate on email_verified (fork's require_verification setting).
	// Default off until the verification mail flow ships (P1).
	RequireVerification bool // SKIN_REQUIRE_VERIFICATION

	// Registration requires a valid invitation code (live plugin's mandatory
	// semantics). Default off = optional code, validated and burned when given.
	RequireInvite bool // SKIN_INVITE_REQUIRED

	// Phase 2: MUA Union member module (off by default; zero surface when off).
	UnionEnabled           bool
	UnionAPIRoot           string
	UnionMemberKey         string
	UnionHostPublicKeyPath string
	UnionAllowUpdateBkKey  bool
	UnionQueryEmailMode    string // "full" | "off"
	UnionSigPrivateKey     string // Union OAuth2 sig private key PEM path
	UnionHubOrigin         string

	// Phase 3 (docker-first): unified data root + schema bootstrap.
	DataDir        string // SKIN_DATA_DIR — textures/keys/themes live under it
	ThemesDir      string // SKIN_THEMES_DIR — theme packs (default <DataDir>/themes)
	AutoMigrate    bool   // SKIN_AUTO_MIGRATE — apply migrations/ on empty DB at boot
	MigrationsDir  string // SKIN_MIGRATIONS_DIR — where *.sql live (default ./migrations)
}

// resolveDataPath picks an explicit env value when set, else joins the data
// root with rel — so a single SKIN_DATA_DIR volume holds everything while
// legacy per-path env vars keep working (explicit always wins).
func resolveDataPath(explicitEnv string, dataDir, rel string) string {
	if v := os.Getenv(explicitEnv); v != "" {
		return v
	}
	return filepath.Join(dataDir, rel)
}

func Load() (*Config, error) {
	dataDir := getenv("SKIN_DATA_DIR", "./data")
	c := &Config{
		Listen:      getenv("SKIN_LISTEN", ":8080"),
		BaseURL:     strings.TrimRight(getenv("SKIN_BASE_URL", "http://localhost:8080"), "/"),
		DatabaseDSN: os.Getenv("SKIN_DATABASE_DSN"),
		TexturesDir: resolveDataPath("SKIN_TEXTURES_DIR", dataDir, "textures"),
		ServerName:  getenv("SKIN_SERVER_NAME", "SkinNexus"),
		SkinDomains: splitCSV(getenv("SKIN_SKIN_DOMAINS", "")),
		SignKeyPath: resolveDataPath("SKIN_SIGN_KEY", dataDir, "sign-key.pem"),
		SiteName:    getenv("SKIN_SITE_NAME", "SkinNexus"),

		OIDCEnabled:    getenv("SKIN_OIDC_ENABLED", "false") == "true",
		RequireVerification: getenv("SKIN_REQUIRE_VERIFICATION", "false") == "true",
		RequireInvite:       getenv("SKIN_INVITE_REQUIRED", "false") == "true",
		OIDCIssuer:     strings.TrimRight(getenv("SKIN_OIDC_ISSUER", strings.TrimRight(getenv("SKIN_BASE_URL", "http://localhost:8080"), "/")), "/"),
		OIDCSignKey:    resolveDataPath("SKIN_OIDC_SIGN_KEY", dataDir, "oidc-key.pem"),
		SharedClientID: getenv("SKIN_OIDC_SHARED_CLIENT_ID", "2"),

		UnionEnabled:           getenv("SKIN_UNION_ENABLED", "false") == "true",
		UnionAPIRoot:           getenv("SKIN_UNION_API_ROOT", "https://skin.mualliance.ltd/api/union"),
		UnionMemberKey:         os.Getenv("SKIN_UNION_MEMBER_KEY"),
		UnionHostPublicKeyPath: resolveDataPath("SKIN_UNION_HOST_PUBKEY", dataDir, "union-host-pubkey.pem"),
		UnionAllowUpdateBkKey:  getenv("SKIN_UNION_ALLOW_UPDATEBACKENDKEY", "false") == "true",
		UnionQueryEmailMode:    getenv("SKIN_UNION_QUERYEMAIL_MODE", "full"),
		UnionSigPrivateKey:     resolveDataPath("SKIN_UNION_OAUTH2_SIG_KEY", dataDir, "union-oauth2-sig.pem"),
		UnionHubOrigin:         getenv("SKIN_UNION_HUB_ORIGIN", "https://skin.mualliance.ltd"),

		DataDir:       dataDir,
		ThemesDir:     getenv("SKIN_THEMES_DIR", filepath.Join(dataDir, "themes")),
		AutoMigrate:   getenv("SKIN_AUTO_MIGRATE", "false") == "true",
		MigrationsDir: getenv("SKIN_MIGRATIONS_DIR", "./migrations"),
	}
	if c.DatabaseDSN == "" {
		return nil, fmt.Errorf("config: SKIN_DATABASE_DSN is required")
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Port extracts the numeric port from Listen for logging.
func (c *Config) Port() int {
	i := strings.LastIndex(c.Listen, ":")
	if i < 0 {
		return 8080
	}
	p, err := strconv.Atoi(c.Listen[i+1:])
	if err != nil {
		return 8080
	}
	return p
}
