// Package seo serves the SEO surface: SPA HTML meta injection (title,
// description, keywords, OG, canonical), /sitemap.xml and /robots.txt.
//
// All settings live in the options table (keys seo_*), edited from the admin
// panel; values are cached in memory for 60s so page serving never blocks on
// the DB. Nothing here touches the /api/yggdrasil JSON surface (freeze rule).
package seo

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
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
		s.Title = e.def.SiteName
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

// FillDefaults returns a ready-to-save Settings with one-click suggestions.
func (e *Engine) FillDefaults(ctx context.Context) Settings {
	s := e.Settings(ctx)
	if s.Title == "" {
		s.Title = e.def.SiteName
	}
	if s.Description == "" {
		s.Description = e.def.SiteName + " — Minecraft 皮肤站 · Yggdrasil 外置登录"
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

// RenderHead replaces the SEO placeholders in the SPA index.html template.
// The template carries markers <!--skin-seo--> so injection is a pure string
// replace — no HTML parsing dependency.
func (e *Engine) RenderHead(ctx context.Context, indexHTML string) string {
	s := e.Settings(ctx)
	if s.NoIndex {
		s.Robots = defaultRobots(s.Canonical, s.SitemapOn, true)
	}
	var sb strings.Builder
	sb.WriteString("<title>" + esc(s.Title) + "</title>\n")
	if s.Description != "" {
		sb.WriteString(`<meta name="description" content="` + esc(s.Description) + `">` + "\n")
	}
	if s.Keywords != "" {
		sb.WriteString(`<meta name="keywords" content="` + esc(s.Keywords) + `">` + "\n")
	}
	sb.WriteString(`<meta name="robots" content="` + robotsContent(s) + `">` + "\n")
	if s.Canonical != "" {
		sb.WriteString(`<link rel="canonical" href="` + esc(s.Canonical) + `/">` + "\n")
	}
	sb.WriteString(`<meta property="og:title" content="` + esc(s.Title) + `">` + "\n")
	sb.WriteString(`<meta property="og:type" content="website">` + "\n")
	if s.Canonical != "" {
		sb.WriteString(`<meta property="og:url" content="` + esc(s.Canonical) + `/">` + "\n")
	}
	if s.Description != "" {
		sb.WriteString(`<meta property="og:description" content="` + esc(s.Description) + `">` + "\n")
	}
	if s.OGImage != "" {
		sb.WriteString(`<meta property="og:image" content="` + esc(s.OGImage) + `">` + "\n")
	}
	return strings.Replace(indexHTML, "<!--skin-seo-->", sb.String(), 1)
}

func robotsContent(s Settings) string {
	if s.NoIndex {
		return "noindex, nofollow"
	}
	return "index, follow"
}

// esc escapes HTML attribute/text content.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// SitemapXML builds the sitemap: static routes first, then player profile
// pages supplied by the caller (bounded to 5000 by the caller).
func SitemapXML(canonical, baseURL string, playerPaths []string) string {
	base := strings.TrimRight(canonical, "/")
	if base == "" {
		base = strings.TrimRight(baseURL, "/")
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	statics := []string{"/", "/register"}
	for _, p := range statics {
		sb.WriteString("  <url><loc>" + esc(base+p) + "</loc></url>\n")
	}
	for _, p := range playerPaths {
		sb.WriteString("  <url><loc>" + esc(base+p) + "</loc></url>\n")
	}
	sb.WriteString("</urlset>\n")
	return sb.String()
}
