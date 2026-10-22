// Package publicapi serves the read-only external surface (requirement #3:
// 社团官网/服务器官网读取皮肤站数据) plus the three legacy-compat profile
// endpoints that existing callers already use:
//
//	CustomSkinAPI R1   GET /{player}.json   {username, skins:{default|slim:hash}, cape:hash}
//	legacy-api         GET /skin/{player}.png | /cape/{player}.png  (raw PNG)
//	usm-api            GET /usm/{player}.json
//
// Shapes follow analysis/blessing-skin-plugins.md §4. Everything here is
// read-only; write operations stay behind /api/v1 session auth.
package publicapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/texture"
	"skin-nexus/server/internal/ygg"
)

type Backend struct {
	Store    *store.Mysql
	Textures *texture.Store
}

func New(st *store.Mysql, ts *texture.Store) *Backend {
	return &Backend{Store: st, Textures: ts}
}

// Handler mounts public + legacy-compat routes (CORS open for官网 integration).
// Go 1.22+ ServeMux panics on wildcard segments with a literal suffix
// ("{player}.json"): a wildcard must end its segment. The suffix routes are
// therefore registered as prefix patterns and parsed in-handler by
// playerFromPath — URL semantics are unchanged.
func (b *Backend) Handler() http.Handler {
	mux := http.NewServeMux()
	// SkinNexus public API
	mux.HandleFunc("GET /api/public/v1/players/byname/{name}", b.handleByName)
	mux.HandleFunc("GET /api/public/v1/players/byuuid/{uuid}", b.handleByUUID)
	// Legacy compat (existing callers): /usm/{player}.json, /skin|cape/{player}.png
	mux.HandleFunc("GET /usm/", b.handleUSM)
	mux.HandleFunc("GET /skin/", b.handleLegacyPNG("skin"))
	mux.HandleFunc("GET /cape/", b.handleLegacyPNG("cape"))
	return cors(mux)
}

// WrapRoot serves CustomSkinAPI R1 at the site root (GET|HEAD /{player}.json)
// and delegates everything else to next. It wraps the top-level mux because
// ServeMux cannot express "{player}.json" alongside the "/" SPA catch-all.
func (b *Backend) WrapRoot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if _, ok := playerFromPath(r.URL.Path, "/", ".json"); ok {
				cors(http.HandlerFunc(b.handleCustomSkinAPI)).ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// playerFromPath extracts <player> from "<prefix><player><suffix>" URLs.
// Exactly one path segment counts as a match (legacy single-name surface).
func playerFromPath(path, prefix, suffix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := path[len(prefix):]
	if !strings.HasSuffix(rest, suffix) {
		return "", false
	}
	name := strings.TrimSuffix(rest, suffix)
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", false
	}
	return name, true
}

// CustomSkinAPIHandler serves GET|HEAD /{player}.json when mounted at root.
// Prefer WrapRoot for the top-level mux (it falls through to the SPA).
func (b *Backend) CustomSkinAPIHandler() http.Handler {
	return cors(http.HandlerFunc(b.handleCustomSkinAPI))
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

// resolve looks up a player by name (case-insensitive via store) and returns
// its texture hashes. Missing player -> (nil, false).
func (b *Backend) resolve(ctx context.Context, name string) (*ygg.Player, string, string, string, bool) {
	if b.Store == nil {
		return nil, "", "", "", false
	}
	p, err := b.Store.PlayerByName(ctx, name)
	if err != nil {
		return nil, "", "", "", false
	}
	skinHash, model, capeHash, err := b.Store.PlayerTextureHashes(ctx, p.ID)
	if err != nil {
		return nil, "", "", "", false
	}
	return &p, skinHash, model, capeHash, true
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- SkinNexus public API ----

func (b *Backend) handleByName(w http.ResponseWriter, r *http.Request) {
	p, skinHash, model, capeHash, ok := b.resolve(r.Context(), r.PathValue("name"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, b.profileJSON(p, skinHash, model, capeHash))
}

func (b *Backend) handleByUUID(w http.ResponseWriter, r *http.Request) {
	p, err := b.Store.PlayerByUUID(r.Context(), strings.ToLower(r.PathValue("uuid")))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	skinHash, model, capeHash, err := b.Store.PlayerTextureHashes(r.Context(), p.ID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, b.profileJSON(&p, skinHash, model, capeHash))
}

func (b *Backend) profileJSON(p *ygg.Player, skinHash, model, capeHash string) map[string]interface{} {
	out := map[string]interface{}{
		"id": p.UUID, "name": p.Name,
		"skins": map[string]string{},
	}
	skins := out["skins"].(map[string]string)
	if skinHash != "" {
		if model == "slim" {
			skins["slim"] = b.Store.TextureURL(skinHash)
		} else {
			skins["default"] = b.Store.TextureURL(skinHash)
		}
	}
	if capeHash != "" {
		out["cape"] = b.Store.TextureURL(capeHash)
	}
	return out
}

// ---- CustomSkinAPI R1 ----

func (b *Backend) handleCustomSkinAPI(w http.ResponseWriter, r *http.Request) {
	name, ok := playerFromPath(r.URL.Path, "/", ".json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p, skinHash, model, capeHash, ok := b.resolve(r.Context(), name)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	skins := map[string]string{}
	if skinHash != "" {
		if model == "slim" {
			skins["slim"] = skinHash
		} else {
			skins["default"] = skinHash
		}
	}
	resp := map[string]interface{}{
		"username": p.Name,
		"skins":    skins,
	}
	if capeHash != "" {
		resp["cape"] = capeHash
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- USM API ----

func (b *Backend) handleUSM(w http.ResponseWriter, r *http.Request) {
	name, ok := playerFromPath(r.URL.Path, "/usm/", ".json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	p, skinHash, model, capeHash, ok := b.resolve(r.Context(), name)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	modelPref := "default"
	if model == "slim" {
		modelPref = "slim"
	}
	skins := map[string]string{}
	if skinHash != "" {
		skins[modelPref] = skinHash
	}
	resp := map[string]interface{}{
		"player_name":      p.Name,
		"model_preference": []string{modelPref},
		"skins":            skins,
	}
	if capeHash != "" {
		resp["cape"] = capeHash
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- legacy-api raw PNG ----

func (b *Backend) handleLegacyPNG(kind string) http.HandlerFunc {
	prefix := "/skin/"
	if kind == "cape" {
		prefix = "/cape/"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := playerFromPath(r.URL.Path, prefix, ".png")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, skinHash, _, capeHash, ok := b.resolve(r.Context(), name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hash := skinHash
		if kind == "cape" {
			hash = capeHash
		}
		if hash == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f, err := b.Textures.Open(hash)
		if err != nil {
			// DB row exists but file missing (legacy anomaly) -> 403 parity
			w.WriteHeader(http.StatusForbidden)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeContent(w, r, "", time.Time{}, f)
	}
}
