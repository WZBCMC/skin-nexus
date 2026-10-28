package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginStateDirAndUploadMax(t *testing.T) {
	t.Setenv("SKIN_DATABASE_DSN", "u:p@tcp(localhost:3306)/x")
	t.Setenv("SKIN_DATA_DIR", t.TempDir())
	t.Setenv("SKIN_PLUGIN_STATE_DIR", "")
	t.Setenv("SKIN_PLUGIN_UPLOAD_MAX", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PluginStateDir != filepath.Join(c.PluginsDir, ".state") {
		t.Fatalf("state dir = %s", c.PluginStateDir)
	}
	if c.PluginUploadMax != 64<<20 {
		t.Fatalf("upload max = %d", c.PluginUploadMax)
	}

	custom := t.TempDir()
	t.Setenv("SKIN_PLUGIN_STATE_DIR", custom)
	t.Setenv("SKIN_PLUGIN_UPLOAD_MAX", "1024")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PluginStateDir != custom || c.PluginUploadMax != 1024 {
		t.Fatalf("custom = %s/%d", c.PluginStateDir, c.PluginUploadMax)
	}
}

func TestPluginMarketURLDefaultAndDisable(t *testing.T) {
	t.Setenv("SKIN_DATABASE_DSN", "u:p@tcp(localhost:3306)/x")
	t.Setenv("SKIN_DATA_DIR", t.TempDir())

	t.Setenv("SKIN_PLUGIN_MARKET_URL", "placeholder")
	_ = os.Unsetenv("SKIN_PLUGIN_MARKET_URL")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PluginMarketURL != OfficialPluginMarketURL {
		t.Fatalf("default market = %q", c.PluginMarketURL)
	}

	t.Setenv("SKIN_PLUGIN_MARKET_URL", "")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PluginMarketURL != "" {
		t.Fatalf("explicit empty should disable, got %q", c.PluginMarketURL)
	}
}
