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
