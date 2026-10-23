// Package adminapi serves the operator surface: user list, texture list,
// weak-hash dashboard, signing key fingerprint, audit log.
//
// Admin gating: phase-1 uses a shared admin token (X-Admin-Token header,
// SKIN_ADMIN_TOKEN env) — small-footprint single-operator model. Role-based
// admin lands when the plugin framework ships (HookAPI v2 admin hooks).
package adminapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/ygg"
)

type Backend struct {
	Store          *store.Mysql
	Ygg            *ygg.Service
	AdminTok       string
	union          UnionDeps        // nil until MountUnion (module disabled = no routes)
	sharedClientID string           // guard: shared launcher client cannot be deleted
	appearance     *AppearanceDeps  // nil until MountAppearance
	// SettingsDefaults carries env-derived fallbacks for the settings panel
	// (injected by main from config).
	SettingsDefaults map[string]string
	// OnUserCredentialChange fires on admin password reset (main wires OIDC
	// grant revocation — same semantics as the self-service change).
	OnUserCredentialChange func(uid int64)
}

func New(st *store.Mysql, yggSvc *ygg.Service) *Backend {
	return &Backend{Store: st, Ygg: yggSvc, AdminTok: os.Getenv("SKIN_ADMIN_TOKEN")}
}

// jsonDecode is a one-shot JSON body decoder shared by admin handlers.
func jsonDecode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (b *Backend) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/admin/v1/overview", b.auth(b.handleOverview))
	mux.HandleFunc("GET /api/admin/v1/users", b.auth(b.handleUsers))
	mux.HandleFunc("GET /api/admin/v1/textures", b.auth(b.handleTextures))
	mux.HandleFunc("GET /api/admin/v1/audit", b.auth(b.handleAudit))
	// Invitation codes (1:1 live blessing plugin admin surface).
	mux.HandleFunc("GET /api/admin/v1/invitation-codes", b.auth(b.handleInvitationList))
	mux.HandleFunc("POST /api/admin/v1/invitation-codes", b.auth(b.handleInvitationGenerate))
	mux.HandleFunc("PUT /api/admin/v1/invitation-codes", b.auth(b.handleInvitationEdit))
	mux.HandleFunc("GET /api/admin/v1/settings", b.auth(b.handleSettingsGet))
	mux.HandleFunc("PUT /api/admin/v1/settings", b.auth(b.handleSettingsPut))
	mux.HandleFunc("POST /api/admin/v1/users/{id}/password", b.auth(b.handleUserSetPassword))
	mux.HandleFunc("DELETE /api/admin/v1/users/{id}", b.auth(b.handleUserDelete))
	if b.union.Status != nil {
		mux.HandleFunc("GET /api/admin/v1/union/status", b.auth(b.handleUnionStatus))
		mux.HandleFunc("POST /api/admin/v1/union/rotation/resume", b.auth(b.handleUnionRotationResume))
	}
	// OIDC client management (works regardless of module flag; the shared
	// launcher client deletion is guarded by sharedClientID).
	mux.HandleFunc("GET /api/admin/v1/oidc/clients", b.auth(b.handleOIDCClientsList))
	mux.HandleFunc("POST /api/admin/v1/oidc/clients", b.auth(b.handleOIDCClientsCreate))
	mux.HandleFunc("DELETE /api/admin/v1/oidc/clients/{id}", b.auth(b.handleOIDCClientsDelete))
	// SEO / appearance / themes (phase 3).
	if b.appearance != nil {
		mux.HandleFunc("GET /api/admin/v1/seo", b.auth(b.handleSEOGet))
		mux.HandleFunc("POST /api/admin/v1/seo", b.auth(b.handleSEOSave))
		mux.HandleFunc("POST /api/admin/v1/seo/oneclick", b.auth(b.handleSEOOneClick))
		mux.HandleFunc("GET /api/admin/v1/appearance", b.auth(b.handleAppearanceGet))
		mux.HandleFunc("POST /api/admin/v1/appearance", b.auth(b.handleAppearanceSave))
		mux.HandleFunc("GET /api/admin/v1/themes", b.auth(b.handleThemesList))
	}
	return mux
}

// auth enforces the admin token (constant-time compare; empty token = locked).
func (b *Backend) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Admin-Token")
		if b.AdminTok == "" || subtle.ConstantTimeCompare([]byte(got), []byte(b.AdminTok)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"Forbidden","message":"admin token invalid"}`))
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleOverview: counters powering the dashboard header.
func (b *Backend) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	weak, _ := b.Store.WeakHashCount(ctx)
	users, _ := b.Store.CountUsers(ctx)
	players, _ := b.Store.CountPlayers(ctx)
	textures, _ := b.Store.CountTextures(ctx)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": users, "players": players, "textures": textures,
		"weakHashUsers":      weak, // 「弱哈希余量」看板核心指标
		"signKeyFingerprint": b.Ygg.SignerFingerprint(),
	})
}

func (b *Backend) handleUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	rows, total, err := b.Store.AdminUsers(r.Context(), page, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": rows, "total": total, "page": page})
}

func (b *Backend) handleTextures(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	rows, total, err := b.Store.AdminTextures(r.Context(), page, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"textures": rows, "total": total, "page": page})
}

func (b *Backend) handleAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := b.Store.RecentAudit(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"audit": rows})
}

var _ = context.Background
