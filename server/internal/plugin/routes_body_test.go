package plugin

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const binManifest = `{
  "name": "bin",
  "version": "1.0.0",
  "hookApi": 2,
  "entrypoint": "index.js",
  "permissions": ["http:routes"],
  "routes": [
    {"method": "POST", "path": "/bin", "tier": "public", "bodyLimit": 4194304, "bodyEncoding": "base64", "timeoutMS": 10000},
    {"method": "POST", "path": "/json", "tier": "public", "bodyLimit": 1024}
  ]
}`

const binScript = `
skin.route("POST", "/bin", function (r) { return {body: r.body, enc: r.bodyEncoding}; });
skin.route("POST", "/json", function (r) { return {body: r.body}; });
`

func newBinHost(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	writePlugin(t, root, "bin", binManifest, binScript)
	m := NewHost(RuntimeConfig{Dir: root, HostVersion: "0.4.0"})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "bin"); err != nil {
		t.Fatal(err)
	}
	return m
}

func binReq(m *Manager, path, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/plugins/bin"+path, strings.NewReader(string(body)))
	r.Host = "skin.test"
	r.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, r)
	return rec
}

func TestBase64RouteBody(t *testing.T) {
	m := newBinHost(t)
	raw := []byte{0x00, 0xff, 0x10, 'a', 'b', 'c'}
	rec := binReq(m, "/bin", "image/png", raw)
	if rec.Code != 200 {
		t.Fatalf("bin = %d %s", rec.Code, rec.Body.String())
	}
	want := base64.StdEncoding.EncodeToString(raw)
	if !strings.Contains(rec.Body.String(), `"body":"`+want+`"`) || !strings.Contains(rec.Body.String(), `"enc":"base64"`) {
		t.Fatalf("bin body = %s (want %s)", rec.Body.String(), want)
	}

	rec = binReq(m, "/bin", "application/json", []byte(`{"x":1}`))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"body":{"x":1}`) {
		t.Fatalf("json on base64 route = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRouteBodyLimits(t *testing.T) {
	m := newBinHost(t)
	rec := binReq(m, "/json", "text/plain", []byte(strings.Repeat("x", 2048)))
	if rec.Code != 413 {
		t.Fatalf("json oversize = %d %s", rec.Code, rec.Body.String())
	}
	rec = binReq(m, "/bin", "image/png", make([]byte, 4<<20+1))
	if rec.Code != 413 {
		t.Fatalf("binary oversize = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRouteTimeoutClamp(t *testing.T) {
	def := time.Second
	if d := routeTimeout(RouteSpec{}, def); d != def {
		t.Fatalf("default = %s", d)
	}
	if d := routeTimeout(RouteSpec{TimeoutMS: 50}, def); d != 100*time.Millisecond {
		t.Fatalf("clamp low = %s", d)
	}
	if d := routeTimeout(RouteSpec{TimeoutMS: 20000}, def); d != 10*time.Second {
		t.Fatalf("clamp high = %s", d)
	}
}

func TestBodyEncodingManifestValidation(t *testing.T) {
	cases := []string{
		`{"name":"bad","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["http:routes"],"routes":[{"method":"POST","path":"/x","bodyEncoding":"gzip"}]}`,
		`{"name":"bad","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["http:routes"],"routes":[{"method":"POST","path":"/x","timeoutMS":20}]}`,
		`{"name":"bad","version":"1","hookApi":2,"entrypoint":"index.js","permissions":["http:routes"],"routes":[{"method":"POST","path":"/x","bodyLimit":2097152}]}`,
	}
	for _, manifest := range cases {
		root := t.TempDir()
		dir := writePlugin(t, root, "bad", manifest, "0;")
		if _, err := loadManifest(dir, "bad"); err == nil {
			t.Fatalf("manifest should be rejected: %s", manifest)
		}
	}
}
