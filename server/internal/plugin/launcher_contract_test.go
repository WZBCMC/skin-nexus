package plugin

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func newLauncherHost(t *testing.T, fa *fakeAccount) *Manager {
	t.Helper()
	root := filepath.Join("testdata")
	m := NewHost(RuntimeConfig{
		Dir: root, HostVersion: "0.4.0",
		BearerResolver: fakeBearerResolver{}, Account: fa,
	})
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(context.Background(), "launcher-api"); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLauncherAPIContract(t *testing.T) {
	fa := &fakeAccount{}
	m := newLauncherHost(t, fa)

	rec := bearerReqPath(m, "GET", "/api/plugins/launcher-api/v1/meta", "", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"apiVersion":1`) || !strings.Contains(rec.Body.String(), `"libraryImport":true`) {
		t.Fatalf("meta = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReqPath(m, "GET", "/api/plugins/launcher-api/v1/account", "Bearer good", "", "")
	if rec.Code != 200 || fa.uid != 9 || !strings.Contains(rec.Body.String(), `"nickname":"launcher"`) {
		t.Fatalf("account = %d %s uid=%d", rec.Code, rec.Body.String(), fa.uid)
	}

	rec = bearerReqPath(m, "POST", "/api/plugins/launcher-api/v1/players", "Bearer good", "application/json", `{"name":"alex"}`)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"name":"alex"`) || fa.name != "alex" {
		t.Fatalf("create = %d %s name=%q", rec.Code, rec.Body.String(), fa.name)
	}

	raw := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	rec = bearerReqPath(m, "PUT", "/api/plugins/launcher-api/v1/players/1/skin?model=slim", "Bearer good", "image/png", string(raw))
	if rec.Code != 200 || string(fa.png) != string(raw) {
		t.Fatalf("skin upload = %d %s png=%v", rec.Code, rec.Body.String(), fa.png)
	}

	rec = bearerReqPath(m, "POST", "/api/plugins/launcher-api/v1/library/7/import", "Bearer good", "application/json", `{}`)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"textureId":7`) {
		t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
	}

	rec = bearerReqPath(m, "GET", "/api/plugins/launcher-api/v1/account", "", "", "")
	if rec.Code != 401 || rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("anonymous = %d %s", rec.Code, rec.Body.String())
	}
}

func TestLauncherAPIErrorMapping(t *testing.T) {
	fa := &fakeAccount{}
	m := newLauncherHost(t, fa)
	rec := bearerReqPath(m, "GET", "/api/plugins/launcher-api/v1/players/2", "Bearer good", "", "")
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "player.not_found") {
		t.Fatalf("missing player = %d %s", rec.Code, rec.Body.String())
	}
	rec = bearerReqPath(m, "POST", "/api/plugins/launcher-api/v1/textures", "Bearer good", "image/png", "not-a-png")
	if rec.Code != 201 {
		t.Fatalf("fake host should accept upload body: %d %s", rec.Code, rec.Body.String())
	}
}
