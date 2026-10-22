// Package config loads SkinNexus runtime configuration from environment
// variables (12-factor, .env-friendly for 宝塔 deployments).
package config

import (
	"fmt"
	"os"
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

	// Phase 2: MUA Union member module (off by default; zero surface when off).
	UnionEnabled           bool
	UnionAPIRoot           string
	UnionMemberKey         string
	UnionHostPublicKeyPath string
	UnionAllowUpdateBkKey  bool
	UnionQueryEmailMode    string // "full" | "off"
	UnionSigPrivateKey     string // Union OAuth2 sig private key PEM path
	UnionHubOrigin         string
}

func Load() (*Config, error) {
	c := &Config{
		Listen:      getenv("SKIN_LISTEN", ":8080"),
		BaseURL:     strings.TrimRight(getenv("SKIN_BASE_URL", "http://localhost:8080"), "/"),
		DatabaseDSN: os.Getenv("SKIN_DATABASE_DSN"),
		TexturesDir: getenv("SKIN_TEXTURES_DIR", "./data/textures"),
		ServerName:  getenv("SKIN_SERVER_NAME", "SkinNexus"),
		SkinDomains: splitCSV(getenv("SKIN_SKIN_DOMAINS", "")),
		SignKeyPath: getenv("SKIN_SIGN_KEY", "./data/sign-key.pem"),
		SiteName:    getenv("SKIN_SITE_NAME", "SkinNexus"),

		OIDCEnabled:    getenv("SKIN_OIDC_ENABLED", "false") == "true",
		OIDCIssuer:     strings.TrimRight(getenv("SKIN_OIDC_ISSUER", strings.TrimRight(getenv("SKIN_BASE_URL", "http://localhost:8080"), "/")), "/"),
		OIDCSignKey:    getenv("SKIN_OIDC_SIGN_KEY", "./data/oidc-key.pem"),
		SharedClientID: getenv("SKIN_OIDC_SHARED_CLIENT_ID", "2"),

		UnionEnabled:           getenv("SKIN_UNION_ENABLED", "false") == "true",
		UnionAPIRoot:           getenv("SKIN_UNION_API_ROOT", "https://skin.mualliance.ltd/api/union"),
		UnionMemberKey:         os.Getenv("SKIN_UNION_MEMBER_KEY"),
		UnionHostPublicKeyPath: getenv("SKIN_UNION_HOST_PUBKEY", "./data/union-host-pubkey.pem"),
		UnionAllowUpdateBkKey:  getenv("SKIN_UNION_ALLOW_UPDATEBACKENDKEY", "false") == "true",
		UnionQueryEmailMode:    getenv("SKIN_UNION_QUERYEMAIL_MODE", "full"),
		UnionSigPrivateKey:     getenv("SKIN_UNION_OAUTH2_SIG_KEY", "./data/union-oauth2-sig.pem"),
		UnionHubOrigin:         getenv("SKIN_UNION_HUB_ORIGIN", "https://skin.mualliance.ltd"),
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
