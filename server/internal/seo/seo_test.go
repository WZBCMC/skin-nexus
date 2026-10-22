package seo

import (
	"context"
	"strings"
	"testing"
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
	xml := SitemapXML("https://skin.test", "", []string{"/profile/Steve"})
	for _, want := range []string{
		"<loc>https://skin.test/</loc>",
		"<loc>https://skin.test/register</loc>",
		"<loc>https://skin.test/profile/Steve</loc>",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("missing %q in %q", want, xml)
		}
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
