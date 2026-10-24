// Package announce renders the site announcement (option `announcement`,
// blessing parity: Markdown source) into sanitized HTML.
//
// 拍板（2026-10-24 dev-plan §9）: 双层消毒 —— 服务端 goldmark 渲染 +
// bluemonday 白名单消毒，前端 marked + DOMPurify 再消毒一层后才 v-html。
// 任何一层被绕过，另一层仍拦截 XSS。
package announce

import (
	"bytes"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// md passes raw HTML through (goldmark default escapes it) — bluemonday
// below is the authority on what survives; escaping here would only mangle
// legitimate <br>/<details> that the policy then allows.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// policy: UGCPolicy — links/images/formatting allowed, scripts/event
// handlers/styles/iframes stripped. Links get rel="nofollow noopener".
var policy = bluemonday.UGCPolicy()

// Render converts announcement Markdown to sanitized HTML. Empty input
// returns empty string (frontend hides the card). Render failures degrade
// to empty rather than echo unsanitized input.
func Render(src string) string {
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return policy.Sanitize(buf.String())
}
