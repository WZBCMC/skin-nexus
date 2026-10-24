package announce

import (
	"strings"
	"testing"
)

// XSS corpus: every vector must come out inert. These assert the SERVER
// layer (bluemonday); the client layer (marked+DOMPurify) is defense #2.
func TestRenderStripsScriptVectors(t *testing.T) {
	cases := []struct{ name, in string }{
		{"script tag", "<script>alert(1)</script>"},
		{"inline handler", `<img src=x onerror="alert(1)">`},
		{"javascript link", "[x](javascript:alert(1))"},
		{"data uri link", "[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)"},
		{"iframe", `<iframe src="https://evil.example"></iframe>`},
		{"style expr", `<div style="background:url(javascript:alert(1))">x</div>`},
		{"svg onload", `<svg onload="alert(1)"></svg>`},
		{"form action", `<form action="javascript:alert(1)"><input type=submit></form>`},
	}
	for _, c := range cases {
		out := Render(c.in)
		for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe", "onload", "<form"} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Errorf("%s: output still contains %q: %s", c.name, bad, out)
			}
		}
	}
}

// Legitimate Markdown must survive the pipeline.
func TestRenderKeepsLegitMarkdown(t *testing.T) {
	out := Render("# 标题\n\n**粗体** 与 [链接](https://example.com)\n\n- 项目一\n- 项目二")
	for _, want := range []string{"<h1", "<strong", "https://example.com", "<li", "nofollow"} {
		if !strings.Contains(out, want) {
			t.Errorf("legit markdown lost %q: %s", want, out)
		}
	}
}

func TestRenderEmptyAndFailureDegrade(t *testing.T) {
	if got := Render(""); got != "" {
		t.Errorf("empty input must render empty, got %q", got)
	}
}
