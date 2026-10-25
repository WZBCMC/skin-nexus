package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skin-nexus/server/internal/appearance"
	"skin-nexus/server/internal/seo"
	"skin-nexus/server/internal/theme"
)

// 拍板（dev-plan §9）: option site_url → legacy option base_url → env。
func TestResolveBaseURL(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
		env  string
		want string
	}{
		{"site_url wins over base_url and env",
			map[string]string{"site_url": "https://new.example", "base_url": "https://old.example"},
			"http://env.example", "https://new.example"},
		{"legacy base_url used when site_url absent",
			map[string]string{"base_url": "https://old.example"},
			"http://env.example", "https://old.example"},
		{"env fallback when both options absent", map[string]string{},
			"http://env.example", "http://env.example"},
		{"non-http site_url rejected -> env",
			map[string]string{"site_url": "ftp://evil"},
			"http://env.example", "http://env.example"},
		{"garbage base_url rejected -> env",
			map[string]string{"base_url": "not a url"},
			"http://env.example", "http://env.example"},
		{"trailing slashes trimmed",
			map[string]string{"site_url": "https://s.example/"},
			"", "https://s.example"},
		{"empty site_url falls through to base_url",
			map[string]string{"site_url": "", "base_url": "https://old.example"},
			"http://env.example", "https://old.example"},
	}
	for _, c := range cases {
		got := resolveBaseURL(func(key string) (string, bool) {
			v, ok := c.opts[key]
			return v, ok
		}, c.env)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

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

// Regression (2026-10-25): "/" used to bypass the SEO/theme/custom injection
// layers entirely — os.Stat on the empty path resolves to the dist directory
// itself and the old code treated "exists on disk" as "static file", so the
// homepage shipped raw <!--skin-seo--> markers and no theme CSS.
func TestSpaHandlerInjectsEveryHTMLPath(t *testing.T) {
	dir := t.TempDir()
	tpl := "<html><head><!--skin-seo--><!--skin-theme--><!--skin-custom-head--><link rel=\"stylesheet\" href=\"/assets/index-CAFE.css\"></head><body><!--skin-custom-body--></body></html>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(tpl), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := &fakeOpts{kv: map[string]string{"seo_title": "示例站", "custom_css": ".card{border-radius:20px}"}}
	reg, err := theme.NewRegistry(t.TempDir(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	h := spaHandler(dir,
		seo.NewEngine(opts, seo.Defaults{SiteName: "示例站", BaseURL: "https://skin.example.com"}),
		appearance.NewEngine(opts, reg))

	for _, path := range []string{"/", "/index.html", "/register", "/profile/Steve"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body := rec.Body.String()
		for _, marker := range []string{"<!--skin-seo-->", "<!--skin-theme-->", "<!--skin-custom-head-->", "<!--skin-custom-body-->"} {
			if strings.Contains(body, marker) {
				t.Fatalf("GET %s: marker %s left unreplaced: %s", path, marker, body)
			}
		}
		if !strings.Contains(body, "<title>示例站</title>") {
			t.Fatalf("GET %s: SEO head not injected: %s", path, body)
		}
		if !strings.Contains(body, "skin-custom-css") || !strings.Contains(body, ".card{border-radius:20px}") {
			t.Fatalf("GET %s: custom CSS not injected: %s", path, body)
		}
		// cascade order: injected overrides must come AFTER the Vite-built
		// stylesheet, otherwise app :root wins over theme :root (R2 regression).
		if ai, bi := strings.Index(body, "/assets/index-CAFE.css"), strings.Index(body, "skin-custom-css"); ai < 0 || bi < 0 || ai > bi {
			t.Fatalf("GET %s: injected CSS must come after app CSS (app=%d custom=%d): %s", path, ai, bi, body)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if got := rec.Body.String(); got != "console.log(1)" {
		t.Fatalf("asset file must be served raw, got %q", got)
	}
}
