package main

// texturepage.go: the public, server-rendered texture detail page at
// /skinlib/{tid} (2026-10-25 拍板: 皮肤详情取代玩家档案页成为搜索引擎的
// 内容页).
//
// Why server-rendered (not the SPA shell): crawlers must see real content —
// name, uploader, likes, the preview image, JSON-LD — in the response body,
// not an empty <div id="app">. The page consumes the same SEO + appearance
// injection layers as the landing page, so themes and custom CSS apply.
// Interactivity is progressive enhancement: a small fixed-name module
// (assets/viewer.js) upgrades the static server-rendered preview <img> into
// an auto-rotating skinview3d canvas and wires the like toggle; without JS
// the page is still a complete, crawlable, downloadable skin page.

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"skin-nexus/server/internal/appearance"
	"skin-nexus/server/internal/seo"
	"skin-nexus/server/internal/store"
)

// texDetailData feeds the detail template.
type texDetailData struct {
	SiteName  string
	TID       int64
	Name      string
	Type      string // "skin" | "cape"
	Model     string // "default" | "slim"
	Nickname  string
	Likes     int64
	Liked     bool
	Public    bool
	UploadAt  string // "2006-01-02"-style display date, "" when unknown
	TextureURL string // absolute, for download + JSON-LD
	PreviewURL string // relative preview image (skin: /preview/{tid}; cape: flat)
	PreviewAbsURL string // absolute preview URL (og:image + JSON-LD)
	Year      int

	HeadAnchors template.HTML
	BodyAnchor  template.HTML
}

var texDetailTmpl = template.Must(template.New("texdetail").Parse(`<!DOCTYPE html>
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
  .wrap { max-width: 760px; margin: 0 auto; padding: 24px 16px 48px; }
  .crumb { font-size: 13px; color: var(--muted, #8b93a1); margin: 8px 0 18px; }
  .crumb a { color: var(--accent, #4f8cff); text-decoration: none; }
  .card { background: var(--panel, #171a21); border: 1px solid var(--border, #262b36);
          border-radius: 12px; padding: 24px; }
  h1 { margin: 0 0 4px; font-size: 26px; word-break: break-all; }
  .sub { color: var(--muted, #8b93a1); font-size: 14px; margin: 0 0 20px; }
  .viewer { display: flex; justify-content: center; align-items: center;
            min-height: 300px; margin: 0 0 20px; position: relative; }
  .viewer img.static { max-height: 280px; image-rendering: pixelated; }
  .viewer canvas { display: none; }
  .viewer.ready img.static { display: none; }
  .viewer.ready canvas { display: block; }
  .meta { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
          gap: 10px; margin: 0 0 20px; }
  .meta div { background: rgba(255,255,255,0.03); border: 1px solid var(--border, #262b36);
              border-radius: 8px; padding: 10px 12px; }
  .meta dt { font-size: 12px; color: var(--muted, #8b93a1); margin: 0 0 2px; }
  .meta dd { margin: 0; font-size: 14px; font-weight: 600; word-break: break-all; }
  .actions { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
  .actions a, .actions button { display: inline-block; padding: 10px 22px; border-radius: 8px;
        font-size: 14px; font-weight: 600; text-decoration: none; cursor: pointer; }
  .actions .primary { background: var(--accent, #4f8cff); color: #fff; border: none; }
  .actions .alt { border: 1px solid var(--border, #262b36); color: var(--text, #d7dae0);
                  background: transparent; }
  .like-btn.liked { color: #f66; border-color: #f66; }
  footer { margin-top: 32px; text-align: center; font-size: 12px; color: var(--muted, #8b93a1); }
  footer a { color: var(--accent, #4f8cff); text-decoration: none; }
  @media (max-width: 560px) { h1 { font-size: 21px; } .viewer { min-height: 240px; } }
</style>
</head>
<body>
  <div class="wrap">
    <nav class="crumb"><a href="/">首页</a> › <a href="/user/skinlib">皮肤库</a> › {{.Name}}</nav>
    <article class="card" itemscope itemtype="https://schema.org/CreativeWork">
      <h1 itemprop="name">{{.Name}}</h1>
      <p class="sub">Minecraft {{if eq .Type "cape"}}披风{{else}}皮肤{{end}} · 上传者 <span itemprop="author">{{.Nickname}}</span></p>
      <div class="viewer" id="tex-viewer">
        <img class="static" src="{{.PreviewURL}}" alt="{{.Name}} 的 Minecraft {{if eq .Type "cape"}}披风{{else}}皮肤{{end}} 预览图" itemprop="image" />
        {{if eq .Type "skin"}}<canvas data-skinview3d data-texture="{{.TextureURL}}" data-model="{{.Model}}" width="320" height="420"></canvas>{{end}}
      </div>
      <dl class="meta">
        <div><dt>类型</dt><dd>{{if eq .Type "cape"}}披风{{else if eq .Model "slim"}}皮肤（Alex 瘦臂）{{else}}皮肤（Steve 常规）{{end}}</dd></div>
        <div><dt>材质 ID</dt><dd>#{{.TID}}</dd></div>
        <div><dt>喜欢</dt><dd><span id="like-count">{{.Likes}}</span> 人</dd></div>
        {{if .UploadAt}}<div><dt>上传时间</dt><dd>{{.UploadAt}}</dd></div>{{end}}
        <div><dt>可见性</dt><dd>{{if .Public}}公开{{else}}私密{{end}}</dd></div>
      </dl>
      <div class="actions">
        <button class="alt like-btn{{if .Liked}} liked{{end}}" data-like="{{.TID}}" data-liked="{{if .Liked}}1{{else}}0{{end}}">♥ <span>喜欢 / 加入衣柜</span></button>
        <a class="primary" href="{{.TextureURL}}" download>下载纹理 PNG</a>
        <a class="alt" href="/register">注册使用同款</a>
      </div>
    </article>
  </div>
  <footer>
    Powered by <a href="https://github.com/YimingZhanshen/skin-nexus">SkinNexus</a>
    · © {{.Year}} {{.SiteName}}
  </footer>
  <script type="module" src="/assets/viewer.js?v=1"></script>
  {{.BodyAnchor}}
</body>
</html>
`))

// texNotFoundTmpl is the minimal branded 404 for missing/private textures —
// a plain http.NotFound would drop the theme and look like a server fault.
var texNotFoundTmpl = template.Must(template.New("tex404").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>材质不存在 - {{.SiteName}}</title>
<meta name="robots" content="noindex, nofollow" />
<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
<style>
  :root { color-scheme: dark; }
  body { margin: 0; font-family: system-ui, sans-serif; background: var(--bg, #0f1115);
         color: var(--text, #d7dae0); display: grid; place-items: center; min-height: 100vh; }
  .box { text-align: center; }
  a { color: var(--accent, #4f8cff); }
</style>
</head>
<body><div class="box"><h1>404</h1><p>材质不存在，或已被设为私密。</p><p><a href="/">返回首页</a></p></div></body>
</html>
`))

// texJSONLD builds the schema.org block for one public texture. All values
// pass through json.Marshal (which escapes HTML metacharacters) and the
// injection layer additionally neutralizes "</".
func texJSONLD(d texDetailData, canonical string) string {
	obj := map[string]any{
		"@context": "https://schema.org",
		"@type":    "CreativeWork",
		"name":     d.Name,
		"image":    d.PreviewAbsURL,
		"url":      canonical,
		"author":   map[string]any{"@type": "Person", "name": d.Nickname},
		"genre":    "Minecraft texture",
		"interactionStatistic": map[string]any{
			"@type":           "InteractionCounter",
			"interactionType": "https://schema.org/LikeAction",
			"userInteractionCount": d.Likes,
		},
	}
	if d.UploadAt != "" {
		obj["datePublished"] = d.UploadAt
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return string(b)
}

// texturePageHandler serves GET /skinlib/{tid}. Visibility: public textures
// render for everyone; private textures render only for their owner (or an
// admin) and carry noindex; anything else 404s with a branded page.
func texturePageHandler(st *store.Mysql, seoEng *seo.Engine, appearEng *appearance.Engine, siteName, baseURL string) http.HandlerFunc {
	base := strings.TrimRight(baseURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		tid, err := strconv.ParseInt(r.PathValue("tid"), 10, 64)
		if err != nil || tid <= 0 {
			writeTexNotFound(w, siteName)
			return
		}
		// viewer identity: session cookie → owner/admin visibility
		var uid int64
		var admin bool
		if c, err := r.Cookie("skin_session"); err == nil {
			if id, err := st.SessionUser(r.Context(), c.Value); err == nil {
				uid = id
				if perm, err := st.UserPermission(r.Context(), id); err == nil && perm > 0 {
					admin = true
				}
			}
		}
		row, err := st.SkinlibOne(r.Context(), tid, uid, admin)
		if err == store.ErrNotFound {
			writeTexNotFound(w, siteName)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		d := texDetailData{
			SiteName: siteName,
			TID:      row.TID,
			Name:     row.Name,
			Type:     row.Type,
			Model:    row.Model,
			Nickname: row.Nickname,
			Likes:    row.Likes,
			Liked:    row.Liked,
			Public:   row.Public,
		}
		if d.Name == "" {
			d.Name = "材质 #" + strconv.FormatInt(row.TID, 10)
		}
		if row.UploadAt.Valid {
			d.UploadAt = row.UploadAt.Time.UTC().Format("2006-01-02")
		}
		// Same split as the skinlib UI / landing cards: skins get the
		// server-rendered body preview, capes the flat texture (a cape
		// through the body renderer is garbage).
		if row.Type == "skin" {
			d.PreviewURL = "/preview/" + strconv.FormatInt(row.TID, 10)
		} else {
			d.PreviewURL = "/textures/" + row.Hash
		}
		d.TextureURL = st.TextureURL(row.Hash)
		d.PreviewAbsURL = absURL(base, d.PreviewURL)

		canonical := base + "/skinlib/" + strconv.FormatInt(row.TID, 10)
		pm := seo.PageMeta{
			TitleSuffix: d.Name + " 的 Minecraft " + texKindLabel(row.Type),
			Description: texDescription(d),
			Path:        "/skinlib/" + strconv.FormatInt(row.TID, 10),
			OGImage:     d.PreviewAbsURL,
			JSONLD:      texJSONLD(d, canonical),
		}
		// Private content viewed by its owner must never be indexed.
		pm.NoIndex = !row.Public

		var sb strings.Builder
		d.Year = time.Now().Year()
		d.HeadAnchors = template.HTML(landingHeadAnchors)
		d.BodyAnchor = template.HTML(landingBodyAnchor)
		if err := texDetailTmpl.Execute(&sb, d); err != nil {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		html := seoEng.RenderHeadWith(r.Context(), sb.String(), pm)
		html = appearEng.Render(r.Context(), html)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}
}

// absURL joins the site base with a relative, possibly query-bearing path.
func absURL(base, rel string) string {
	if strings.HasPrefix(rel, "http://") || strings.HasPrefix(rel, "https://") {
		return rel
	}
	return strings.TrimRight(base, "/") + rel
}

func texKindLabel(t string) string {
	if t == "cape" {
		return "披风"
	}
	return "皮肤"
}

func texDescription(d texDetailData) string {
	kind := texKindLabel(d.Type)
	model := ""
	if d.Type == "skin" {
		if d.Model == "slim" {
			model = "，Alex 瘦臂模型"
		} else {
			model = "，Steve 常规模型"
		}
	}
	return d.Name + " — " + d.SiteName + "皮肤库中的 Minecraft " + kind + model +
		"，由 " + d.Nickname + " 上传，" + strconv.FormatInt(d.Likes, 10) +
		" 人喜欢。注册账号即可一键使用。"
}

func writeTexNotFound(w http.ResponseWriter, siteName string) {
	var sb strings.Builder
	_ = texNotFoundTmpl.Execute(&sb, map[string]string{"SiteName": siteName})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(sb.String()))
}
