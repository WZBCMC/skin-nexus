package seo

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeOpts is a minimal in-memory OptionStore.
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

func TestSettingsDefaults(t *testing.T) {
	e := NewEngine(&fakeOpts{}, Defaults{SiteName: "SkinNexus", BaseURL: "https://skin.test"})
	s := e.Settings(context.Background())
	if s.Title != "SkinNexus" {
		t.Fatalf("title = %q", s.Title)
	}
	if !s.SitemapOn {
		t.Fatal("sitemap should default on")
	}
	if !strings.Contains(s.Robots, "Sitemap: https://skin.test/sitemap.xml") {
		t.Fatalf("robots = %q", s.Robots)
	}
}

func TestRenderHeadInjectsMeta(t *testing.T) {
	e := NewEngine(&fakeOpts{kv: map[string]string{
		"seo_title":       "我的皮肤站",
		"seo_description": "desc",
		"seo_keywords":    "mc,skin",
		"seo_og_image":    "https://skin.test/og.png",
	}}, Defaults{SiteName: "SkinNexus", BaseURL: "https://skin.test"})
	html := e.RenderHead(context.Background(), "<html><head><!--skin-seo--></head><body></body></html>")
	for _, want := range []string{
		"<title>我的皮肤站</title>",
		`name="description" content="desc"`,
		`name="keywords" content="mc,skin"`,
		`property="og:image" content="https://skin.test/og.png"`,
		`rel="canonical" href="https://skin.test/"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %q", want, html)
		}
	}
}

func TestRenderHeadEscapes(t *testing.T) {
	e := NewEngine(&fakeOpts{kv: map[string]string{
		"seo_title": `<script>alert(1)</script>`,
	}}, Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	html := e.RenderHead(context.Background(), "<!--skin-seo-->")
	if strings.Contains(html, "<script>alert") {
		t.Fatalf("unescaped title: %q", html)
	}
}

func TestNoIndex(t *testing.T) {
	e := NewEngine(&fakeOpts{kv: map[string]string{"seo_noindex": "true"}},
		Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	s := e.Settings(context.Background())
	if !strings.Contains(s.Robots, "Disallow: /") {
		t.Fatalf("robots = %q", s.Robots)
	}
	html := e.RenderHead(context.Background(), "<!--skin-seo-->")
	if !strings.Contains(html, `content="noindex, nofollow"`) {
		t.Fatalf("html = %q", html)
	}
}

func TestSitemapXML(t *testing.T) {
	xml := SitemapXML("https://skin.test", "", []SitemapEntry{
		{Path: "/skinlib/42", LastMod: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{Path: "/skinlib/43"},
	})
	for _, want := range []string{
		"<loc>https://skin.test/</loc>",
		"<loc>https://skin.test/skinlib</loc>",
		"<loc>https://skin.test/skinlib/42</loc>",
		"<lastmod>2026-10-01</lastmod>",
		"<loc>https://skin.test/skinlib/43</loc>",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("missing %q in %q", want, xml)
		}
	}
	// entry without lastmod must not carry an empty <lastmod> element
	if strings.Count(xml, "<lastmod>") != 1 {
		t.Fatalf("lastmod should appear once: %q", xml)
	}
	// player profile pages are gone from the sitemap (SPA shells, no
	// anonymous content — soft-404 poison)
	if strings.Contains(xml, "/profile/") {
		t.Fatalf("profile entries must not be listed: %q", xml)
	}
}

// Per-page JSON-LD is injected into the SEO block and can never close its
// script element early, even with adversarial texture names.
func TestRenderHeadJSONLD(t *testing.T) {
	e := NewEngine(&fakeOpts{}, Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	pm := PageMeta{JSONLD: `{"@type":"CreativeWork","name":"</script><img src=x>"}`}
	html := e.RenderHeadWith(context.Background(), "<!--skin-seo-->", pm)
	if !strings.Contains(html, `type="application/ld+json"`) {
		t.Fatalf("JSON-LD block missing: %q", html)
	}
	if strings.Contains(html, "</script><img") {
		t.Fatalf("raw </ not neutralized: %q", html)
	}
	if !strings.Contains(html, `<\/script>`) {
		t.Fatalf("escaped form missing: %q", html)
	}
}

// Per-page NoIndex wins over the site-wide indexable setting (private
// texture viewed by its owner).
func TestPerPageNoIndex(t *testing.T) {
	e := NewEngine(&fakeOpts{}, Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	html := e.RenderHeadWith(context.Background(), "<!--skin-seo-->", PageMeta{NoIndex: true})
	if !strings.Contains(html, `content="noindex, nofollow"`) {
		t.Fatalf("per-page noindex missing: %q", html)
	}
}

// Per-page OG image overrides the site-wide default.
func TestPerPageOGImage(t *testing.T) {
	e := NewEngine(&fakeOpts{kv: map[string]string{"seo_og_image": "https://skin.test/site.png"}},
		Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	html := e.RenderHeadWith(context.Background(), "<!--skin-seo-->", PageMeta{OGImage: "https://skin.test/preview/7"})
	if !strings.Contains(html, `content="https://skin.test/preview/7"`) {
		t.Fatalf("per-page og:image missing: %q", html)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	e := NewEngine(&fakeOpts{}, Defaults{SiteName: "X", BaseURL: "https://skin.test"})
	s := e.FillDefaults(context.Background())
	s.Title = "新标题"
	if err := e.Save(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if got := e.Settings(context.Background()).Title; got != "新标题" {
		t.Fatalf("title after save = %q", got)
	}
}
