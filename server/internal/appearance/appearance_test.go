package appearance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skin-nexus/server/internal/theme"
)

type fakeOpts struct{ kv map[string]string }

func (f *fakeOpts) GetOption(_ context.Context, k string) (string, bool, error) {
	v, ok := f.kv[k]
	return v, ok, nil
}
func (f *fakeOpts) SetOption(_ context.Context, k, v string) error {
	if f.kv == nil {
		f.kv = map[string]string{}
	}
	f.kv[k] = v
	return nil
}

func testRegistry(t *testing.T) *theme.Registry {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "dark")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	man := `{"id":"dark","name":"Dark","version":"1.0.0","inject":{"css":["style.css"],"js":["fx.js"]}}`
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte(man), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := theme.NewRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const tpl = `<html><head><!--skin-theme--><!--skin-custom-head--></head><body><!--skin-custom-body--></body></html>`

func TestRenderLayers(t *testing.T) {
	e := NewEngine(&fakeOpts{kv: map[string]string{
		"active_theme": "dark",
		"custom_css":   ".card{border-radius:20px}",
		"custom_js":    "console.log('hi')",
	}}, testRegistry(t))
	html := e.Render(context.Background(), tpl)
	for _, want := range []string{
		`/theme-assets/dark/style.css`,
		`/theme-assets/dark/fx.js`,
		`<style id="skin-custom-css">`,
		`.card{border-radius:20px}`,
		`<script id="skin-custom-js">`,
		`console.log('hi')`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %q", want, html)
		}
	}
	// order: theme css must come before custom css (custom wins in cascade)
	if strings.Index(html, "/theme-assets/dark/style.css") > strings.Index(html, "skin-custom-css") {
		t.Fatal("theme must inject before custom css")
	}
}

func TestSaveInvalidThemeClears(t *testing.T) {
	e := NewEngine(&fakeOpts{}, testRegistry(t))
	if err := e.Save(context.Background(), Settings{ActiveTheme: "nope", CustomCSS: "x"}); err != nil {
		t.Fatal(err)
	}
	s := e.Settings(context.Background())
	if s.ActiveTheme != "" {
		t.Fatalf("invalid theme kept: %q", s.ActiveTheme)
	}
	if s.CustomCSS != "x" {
		t.Fatalf("custom css lost: %q", s.CustomCSS)
	}
}

func TestNoThemeNoMarkersLeft(t *testing.T) {
	e := NewEngine(&fakeOpts{}, testRegistry(t))
	html := e.Render(context.Background(), tpl)
	if strings.Contains(html, "skin-theme-->") || strings.Contains(html, "skin-custom-") {
		t.Fatalf("markers left: %q", html)
	}
}
