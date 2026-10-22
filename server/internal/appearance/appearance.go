// Package appearance assembles the HTML injection layer: active theme pack
// (<!--skin-theme-->) and admin-supplied custom CSS/JS (<!--skin-custom-head-->
// / <!--skin-custom-body-->), with SEO handled by internal/seo.
//
// Injection order matches the blessing convention: theme CSS first, then
// custom CSS (later wins), then custom JS. Custom CSS/JS are trusted admin
// input — rendered verbatim like blessing does, with an explicit risk warning
// in the admin panel. Option values are cached for 60s.
package appearance

import (
	"context"
	"strings"
	"sync"
	"time"

	"skin-nexus/server/internal/theme"
)

// OptionStore is the persistence port (satisfied by *store.Mysql).
type OptionStore interface {
	GetOption(ctx context.Context, k string) (string, bool, error)
	SetOption(ctx context.Context, k, v string) error
}

// Settings is the admin-editable appearance configuration.
type Settings struct {
	ActiveTheme string `json:"active_theme"`
	CustomCSS   string `json:"custom_css"`
	CustomJS    string `json:"custom_js"`
	CustomHead  string `json:"custom_head_html"`
}

// Engine renders the injection blocks.
type Engine struct {
	opts  OptionStore
	themes *theme.Registry

	mu       sync.Mutex
	cached   Settings
	cachedAt time.Time
}

func NewEngine(opts OptionStore, themes *theme.Registry) *Engine {
	return &Engine{opts: opts, themes: themes}
}

// Settings returns the current appearance config (60s cache).
func (e *Engine) Settings(ctx context.Context) Settings {
	e.mu.Lock()
	if !e.cachedAt.IsZero() && time.Since(e.cachedAt) < time.Minute {
		s := e.cached
		e.mu.Unlock()
		return s
	}
	e.mu.Unlock()

	var s Settings
	get := func(k string) string {
		if v, ok, err := e.opts.GetOption(ctx, k); err == nil && ok {
			return v
		}
		return ""
	}
	s.ActiveTheme = get("active_theme")
	s.CustomCSS = get("custom_css")
	s.CustomJS = get("custom_js")
	s.CustomHead = get("custom_head_html")

	e.mu.Lock()
	e.cached, e.cachedAt = s, time.Now()
	e.mu.Unlock()
	return s
}

// Save persists the appearance config and busts the cache. An invalid or
// missing theme id clears to "no theme" instead of erroring (the admin panel
// shows the installed list anyway).
func (e *Engine) Save(ctx context.Context, s Settings) error {
	if s.ActiveTheme != "" {
		if _, err := e.themes.Get(s.ActiveTheme); err != nil {
			s.ActiveTheme = ""
		}
	}
	pairs := map[string]string{
		"active_theme":      s.ActiveTheme,
		"custom_css":        s.CustomCSS,
		"custom_js":         s.CustomJS,
		"custom_head_html":  s.CustomHead,
	}
	for k, v := range pairs {
		if err := e.opts.SetOption(ctx, k, v); err != nil {
			return err
		}
	}
	e.mu.Lock()
	e.cachedAt = time.Time{}
	e.mu.Unlock()
	return nil
}

// Render fills the injection markers in the SPA index.html.
func (e *Engine) Render(ctx context.Context, indexHTML string) string {
	s := e.Settings(ctx)

	// theme block
	var themeHTML string
	if s.ActiveTheme != "" {
		if t, err := e.themes.Get(s.ActiveTheme); err == nil {
			themeHTML = t.HeadHTML()
		}
	}
	indexHTML = strings.Replace(indexHTML, "<!--skin-theme-->", themeHTML, 1)

	// custom head: <style> + raw head snippets
	var head strings.Builder
	if s.CustomCSS != "" {
		head.WriteString(`<style id="skin-custom-css">` + "\n" + s.CustomCSS + "\n</style>\n")
	}
	if s.CustomHead != "" {
		head.WriteString(s.CustomHead + "\n")
	}
	indexHTML = strings.Replace(indexHTML, "<!--skin-custom-head-->", head.String(), 1)

	// custom body: JS before </body>
	var body strings.Builder
	if s.CustomJS != "" {
		body.WriteString(`<script id="skin-custom-js">` + "\n" + s.CustomJS + "\n</script>\n")
	}
	return strings.Replace(indexHTML, "<!--skin-custom-body-->", body.String(), 1)
}
