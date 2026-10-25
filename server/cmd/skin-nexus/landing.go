package main

// landing.go: the public, server-rendered landing page served at "/".
//
// Rationale (2026-10-25 拍板): the SPA login card is fine for returning players
// but a dead end for first-time visitors, crawlers and AI agents. The landing
// page is rendered by Go templates straight into the response body — readable
// without JavaScript — and reuses the SEO + appearance injection layers so the
// theme / custom CSS apply (the page consumes the same design tokens as the
// app, with fallbacks).

import (
	"context"
	"html/template"
	"strings"
	"time"

	"skin-nexus/server/internal/appearance"
	"skin-nexus/server/internal/seo"
)

// landingTexture is one featured library card (preview rendered server-side).
type landingTexture struct {
	TID  int64
	Name string
}

// landingData feeds the landing template.
type landingData struct {
	SiteName string
	Intro    string
	Users    int
	Players  int
	Textures int
	Featured []landingTexture
	Year     int
}

const landingIntro = "Minecraft 皮肤站：上传皮肤/披风、管理角色衣柜、逛皮肤库，配合 Yggdrasil 外置登录在启动器与服务器之间无缝换装。"

var landingTmpl = template.Must(template.New("landing").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<!--skin-seo-->
<!--skin-theme-->
<!--skin-custom-head-->
<style>
  :root { color-scheme: dark; }
  body { margin: 0; font-family: system-ui, -apple-serif, "Segoe UI", sans-serif;
         background: var(--bg, #0f1115); color: var(--text, #d7dae0); }
  .wrap { max-width: 880px; margin: 0 auto; padding: 24px 16px 48px; }
  .hero { padding: 56px 0 32px; text-align: center; }
  .hero h1 { margin: 0 0 8px; font-size: 40px; letter-spacing: 1px; }
  .hero .tag { color: var(--muted, #8b93a1); font-size: 16px; margin: 0 0 24px; }
  .cta a { display: inline-block; margin: 0 8px; padding: 11px 28px; border-radius: 8px;
           font-size: 15px; text-decoration: none; font-weight: 600; }
  .cta .primary { background: var(--accent, #4f8cff); color: #fff; }
  .cta .alt { border: 1px solid var(--border, #262b36); color: var(--text, #d7dae0); }
  .stats { display: flex; gap: 12px; margin: 28px 0; }
  .stats div { flex: 1; text-align: center; padding: 18px 8px; border-radius: 10px;
               background: var(--panel, #171a21); border: 1px solid var(--border, #262b36); }
  .stats b { display: block; font-size: 26px; color: var(--accent, #4f8cff); }
  .stats span { font-size: 13px; color: var(--muted, #8b93a1); }
  .intro { line-height: 1.8; color: var(--text, #d7dae0); margin: 24px 0; }
  .feat h2 { font-size: 20px; margin: 32px 0 16px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 12px; }
  .grid figure { margin: 0; padding: 10px; border-radius: 10px; text-align: center;
                 background: var(--panel, #171a21); border: 1px solid var(--border, #262b36); }
  .grid img { width: 96px; height: 96px; image-rendering: pixelated; }
  .grid figcaption { font-size: 12px; color: var(--muted, #8b93a1); margin-top: 6px;
                     overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  footer { margin-top: 40px; text-align: center; font-size: 12px; color: var(--muted, #8b93a1); }
  footer a { color: var(--accent, #4f8cff); text-decoration: none; }
  @media (max-width: 560px) { .hero h1 { font-size: 30px; } .stats { flex-wrap: wrap; } }
</style>
</head>
<body>
  <div class="wrap">
    <header class="hero">
      <h1>{{.SiteName}}</h1>
      <p class="tag">{{.SiteName}} — Minecraft 皮肤站 · Yggdrasil 外置登录</p>
      <div class="cta">
        <a class="primary" href="/register">注册账号</a>
        <a class="alt" href="/login">登录</a>
      </div>
    </header>
    <section class="stats">
      <div><b>{{.Users}}</b><span>注册用户</span></div>
      <div><b>{{.Players}}</b><span>游戏角色</span></div>
      <div><b>{{.Textures}}</b><span>皮肤材质</span></div>
    </section>
    <section class="intro"><p>{{.Intro}}</p></section>
    {{if .Featured}}
    <section class="feat">
      <h2>皮肤库精选</h2>
      <div class="grid">
        {{range .Featured}}
        <figure><img src="/preview/{{.TID}}" alt="{{.Name}}" loading="lazy" /><figcaption>{{.Name}}</figcaption></figure>
        {{end}}
      </div>
    </section>
    {{end}}
    <footer>
      Powered by <a href="https://github.com/YimingZhanshen/skin-nexus">SkinNexus</a>
      · © {{.Year}} {{.SiteName}}
    </footer>
  </div>
  <!--skin-custom-body-->
</body>
</html>
`))

// pageMetaFor derives per-page SEO overrides from the SPA path — player
// profile pages get name-bearing titles so sitemap entries are not identical.
func pageMetaFor(p string) seo.PageMeta {
	if name, ok := strings.CutPrefix(p, "profile/"); ok {
		name = strings.Trim(name, "/")
		if name != "" {
			return seo.PageMeta{TitleSuffix: name, Path: "/profile/" + name}
		}
	}
	return seo.PageMeta{}
}

// renderLanding renders the landing page through the same SEO + appearance
// injection layers as the SPA shell.
func renderLanding(ctx context.Context, seoEng *seo.Engine, appearEng *appearance.Engine, d landingData) string {
	d.Year = time.Now().Year()
	d.Intro = landingIntro
	var sb strings.Builder
	if err := landingTmpl.Execute(&sb, d); err != nil {
		return "<!DOCTYPE html><html><body><h1>landing render error</h1></body></html>"
	}
	html := seoEng.RenderHeadWith(ctx, sb.String(), seo.PageMeta{Description: landingIntro})
	return appearEng.Render(ctx, html)
}
