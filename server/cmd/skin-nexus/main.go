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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"skin-nexus/server/internal/adminapi"
	"skin-nexus/server/internal/appearance"
	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/config"
	"skin-nexus/server/internal/migrate"
	"skin-nexus/server/internal/oidc"
	"skin-nexus/server/internal/publicapi"
	"skin-nexus/server/internal/ratelimit"
	"skin-nexus/server/internal/seo"
	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/texture"
	"skin-nexus/server/internal/theme"
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

	// Phase 3 / plan option A (2026-10-23): tracked migrations. Runs on every
	// SKIN_AUTO_MIGRATE boot — new files reach EXISTING databases (the
	// empty-DB-only path left 0006 unapplied on the live test instance);
	// schema_migrations markers skip completed files, duplicate-object
	// collisions from pre-tracking databases are tolerated, all else fatal.
	if cfg.AutoMigrate {
		if err := migrate.ApplySQLDirTracked(st.DB(), cfg.MigrationsDir); err != nil {
			log.Fatalf("schema: %v", err)
		}
	}

	// Setup wizard writes site_name/skin_domains to options; they override the
	// env defaults here (explicit env SKIN_SKIN_DOMAINS/SKIN_SITE_NAME wins).
	if os.Getenv("SKIN_SITE_NAME") == "" {
		if v, ok, err := st.GetOption(context.Background(), "site_name"); err == nil && ok && v != "" {
			cfg.SiteName = v
			cfg.ServerName = v
		}
	}
	// review P1 (2026-10-23): server_name/base_url options were write-only —
	// the settings panel / setup wizard wrote them and nothing read them.
	if os.Getenv("SKIN_SERVER_NAME") == "" {
		if v, ok, err := st.GetOption(context.Background(), "server_name"); err == nil && ok && v != "" {
			cfg.ServerName = v
		}
	}
	if os.Getenv("SKIN_BASE_URL") == "" {
		if v, ok, err := st.GetOption(context.Background(), "base_url"); err == nil && ok && v != "" {
			cfg.BaseURL = strings.TrimRight(v, "/")
		}
	}
	if os.Getenv("SKIN_SKIN_DOMAINS") == "" {
		if v, ok, err := st.GetOption(context.Background(), "skin_domains"); err == nil && ok && v != "" {
			for _, d := range strings.Split(v, ",") {
				if d = strings.TrimSpace(d); d != "" {
					cfg.SkinDomains = append(cfg.SkinDomains, d)
				}
			}
		}
	}

	ts, err := texture.NewStore(cfg.TexturesDir)
	if err != nil {
		log.Fatalf("textures: %v", err)
	}

	// Phase 3: SEO + appearance (theme packs, custom CSS/JS).
	seoEng := seo.NewEngine(st, seo.Defaults{SiteName: cfg.SiteName, BaseURL: cfg.BaseURL})
	themeReg, err := theme.NewRegistry(cfg.ThemesDir)
	if err != nil {
		log.Fatalf("themes: %v", err)
	}
	if n := len(themeReg.List()); n > 0 {
		log.Printf("themes: %d packs loaded from %s", n, cfg.ThemesDir)
	}
	appearEng := appearance.NewEngine(st, themeReg)

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
	var unionSigner *union.UnionSigner
	if cfg.UnionEnabled {
		hostPub, err := os.ReadFile(cfg.UnionHostPublicKeyPath)
		if err != nil {
			log.Fatalf("union host pubkey: %v", err)
		}
		sigPEM, err := os.ReadFile(cfg.UnionSigPrivateKey)
		if err != nil {
			log.Fatalf("union oauth2 sig key: %v", err)
		}
		// T3.2: audit rows go to the DB (plus the log); actor_uid NULL = system.
		audit := func(event string, detail map[string]any) {
			log.Printf("union audit: %s %v", event, detail)
			st.InsertAudit(context.Background(), 0, "union."+event, detail)
		}
		// member key rotation persistence (T3.3): a rotated key survives
		// restarts via options and overrides the env initial value.
		memberKey := cfg.UnionMemberKey
		if k, ok, err := st.GetOption(context.Background(), "union_member_key"); err == nil && ok && k != "" {
			memberKey = k
			log.Printf("union: member key restored from options (rotated)")
		}
		ucfg := union.Config{
			Enabled:               true,
			ApiRoot:               cfg.UnionAPIRoot,
			MemberKey:             memberKey,
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
		// T3.3: member data hooks — profile sync list, UUID remap, queryemail.
		hooks := union.Hooks{
			OnServerList: func(list any, version string) {
				_ = st.SetOption(context.Background(), "union_server_list_version", version)
				// Legacy skinDomains formula: member hosts feed the texture
				// whitelist (options-cached like union_server_list).
				if raw, err := json.Marshal(list); err == nil {
					var entries []struct {
						BSRoot string `json:"bs_root"`
					}
					if err := json.Unmarshal(raw, &entries); err == nil {
						hosts := make([]string, 0, len(entries))
						for _, e := range entries {
							if u, err := url.Parse(e.BSRoot); err == nil && u.Host != "" {
								hosts = append(hosts, u.Host)
							}
						}
						_ = st.SetOption(context.Background(), "union_skin_domains", strings.Join(hosts, ","))
					}
				}
			},
			OnMemberKeyRotated: func(newKey string) {
				if err := st.SetOption(context.Background(), "union_member_key", newKey); err != nil {
					audit("member_key_persist_failed", map[string]any{"error": err.Error()})
					return
				}
				audit("member_key_rotated", nil)
			},
			OnRemapUUID: func(remap map[string]string) error {
				applied, skipped, err := st.RemapUUIDs(context.Background(), remap)
				if err != nil {
					return err
				}
				// true counts (MUA review P1: the old audit logged len(batch)
				// regardless of rows actually moved)
				st.InsertAudit(context.Background(), 0, "union_remapuuid_detail",
					map[string]interface{}{"applied": applied, "skipped": skipped})
				return nil
			},
			ProfileListFunc: func() (map[string]string, error) {
				return st.ProfileNameByUUID(context.Background())
			},
			QueryEmailFunc: func(name string) (string, bool, error) {
				return st.EmailByPlayerName(context.Background(), name)
			},
		}
		unionAPI, err = union.NewAPI(ucfg, out, usigner, grant, hooks, audit)
		if err != nil {
			log.Fatalf("union api: %v", err)
		}
		unionGrant, unionOut, unionSigner = grant, out, usigner
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
		oidcProvider.Audit = func(event string, detail map[string]any) {
			log.Printf("oidc audit: %s %v", event, detail)
			st.InsertAudit(context.Background(), 0, event, detail)
		}
		oidc.SetClientSecretVerifier(func(hash, secret string) bool { return auth.VerifyPHC(secret, hash) })
		if err := oidcProvider.SeedSharedClient(context.Background()); err != nil {
			log.Fatalf("oidc seed client: %v", err)
		}
		// Dual-state token resolution: OIDC JWTs resolve through the jti
		// ledger; opaque password-flow tokens fall back to the store lookup.
		// Only ErrNotJOSE falls back — a JWT-shaped token that fails
		// verification must not degrade to another authority (2026-10-23).
		jwtResolve := oidc.NewYggTokenResolver(st, cfg.OIDCIssuer, &okey.PublicKey)
		yggSvc.SetTokenResolver(func(ctx context.Context, access string) (ygg.Token, error) {
			t, err := jwtResolve(ctx, access)
			if errors.Is(err, oidc.ErrNotJOSE) {
				return st.TokenByAccess(ctx, access)
			}
			return t, err
		})
		oidcDiscoveryURL = cfg.BaseURL + "/api/yggdrasil/.well-known/openid-configuration"
		log.Printf("oidc: enabled (issuer %s, shared client %s)", cfg.OIDCIssuer, cfg.SharedClientID)
	}
	yggSvc.RequireVerification = cfg.RequireVerification
	yggAPI := ygg.NewAPI(yggSvc, ygg.MetadataConfig{
		ServerName:             cfg.ServerName,
		Version:                version,
		Homepage:               cfg.BaseURL,
		Register:               cfg.BaseURL + "/auth/register",
		SkinDomains:            cfg.SkinDomains,
		OpenIDConfigurationURL: oidcDiscoveryURL,
		ExtraSkinDomains: func() []string {
			if v, ok, err := st.GetOption(context.Background(), "union_skin_domains"); err == nil && ok && v != "" {
				return strings.Split(v, ",")
			}
			return nil
		},
	})
	backend := webapi.New(st, ts, yggSvc)
	backend.RequireInvite = cfg.RequireInvite
	backend.OnPasswordChange = func(uid int64) {
		if oidcProvider != nil {
			if err := oidcProvider.RevokeUserAllTokens(context.Background(), uid); err != nil {
				log.Printf("password change: OIDC revoke uid=%d: %v", uid, err)
			}
		}
	}
	// Options panel → request-time join gate (no restart needed).
	yggSvc.RequireVerificationResolver = func(ctx context.Context) bool {
		if v, ok, err := st.GetOption(ctx, "require_verification"); err == nil && ok {
			return v == "1" || v == "true"
		}
		return cfg.RequireVerification
	}
	// Rate limiting (P1 ygg_rate_limit): settings-panel values override env
	// at boot (limiter thresholds are boot-constructed; restart to apply).
	if v, ok, err := st.GetOption(context.Background(), "rate_limit_auth"); err == nil && ok {
		if n, e := strconv.Atoi(v); e == nil {
			cfg.RateLimitAuth = n
		}
	}
	if v, ok, err := st.GetOption(context.Background(), "login_fail_limit"); err == nil && ok {
		if n, e := strconv.Atoi(v); e == nil {
			cfg.LoginFailLimit = n
		}
	}
	rateLimiter := ratelimit.New(cfg.RateLimitAuth, 10*time.Minute, cfg.LoginFailLimit, 15*time.Minute)
	yggSvc.Limiter = rateLimiter
	backend.Limiter = rateLimiter
	if unionOut != nil {
		backend.MountUnion(unionOut) // registers /api/v1/union/* (zero surface when disabled)
	}
	pub := publicapi.New(st, ts)
	admin := adminapi.New(st, yggSvc)
	// Settings panel: env-derived fallbacks; option values win per request.
	admin.SettingsDefaults = map[string]string{
		"site_name":            cfg.SiteName,
		"server_name":          cfg.ServerName,
		"skin_domains":         strings.Join(cfg.SkinDomains, ","),
		"invite_required":      boolStr(cfg.RequireInvite),
		"require_verification": boolStr(cfg.RequireVerification),
		"rate_limit_auth":      strconv.Itoa(cfg.RateLimitAuth),
		"login_fail_limit":     strconv.Itoa(cfg.LoginFailLimit),
	}
	// Admin user management shares the credential-change revoke semantics.
	admin.OnUserCredentialChange = func(uid int64) {
		if oidcProvider != nil {
			if err := oidcProvider.RevokeUserAllTokens(context.Background(), uid); err != nil {
				log.Printf("admin credential change: OIDC revoke uid=%d: %v", uid, err)
			}
		}
	}
	admin.SetSharedClientID(cfg.SharedClientID) // guards shared launcher client deletion
	admin.MountAppearance(adminapi.AppearanceDeps{SEO: seoEng, Appear: appearEng, Themes: themeReg})
	if unionSigner != nil {
		admin.MountUnion(adminapi.UnionDeps{
			Status: func() map[string]any {
				return map[string]any{
					"enabled":             true,
					"mode":                unionSigner.Mode(),
					"key_fingerprint":     unionSigner.KeyFingerprint(),
					"private_key_version": unionSigner.PrivateKeyVersion(),
					"server_list_version": unionAPI.ServerListVersion(),
					"rotation_suspended":  unionSigner.RotationSuspended(),
					"queryemail_mode":     cfg.UnionQueryEmailMode,
					"hub_origin":          cfg.UnionHubOrigin,
					"allow_update_bk_key": cfg.UnionAllowUpdateBkKey,
				}
			},
			ResumeRotation: unionSigner.ResumeRotation,
		})
	}

	root := http.NewServeMux()
	root.Handle("/api/yggdrasil/", yggAPI.Handler())
	if oidcProvider != nil {
		root.Handle("GET /api/yggdrasil/.well-known/openid-configuration", oidcProvider.DiscoveryHandler())
		// /api/v1/oidc/* must be claimed at the root mux: the webapi
		// "/api/v1/" subtree below would otherwise shadow the provider's
		// device/token API (live: /device page HTTP 404, 2026-10-23).
		root.Handle("/api/v1/oidc/", oidcProvider.DeviceAPIHandler())
	}
	if unionAPI != nil {
		// 1 MiB body cap (MUA review P2): /api/union/* verified its signature
		// over io.ReadAll — an unbounded body was a memory DoS surface.
		root.Handle("/api/union/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			}
			unionAPI.Handler().ServeHTTP(w, r)
		}))
		root.Handle("GET /api/union/member/oauth2/grant", unionGrantHandler(unionGrant, unionOut, st, cfg.UnionAPIRoot))
	}
	root.Handle("/api/v1/", backend.Handler())
	root.Handle("/api/admin/v1/", admin.Handler())
	root.Handle("/api/public/", pub.Handler())
	root.Handle("/usm/", pub.Handler())
	root.Handle("/skin/", pub.Handler())
	root.Handle("/cape/", pub.Handler())
	// CustomSkinAPI R1 (GET /{player}.json) is served by pub.WrapRoot below:
	// ServeMux cannot register "{player}.json" next to the "/" SPA catch-all.
	root.Handle("GET /textures/{hash}", textureHandler(ts))
	// Phase 3 public surface: health probe, SEO outputs, theme assets.
	root.Handle("GET /healthz", healthzHandler(st))
	root.Handle("GET /sitemap.xml", sitemapHandler(seoEng, st, cfg.BaseURL))
	root.Handle("GET /robots.txt", robotsHandler(seoEng))
	root.Handle("GET /theme-assets/{id}/{file}", themeAssetHandler(themeReg))
	spa := spaHandler(seoEng, appearEng)
	if oidcProvider != nil {
		spa = oidcProvider.Wrap(spa)
	}
	root.Handle("/", spa)

	addr := cfg.Listen
	log.Printf("skin-nexus %s listening on %s (base %s)", version, addr, cfg.BaseURL)
	if err := http.ListenAndServe(addr, pub.WrapRoot(root)); err != nil {
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

// spaHandler serves web/dist if present, else a placeholder page. index.html
// passes through the SEO + appearance injection layers (phase 3).
func spaHandler(seoEng *seo.Engine, appearEng *appearance.Engine) http.Handler {
	dir := "web/dist"
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		fs := http.FileServer(http.Dir(dir))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: unknown paths -> index.html
			p := strings.TrimPrefix(r.URL.Path, "/")
			if _, err := os.Stat(dir + "/" + p); err != nil && !strings.HasPrefix(p, "api/") {
				raw, err := os.ReadFile(dir + "/index.html")
				if err != nil {
					http.Error(w, "index missing", http.StatusInternalServerError)
					return
				}
				html := string(raw)
				html = seoEng.RenderHead(r.Context(), html)
				html = appearEng.Render(r.Context(), html)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(html))
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

// healthzHandler is the container health probe (DB round-trip).
func healthzHandler(st *store.Mysql) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := st.Health(r.Context()); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// sitemapHandler serves /sitemap.xml (static routes + bounded player pages).
func sitemapHandler(seoEng *seo.Engine, st *store.Mysql, baseURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := seoEng.Settings(r.Context())
		if !s.SitemapOn {
			http.NotFound(w, r)
			return
		}
		paths := []string{}
		// bounded: at most 5000 profile pages
		if rows, err := st.ProfileNameByUUID(r.Context()); err == nil {
			names := make([]string, 0, len(rows))
			for _, name := range rows {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) > 5000 {
				names = names[:5000]
			}
			for _, n := range names {
				paths = append(paths, "/profile/"+url.PathEscape(n))
			}
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write([]byte(seo.SitemapXML(s.Canonical, baseURL, paths)))
	})
}

// robotsHandler serves /robots.txt from the SEO settings.
func robotsHandler(seoEng *seo.Engine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := seoEng.Settings(r.Context())
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(s.Robots))
	})
}

// themeAssetHandler serves theme pack assets with strict path validation.
func themeAssetHandler(reg *theme.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		file := r.PathValue("file")
		full, err := reg.ResolveAsset(id, file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFile(w, r, full)
	})
}
