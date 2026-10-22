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
	"context"
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
	"skin-nexus/server/internal/oidc"
	"skin-nexus/server/internal/publicapi"
	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/texture"
	"skin-nexus/server/internal/union"
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

	// Phase 2: MUA Union member module. Switchable — when disabled (default)
	// nothing is mounted and the texture signer stays local (zero surface).
	var activeSigner ygg.Signer = signer
	var unionAPI *union.API
	var unionGrant *union.Grantor
	var unionOut *union.Outbound
	if cfg.UnionEnabled {
		hostPub, err := os.ReadFile(cfg.UnionHostPublicKeyPath)
		if err != nil {
			log.Fatalf("union host pubkey: %v", err)
		}
		sigPEM, err := os.ReadFile(cfg.UnionSigPrivateKey)
		if err != nil {
			log.Fatalf("union oauth2 sig key: %v", err)
		}
		audit := func(event string, detail map[string]any) {
			log.Printf("union audit: %s %v", event, detail)
		}
		ucfg := union.Config{
			Enabled:               true,
			ApiRoot:               cfg.UnionAPIRoot,
			MemberKey:             cfg.UnionMemberKey,
			HostPublicKeyPEM:      string(hostPub),
			AllowUpdateBackendKey: cfg.UnionAllowUpdateBkKey,
			QueryEmailMode:        cfg.UnionQueryEmailMode,
			HubOrigin:             cfg.UnionHubOrigin,
			SigPrivateKeyPEM:      string(sigPEM),
			YggdrasilApiVersion:   version,
			UnionOAuth2:           true,
		}
		out, err := union.NewOutbound(ucfg, nil)
		if err != nil {
			log.Fatalf("union outbound: %v", err)
		}
		usigner := union.NewUnionSigner(ucfg, out, signer, audit)
		grant, err := union.NewGrantor(ucfg)
		if err != nil {
			log.Fatalf("union grantor: %v", err)
		}
		unionAPI, err = union.NewAPI(ucfg, out, usigner, grant, union.Hooks{}, audit)
		if err != nil {
			log.Fatalf("union api: %v", err)
		}
		unionGrant, unionOut = grant, out
		activeSigner = usigner                                         // hub-distributed key with local fallback
		go usigner.StartRotation(context.Background(), 12*time.Minute) // 10–15min base + internal jitter
		log.Printf("union: enabled (hub %s, queryemail %s)", cfg.UnionAPIRoot, cfg.UnionQueryEmailMode)
	}

	yggSvc := ygg.NewService(st, activeSigner)
	oidcDiscoveryURL := ""
	var oidcProvider *oidc.Provider
	if cfg.OIDCEnabled {
		okey, err := oidc.LoadOrCreateKey(cfg.OIDCSignKey)
		if err != nil {
			log.Fatalf("oidc key: %v", err)
		}
		oidcProvider, err = oidc.New(st, oidc.ProfileSource{
			Players: func(ctx context.Context, uid int64) ([]oidc.PlayerBrief, error) {
				ps, err := st.PlayersByUID(ctx, uid)
				if err != nil {
					return nil, err
				}
				out := make([]oidc.PlayerBrief, 0, len(ps))
				for _, p := range ps {
					out = append(out, oidc.PlayerBrief{ID: p.ID, UUID: p.UUID, Name: p.Name})
				}
				return out, nil
			},
			User: st.UserBrief,
		}, oidc.Config{
			Issuer:         cfg.OIDCIssuer,
			Key:            okey,
			SharedClientID: cfg.SharedClientID,
			LoginURL:       cfg.BaseURL + "/login",
		})
		if err != nil {
			log.Fatalf("oidc: %v", err)
		}
		oidcProvider.SessionResolver = st.SessionUser
		oidc.SetClientSecretVerifier(func(hash, secret string) bool { return auth.VerifyPHC(secret, hash) })
		if err := oidcProvider.SeedSharedClient(context.Background()); err != nil {
			log.Fatalf("oidc seed client: %v", err)
		}
		yggSvc.SetTokenResolver(oidc.NewYggTokenResolver(st, cfg.OIDCIssuer, &okey.PublicKey))
		oidcDiscoveryURL = cfg.BaseURL + "/api/yggdrasil/.well-known/openid-configuration"
		log.Printf("oidc: enabled (issuer %s, shared client %s)", cfg.OIDCIssuer, cfg.SharedClientID)
	}
	yggAPI := ygg.NewAPI(yggSvc, ygg.MetadataConfig{
		ServerName:             cfg.ServerName,
		Version:                version,
		Homepage:               cfg.BaseURL,
		Register:               cfg.BaseURL + "/auth/register",
		SkinDomains:            cfg.SkinDomains,
		OpenIDConfigurationURL: oidcDiscoveryURL,
	})
	backend := webapi.New(st, ts, yggSvc)
	pub := publicapi.New(st, ts)
	admin := adminapi.New(st, yggSvc)

	root := http.NewServeMux()
	root.Handle("/api/yggdrasil/", yggAPI.Handler())
	if oidcProvider != nil {
		root.Handle("GET /api/yggdrasil/.well-known/openid-configuration", oidcProvider.DiscoveryHandler())
	}
	if unionAPI != nil {
		root.Handle("/api/union/", unionAPI.Handler())
		root.Handle("GET /api/union/member/oauth2/grant", unionGrantHandler(unionGrant, unionOut, st, cfg.UnionAPIRoot))
	}
	root.Handle("/api/v1/", backend.Handler())
	root.Handle("/api/admin/v1/", admin.Handler())
	root.Handle("/api/public/", pub.Handler())
	root.Handle("/usm/", pub.Handler())
	root.Handle("/skin/", pub.Handler())
	root.Handle("/cape/", pub.Handler())
	root.Handle("GET /{player}.json", pub.CustomSkinAPIHandler())
	root.Handle("GET /textures/{hash}", textureHandler(ts))
	spa := spaHandler()
	if oidcProvider != nil {
		spa = oidcProvider.Wrap(spa)
	}
	root.Handle("/", spa)

	addr := cfg.Listen
	log.Printf("skin-nexus %s listening on %s (base %s)", version, addr, cfg.BaseURL)
	if err := http.ListenAndServe(addr, root); err != nil {
		log.Fatal(err)
	}
}

// unionGrantHandler is the login-state glue for the Union OAuth2 grant
// (analysis/yggdrasil-connect.md §4.5): fetch the hub encryption key, build
// the signed+encrypted userInfo token, 302 to the hub continuation URL.
func unionGrantHandler(grant *union.Grantor, out *union.Outbound, st *store.Mysql, apiRoot string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("skin_session")
		if err != nil {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		uid, err := st.SessionUser(r.Context(), c.Value)
		if err != nil {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		u, err := st.UserBrief(r.Context(), uid)
		if err != nil {
			http.Error(w, "user lookup failed", http.StatusInternalServerError)
			return
		}
		hubKey, err := out.OAuth2BackendKey(r.Context())
		if err != nil {
			http.Error(w, "hub key unavailable", http.StatusBadGateway)
			return
		}
		token, err := grant.GrantToken(uid, u.Nickname, u.Email, hubKey.PublicKey)
		if err != nil {
			http.Error(w, "grant failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, grant.GrantRedirectURL(token, apiRoot+"/oauth2/continue", r.URL.Query()), http.StatusFound)
	})
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
