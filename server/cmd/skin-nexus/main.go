// Command skin-nexus is the SkinNexus server.
//
// Routes:
//   - /api/yggdrasil/*        Yggdrasil API (legacy-compatible surface)
//   - /api/v1/*               SPA backend (webapi)
//   - /api/public/v1/*        public read-only API (S8)
//   - /textures/{hash}        texture file serving (image/png, immutable)
//   - /skin/{player}.png etc. legacy-api compatibility (S8)
//   - /*                      SPA static assets (embedded later; falls back to web/dist)
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"skin-nexus/server/internal/adminapi"
	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/config"
	"skin-nexus/server/internal/migrate"
	"skin-nexus/server/internal/publicapi"
	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/texture"
	"skin-nexus/server/internal/webapi"
	"skin-nexus/server/internal/ygg"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("skin-nexus", version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := migrate.RunCLI(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Legacy verifier: built from optional legacy .env (SKIN_LEGACY_ENV).
	var legacy *auth.LegacyVerifier
	if envPath := os.Getenv("SKIN_LEGACY_ENV"); envPath != "" {
		lenv, err := migrate.ParseLegacyEnv(envPath)
		if err != nil {
			log.Fatalf("legacy env: %v", err)
		}
		legacy = auth.NewLegacyVerifier(lenv.MethodSalts())
		log.Printf("legacy verifier: %d methods loaded", len(lenv.PWDMethods))
	}

	st, err := store.Open(cfg.DatabaseDSN, cfg.BaseURL, legacy)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	ts, err := texture.NewStore(cfg.TexturesDir)
	if err != nil {
		log.Fatalf("textures: %v", err)
	}

	signer, err := ygg.NewLocalSigner(cfg.SignKeyPath)
	if err != nil {
		log.Fatalf("signer: %v", err)
	}
	log.Printf("signing key fingerprint: %s", signer.KeyFingerprint())

	yggSvc := ygg.NewService(st, signer)
	yggAPI := ygg.NewAPI(yggSvc, ygg.MetadataConfig{
		ServerName:  cfg.ServerName,
		Version:     version,
		Homepage:    cfg.BaseURL,
		Register:    cfg.BaseURL + "/auth/register",
		SkinDomains: cfg.SkinDomains,
	})
	backend := webapi.New(st, ts, yggSvc)
	pub := publicapi.New(st, ts)
	admin := adminapi.New(st, yggSvc)

	root := http.NewServeMux()
	root.Handle("/api/yggdrasil/", yggAPI.Handler())
	root.Handle("/api/v1/", backend.Handler())
	root.Handle("/api/admin/v1/", admin.Handler())
	root.Handle("/api/public/", pub.Handler())
	root.Handle("/usm/", pub.Handler())
	root.Handle("/skin/", pub.Handler())
	root.Handle("/cape/", pub.Handler())
	root.Handle("GET /{player}.json", pub.CustomSkinAPIHandler())
	root.Handle("GET /textures/{hash}", textureHandler(ts))
	root.Handle("/", spaHandler())

	addr := cfg.Listen
	log.Printf("skin-nexus %s listening on %s (base %s)", version, addr, cfg.BaseURL)
	if err := http.ListenAndServe(addr, root); err != nil {
		log.Fatal(err)
	}
}

// textureHandler serves stored textures byte-exact with strict hash validation.
func zeroTime() time.Time { return time.Time{} }

func textureHandler(ts *texture.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := ts.Open(r.PathValue("hash"))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer f.Close()
		// Content-addressed: immutable cache is safe (hash = filename).
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, "", zeroTime(), f)
	}
}

// spaHandler serves web/dist if present, else a placeholder page.
func spaHandler() http.Handler {
	dir := "web/dist"
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		fs := http.FileServer(http.Dir(dir))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: unknown paths -> index.html
			p := strings.TrimPrefix(r.URL.Path, "/")
			if _, err := os.Stat(dir + "/" + p); err != nil && !strings.HasPrefix(p, "api/") {
				http.ServeFile(w, r, dir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<h1>SkinNexus</h1><p>web/dist not built yet. See docs/.</p>"))
	})
}
