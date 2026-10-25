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

// landingTexture is one featured library card. URL follows the skinlib UI
// rule (SkinlibPage.vue): skins get the server-rendered body preview, capes
// get the flat texture image (/preview of a cape through the body renderer
// is garbage, hence 404 there).
type landingTexture struct {
	TID  int64
	Name string
	URL  string
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
<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
<link rel="icon" type="image/x-icon" href="/favicon.ico" sizes="48x48" />
<link rel="apple-touch-icon" href="/apple-touch-icon.png" />
<link rel="manifest" href="/manifest.webmanifest" />
<meta name="theme-color" content="#2f66ff" />
{{.HeadAnchors}}
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
  .grid img { max-width: 96px; max-height: 96px; image-rendering: pixelated; }
  .grid figcaption { font-size: 12px; color: var(--muted, #8b93a1); margin-top: 6px;
                     overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  footer { margin-top: 40px; text-align: center; font-size: 12px; color: var(--muted, #8b93a1); }
  footer a { color: var(--accent, #4f8cff); text-decoration: none; }
  /* 品牌标识入场编舞：1600ms 共此时钟，offset 为关键帧内百分比；
     keyframe 缓动一律字面值（var() 在 @keyframes 内会被 Chromium 静默丢弃）。 */
  .brandmark { width: 112px; height: 112px; margin: 0 auto 18px; }
  .brandmark svg { width: 100%; height: 100%; display: block; }
  #p2m-stage, #p2m-plate, #p2m-wipe, #p2m-node-1, #p2m-node-2, #p2m-node-3 { transform-box: fill-box; }
  #p2m-stage, #p2m-plate, #p2m-node-1, #p2m-node-2, #p2m-node-3 { transform-origin: center; }
  #p2m-wipe { transform-origin: left center; }
  #p2m-plate { animation: p2m-plate 1600ms both; }
  @keyframes p2m-plate {
    0% { opacity: 0; transform: scale(0.96); animation-timing-function: cubic-bezier(0.22, 1, 0.36, 1); }
    18% { opacity: 1; transform: scale(1); }
    100% { opacity: 1; transform: scale(1); }
  }
  #p2m-wipe { animation: p2m-wipe 1600ms both; }
  @keyframes p2m-wipe {
    0% { transform: scaleX(0); }
    15% { transform: scaleX(0); animation-timing-function: cubic-bezier(0.65, 0, 0.35, 1); }
    55% { transform: scaleX(1); }
    100% { transform: scaleX(1); }
  }
  #p2m-connector { stroke-dasharray: 1; stroke-dashoffset: 1; animation: p2m-draw 1600ms both; }
  @keyframes p2m-draw {
    0% { stroke-dashoffset: 1; }
    45% { stroke-dashoffset: 1; animation-timing-function: cubic-bezier(0.65, 0, 0.35, 1); }
    70% { stroke-dashoffset: 0; }
    100% { stroke-dashoffset: 0; }
  }
  #p2m-node-1 { animation: p2m-pop1 1600ms both; }
  #p2m-node-2 { animation: p2m-pop2 1600ms both; }
  #p2m-node-3 { animation: p2m-pop3 1600ms both; }
  @keyframes p2m-pop1 {
    0% { transform: scale(0); }
    58% { transform: scale(0); animation-timing-function: cubic-bezier(0.34, 1.56, 0.64, 1); }
    72% { transform: scale(1); }
    100% { transform: scale(1); }
  }
  @keyframes p2m-pop2 {
    0% { transform: scale(0); }
    64% { transform: scale(0); animation-timing-function: cubic-bezier(0.34, 1.56, 0.64, 1); }
    78% { transform: scale(1); }
    100% { transform: scale(1); }
  }
  @keyframes p2m-pop3 {
    0% { transform: scale(0); }
    70% { transform: scale(0); animation-timing-function: cubic-bezier(0.34, 1.56, 0.64, 1); }
    84% { transform: scale(1); }
    100% { transform: scale(1); }
  }
  #p2m-stage { animation: p2m-stage 1600ms both; }
  @keyframes p2m-stage {
    0% { transform: scale(1); }
    78% { transform: scale(1); animation-timing-function: cubic-bezier(0.22, 1, 0.36, 1); }
    84% { transform: scale(1.03); animation-timing-function: cubic-bezier(0.16, 1, 0.3, 1); }
    100% { transform: scale(1); }
  }
  @media (prefers-reduced-motion: reduce) {
    #p2m-stage, #p2m-plate, #p2m-wipe, #p2m-connector, #p2m-node-1, #p2m-node-2, #p2m-node-3 {
      animation: none !important;
      opacity: 1 !important;
      transform: none !important;
      stroke-dashoffset: 0 !important;
    }
  }
  @media (max-width: 560px) { .hero h1 { font-size: 30px; } .stats { flex-wrap: wrap; } .brandmark { width: 88px; height: 88px; margin-bottom: 14px; } }
</style>
</head>
<body>
  <div class="wrap">
    <header class="hero">
      <div class="brandmark" aria-hidden="true">
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <defs>
    <linearGradient id="p2m-g-plate" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#ffffff"/>
      <stop offset="1" stop-color="#eef3fb"/>
    </linearGradient>
    <linearGradient id="p2m-g-n" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#2f66ff"/>
      <stop offset="0.55" stop-color="#2a8fe0"/>
      <stop offset="1" stop-color="#20b983"/>
    </linearGradient>
    <filter id="p2m-soft" x="-20%" y="-20%" width="140%" height="140%">
      <feDropShadow dx="0" dy="10" stdDeviation="18" flood-color="#2f66ff" flood-opacity="0.18"/>
    </filter>
    <mask id="p2m-wipe-mask">
      <rect id="p2m-wipe" x="96" y="76" width="420" height="380" fill="#ffffff"/>
    </mask>
  </defs>
  <g id="p2m-stage">
    <rect id="p2m-plate" x="36" y="36" width="440" height="440" rx="96"
          fill="url(#p2m-g-plate)" stroke="#d3def8" stroke-width="2"/>
    <g id="p2m-mark" mask="url(#p2m-wipe-mask)" filter="url(#p2m-soft)">
      <path d="M 128 400 V 112 h 80 l 98 150 V 112 h 80 v 288 h -80 l -98 -150 v 150 z" fill="url(#p2m-g-n)"/>
      <rect x="128" y="112" width="80" height="80" fill="#ffffff" opacity="0.12"/>
      <rect x="306" y="320" width="80" height="80" fill="#ffffff" opacity="0.10"/>
    </g>
    <path id="p2m-connector" d="M 168 248 V 178 L 282 262 H 346 V 330"
          stroke="#ffffff" stroke-opacity="0.55" stroke-width="7" fill="none"
          stroke-linecap="butt" pathLength="1"/>
    <rect id="p2m-node-1" x="152" y="150" width="32" height="32" rx="6" fill="#ffffff" opacity="0.92"/>
    <rect id="p2m-node-2" x="266" y="246" width="32" height="32" rx="6" fill="#ffffff" opacity="0.92"/>
    <rect id="p2m-node-3" x="330" y="330" width="32" height="32" rx="6" fill="#eafff6" opacity="0.92"/>
  </g>
</svg>
      </div>
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
        <figure><img src="{{.URL}}" alt="{{.Name}}" loading="lazy" /><figcaption>{{.Name}}</figcaption></figure>
        {{end}}
      </div>
    </section>
    {{end}}
  </div>
  <footer>
    Powered by <a href="https://github.com/YimingZhanshen/skin-nexus">SkinNexus</a>
    · © {{.Year}} {{.SiteName}}
  </footer>
  {{.BodyAnchor}}
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

// Injection anchors are passed as trusted template.HTML data: html/template
// strips HTML comments written in template source, which silently deleted the
// <!--skin-*--> markers and left the SEO/theme/custom blocks unanchored
// (2026-10-25 production regression, reproduced in tests).
const (
	landingHeadAnchors = "<!--skin-seo-->\n<!--skin-theme-->\n<!--skin-custom-head-->"
	landingBodyAnchor  = "<!--skin-custom-body-->"
)

type landingView struct {
	landingData
	HeadAnchors template.HTML
	BodyAnchor  template.HTML
}

// renderLanding renders the landing page through the same SEO + appearance
// injection layers as the SPA shell.
func renderLanding(ctx context.Context, seoEng *seo.Engine, appearEng *appearance.Engine, d landingData) string {
	d.Year = time.Now().Year()
	d.Intro = landingIntro
	var sb strings.Builder
	v := landingView{
		landingData: d,
		HeadAnchors: template.HTML(landingHeadAnchors),
		BodyAnchor:  template.HTML(landingBodyAnchor),
	}
	if err := landingTmpl.Execute(&sb, v); err != nil {
		return "<!DOCTYPE html><html><body><h1>landing render error</h1></body></html>"
	}
	html := seoEng.RenderHeadWith(ctx, sb.String(), seo.PageMeta{Description: landingIntro})
	return appearEng.Render(ctx, html)
}
