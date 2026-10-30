// Package seo serves the SEO surface: SPA HTML meta injection (title,
// description, keywords, OG, canonical), /sitemap.xml and /robots.txt.
//
// All settings live in the options table (keys seo_*), edited from the admin
// panel; values are cached in memory for 60s so page serving never blocks on
// the DB. Nothing here touches the /api/yggdrasil JSON surface (freeze rule).
package seo

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Settings is the SEO configuration (options keys, all optional).
type Settings struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
	OGImage     string `json:"og_image"`
	Canonical   string `json:"canonical"`
	SitemapOn   bool   `json:"sitemap"`
	Robots      string `json:"robots"`
	NoIndex     bool   `json:"noindex"`
}

// OptionStore is the persistence port (satisfied by *store.Mysql).
type OptionStore interface {
	GetOption(ctx context.Context, k string) (string, bool, error)
	SetOption(ctx context.Context, k, v string) error
}

// defaults filled when options are empty.
type Defaults struct {
	SiteName string
	BaseURL  string
}

// Engine caches settings and renders outputs.
type Engine struct {
	store OptionStore
	def   Defaults

	mu       sync.Mutex
	cached   Settings
	cachedAt time.Time
}

func NewEngine(store OptionStore, def Defaults) *Engine {
	return &Engine{store: store, def: def}
}

var optKeys = []string{
	"seo_title", "seo_description", "seo_keywords", "seo_og_image",
	"seo_canonical", "seo_sitemap", "seo_robots", "seo_noindex",
}

// Settings returns the current configuration (60s cache, defaults applied).
func (e *Engine) Settings(ctx context.Context) Settings {
	e.mu.Lock()
	if time.Since(e.cachedAt) < time.Minute && !e.cachedAt.IsZero() {
		s := e.cached
		e.mu.Unlock()
		return s
	}
	e.mu.Unlock()

	s := Settings{SitemapOn: true}
	vals := map[string]string{}
	for _, k := range optKeys {
		if v, ok, err := e.store.GetOption(ctx, k); err == nil && ok {
			vals[k] = v
		}
	}
	s.Title = vals["seo_title"]
	s.Description = vals["seo_description"]
	s.Keywords = vals["seo_keywords"]
	s.OGImage = vals["seo_og_image"]
	s.Canonical = vals["seo_canonical"]
	s.Robots = vals["seo_robots"]
	if v, err := strconv.ParseBool(vals["seo_sitemap"]); err == nil {
		s.SitemapOn = v
	} else if vals["seo_sitemap"] == "" {
		s.SitemapOn = true
	}
	s.NoIndex, _ = strconv.ParseBool(vals["seo_noindex"])

	// defaults
	if s.Title == "" {
		s.Title = defaultTitle(e.def.SiteName)
	}
	if s.Canonical == "" {
		s.Canonical = strings.TrimRight(e.def.BaseURL, "/")
	}
	if s.Robots == "" {
		s.Robots = defaultRobots(s.Canonical, s.SitemapOn, s.NoIndex)
	}

	e.mu.Lock()
	e.cached, e.cachedAt = s, time.Now()
	e.mu.Unlock()
	return s
}

// Save persists the settings and busts the cache. Empty fields clear keys.
func (e *Engine) Save(ctx context.Context, s Settings) error {
	vals := map[string]string{
		"seo_title":       s.Title,
		"seo_description": s.Description,
		"seo_keywords":    s.Keywords,
		"seo_og_image":    s.OGImage,
		"seo_canonical":   s.Canonical,
		"seo_robots":      s.Robots,
		"seo_sitemap":     strconv.FormatBool(s.SitemapOn),
		"seo_noindex":     strconv.FormatBool(s.NoIndex),
	}
	for k, v := range vals {
		if err := e.store.SetOption(ctx, k, v); err != nil {
			return err
		}
	}
	e.mu.Lock()
	e.cachedAt = time.Time{} // bust
	e.mu.Unlock()
	return nil
}

// titleDescriptor keeps generated titles above search engines' minimum
// length; a bare site name is often too short (Bing flags <15 characters).
const titleDescriptor = "Minecraft 皮肤站 · 皮肤、披风与角色管理"

func defaultTitle(siteName string) string {
	name := strings.TrimSpace(siteName)
	if name == "" {
		return titleDescriptor
	}
	if utf8.RuneCountInString(name) >= 15 {
		return name
	}
	return name + "｜" + titleDescriptor
}

func defaultDescription(siteName string) string {
	name := strings.TrimSpace(siteName)
	if name == "" {
		return "Minecraft 皮肤站：上传与分享皮肤、披风，管理角色、衣柜与皮肤库，支持 Yggdrasil 外置登录与启动器接入。"
	}
	return name + "：上传与分享 Minecraft 皮肤和披风，管理角色、衣柜与皮肤库，支持 Yggdrasil 外置登录与启动器接入。"
}

// FillDefaults returns a ready-to-save Settings with one-click suggestions.
func (e *Engine) FillDefaults(ctx context.Context) Settings {
	s := e.Settings(ctx)
	if s.Title == "" {
		s.Title = defaultTitle(e.def.SiteName)
	}
	if s.Description == "" {
		s.Description = defaultDescription(e.def.SiteName)
	}
	s.SitemapOn = true
	s.NoIndex = false
	s.Canonical = strings.TrimRight(e.def.BaseURL, "/")
	s.Robots = defaultRobots(s.Canonical, true, false)
	return s
}

func defaultRobots(canonical string, sitemapOn, noIndex bool) string {
	var sb strings.Builder
	if noIndex {
		sb.WriteString("User-agent: *\nDisallow: /\n")
		return sb.String()
	}
	sb.WriteString("User-agent: *\nAllow: /\n")
	if sitemapOn && canonical != "" {
		sb.WriteString("Sitemap: " + canonical + "/sitemap.xml\n")
	}
	return sb.String()
}

// PageMeta carries per-page overrides for RenderHeadWith.
type PageMeta struct {
	TitleSuffix string // appended as "<suffix> - <site title>" when set
	Description string // "" = keep the site-wide description
	Path        string // canonical path below the site base ("/profile/steve")
	OGImage     string // absolute per-page image; "" = site-wide OG image
	NoIndex     bool   // private content viewed by its owner: never index
	JSONLD      string // pre-marshalled JSON-LD block ("" = none); "</" is
	// escaped on injection so user-supplied names can never close the
	// script element early.
}

// siteJSONLD builds the site-wide schema.org graph (WebSite + Organization).
// It is emitted on every rendered page so search and AI crawlers can resolve
// the site entity even before they reach a detail page.
func (e *Engine) siteJSONLD(s Settings) string {
	base := strings.TrimRight(s.Canonical, "/")
	if base == "" {
		base = strings.TrimRight(e.def.BaseURL, "/")
	}
	if base == "" {
		return ""
	}
	name := strings.TrimSpace(e.def.SiteName)
	if name == "" {
		name = strings.TrimSpace(s.Title)
	}
	if name == "" {
		return ""
	}
	desc := s.Description
	if desc == "" {
		desc = defaultDescription(e.def.SiteName)
	}
	logo := s.OGImage
	if logo == "" {
		logo = base + "/favicon.svg"
	}
	siteID := base + "/#website"
	orgID := base + "/#organization"
	obj := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type":       "WebSite",
				"@id":         siteID,
				"url":         base + "/",
				"name":        name,
				"description": desc,
				"inLanguage":  "zh-CN",
				"publisher":   map[string]any{"@id": orgID},
				"potentialAction": map[string]any{
					"@type":       "SearchAction",
					"target":      base + "/skinlib?q={search_term_string}",
					"query-input": "required name=search_term_string",
				},
			},
			map[string]any{
				"@type":       "Organization",
				"@id":         orgID,
				"url":         base + "/",
				"name":        name,
				"description": desc,
				"logo": map[string]any{
					"@type": "ImageObject",
					"url":   logo,
				},
			},
		},
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(b)
}

// LLMSTxt renders /llms.txt: a compact, crawl-friendly map of public pages
// for AI answer engines. It is generated from live SEO settings, so it stays
// in sync with the canonical base and description.
func (e *Engine) LLMSTxt(ctx context.Context) string {
	s := e.Settings(ctx)
	base := strings.TrimRight(s.Canonical, "/")
	if base == "" {
		base = strings.TrimRight(e.def.BaseURL, "/")
	}
	name := strings.TrimSpace(e.def.SiteName)
	if name == "" {
		name = strings.TrimSpace(s.Title)
	}
	desc := s.Description
	if desc == "" {
		desc = defaultDescription(e.def.SiteName)
	}
	var sb strings.Builder
	sb.WriteString("# " + name + "\n\n> " + desc + "\n\n")
	sb.WriteString("## Pages\n\n")
	sb.WriteString("- [Home](" + base + "/): latest public skins and site introduction\n")
	sb.WriteString("- [Skin library](" + base + "/skinlib): public Minecraft skins and capes\n")
	sb.WriteString("- [Sitemap](" + base + "/sitemap.xml): crawlable public texture pages\n\n")
	sb.WriteString("## Technical\n\n")
	sb.WriteString("- Yggdrasil-compatible authentication: `" + base + "/api/yggdrasil`\n")
	sb.WriteString("- Launcher API v1 (when the `launcher-api` plugin is enabled): `" + base + "/api/plugins/launcher-api/v1/meta`\n")
	sb.WriteString("- Public texture detail pages are server-rendered; account pages and private textures are `noindex`.\n")
	return sb.String()
}

// RenderHead replaces the SEO placeholders in the SPA index.html template.
// The template carries markers <!--skin-seo--> so injection is a pure string
// replace — no HTML parsing dependency.
func (e *Engine) RenderHead(ctx context.Context, indexHTML string) string {
	return e.RenderHeadWith(ctx, indexHTML, PageMeta{})
}

// RenderHeadWith is RenderHead with per-page overrides so the landing page
// and player pages carry distinct titles/descriptions/canonicals.
func (e *Engine) RenderHeadWith(ctx context.Context, indexHTML string, pm PageMeta) string {
	s := e.Settings(ctx)
	if s.NoIndex {
		s.Robots = defaultRobots(s.Canonical, s.SitemapOn, true)
	}
	title := s.Title
	if pm.TitleSuffix != "" {
		title = pm.TitleSuffix + " - " + s.Title
	}
	desc := s.Description
	if pm.Description != "" {
		desc = pm.Description
	}
	canonical := strings.TrimRight(s.Canonical, "/")
	if pm.Path != "" {
		canonical += "/" + strings.TrimPrefix(pm.Path, "/")
	} else {
		canonical += "/"
	}
	var sb strings.Builder
	sb.WriteString("<title>" + esc(title) + "</title>\n")
	if desc != "" {
		sb.WriteString(`<meta name="description" content="` + esc(desc) + `">` + "\n")
	}
	if s.Keywords != "" {
		sb.WriteString(`<meta name="keywords" content="` + esc(s.Keywords) + `">` + "\n")
	}
	sb.WriteString(`<meta name="robots" content="` + robotsContent(s, pm.NoIndex) + `">` + "\n")
	if canonical != "" {
		sb.WriteString(`<link rel="canonical" href="` + esc(canonical) + `">` + "\n")
	}
	sb.WriteString(`<meta property="og:title" content="` + esc(title) + `">` + "\n")
	sb.WriteString(`<meta property="og:type" content="website">` + "\n")
	siteName := strings.TrimSpace(e.def.SiteName)
	if siteName != "" {
		sb.WriteString(`<meta property="og:site_name" content="` + esc(siteName) + `">` + "\n")
	}
	if canonical != "" {
		sb.WriteString(`<meta property="og:url" content="` + esc(canonical) + `">` + "\n")
	}
	if desc != "" {
		sb.WriteString(`<meta property="og:description" content="` + esc(desc) + `">` + "\n")
	}
	ogImage := s.OGImage
	if pm.OGImage != "" {
		ogImage = pm.OGImage
	}
	if ogImage != "" {
		sb.WriteString(`<meta property="og:image" content="` + esc(ogImage) + `">` + "\n")
		sb.WriteString(`<meta name="twitter:card" content="summary_large_image">` + "\n")
		sb.WriteString(`<meta name="twitter:image" content="` + esc(ogImage) + `">` + "\n")
	} else {
		sb.WriteString(`<meta name="twitter:card" content="summary">` + "\n")
	}
	sb.WriteString(`<meta name="twitter:title" content="` + esc(title) + `">` + "\n")
	if desc != "" {
		sb.WriteString(`<meta name="twitter:description" content="` + esc(desc) + `">` + "\n")
	}
	if siteLD := e.siteJSONLD(s); siteLD != "" {
		sb.WriteString(`<script type="application/ld+json">` +
			strings.ReplaceAll(siteLD, "</", "<\\/") + `</script>` + "\n")
	}
	if pm.JSONLD != "" {
		// "</" → "<\/" keeps JSON semantics intact (legal in JSON strings)
		// while making it impossible to close the script element early.
		sb.WriteString(`<script type="application/ld+json">` +
			strings.ReplaceAll(pm.JSONLD, "</", "<\\/") + `</script>` + "\n")
	}
	return strings.Replace(indexHTML, "<!--skin-seo-->", sb.String(), 1)
}

func robotsContent(s Settings, pageNoIndex bool) string {
	if s.NoIndex || pageNoIndex {
		return "noindex, nofollow"
	}
	return "index, follow"
}

// esc escapes HTML attribute/text content.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// SitemapEntry is one dynamic sitemap URL with an optional lastmod.
type SitemapEntry struct {
	Path    string    // "/skinlib/42"
	LastMod time.Time // zero = omit <lastmod>
}

// SitemapXML builds the sitemap: static routes first, then the dynamic
// entries supplied by the caller (bounded to 5000 by the caller).
func SitemapXML(canonical, baseURL string, entries []SitemapEntry) string {
	base := strings.TrimRight(canonical, "/")
	if base == "" {
		base = strings.TrimRight(baseURL, "/")
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	// Only public, crawlable routes: the landing page, the anonymous-readable
	// skin library list (/skinlib, 2026-10-27 逻辑修复：公开列表须收录、注册页
	// 不收录), and the server-rendered texture detail pages. /user/* stays
	// behind login — never advertise it.
	statics := []string{"/", "/skinlib"}
	writeURL := func(p string, lastmod time.Time) {
		sb.WriteString("  <url><loc>" + esc(base+p) + "</loc>")
		if !lastmod.IsZero() {
			sb.WriteString("<lastmod>" + lastmod.UTC().Format("2006-01-02") + "</lastmod>")
		}
		sb.WriteString("</url>\n")
	}
	for _, p := range statics {
		writeURL(p, time.Time{})
	}
	for _, e := range entries {
		writeURL(e.Path, e.LastMod)
	}
	sb.WriteString("</urlset>\n")
	return sb.String()
}
