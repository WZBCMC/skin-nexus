// Package webapi serves the SkinNexus SPA backend: registration, login
// (web session cookie), profile, texture upload, player management.
//
// All responses are JSON except texture file serving. Errors use
// {"error": "...", "message": "..."} — distinct from the Yggdrasil legacy
// shape (which stays frozen in the ygg package).
package webapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/store"
	"skin-nexus/server/internal/texture"
	"skin-nexus/server/internal/union"
	"skin-nexus/server/internal/ygg"
)

var (
	nickRe = regexp.MustCompile(`^[A-Za-z0-9_一-龥]{2,24}$`)
	mailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// Backend bundles dependencies for the web API.
type Backend struct {
	Store    *store.Mysql
	Textures *texture.Store
	// RequireInvite forces invitationCode at registration
	// (SKIN_INVITE_REQUIRED). Default off = optional code, validated and
	// burned when supplied (legacy live semantics: required + "邀请码不能为空").
	RequireInvite bool
	Ygg      *ygg.Service
	MaxChars int // per-user player limit

	// union is the MUA Union hub client (nil or disabled -> no /api/v1/union/*
	// routes exist at all: zero surface for non-alliance deployments).
	union    *union.Outbound
	unionSrv *unionServerCache
}

func New(st *store.Mysql, ts *texture.Store, yggSvc *ygg.Service) *Backend {
	return &Backend{Store: st, Textures: ts, Ygg: yggSvc, MaxChars: 3}
}

const sessionCookie = "skin_session"

// Handler mounts all /api/v1 routes.
func (b *Backend) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/register", b.handleRegister)
	mux.HandleFunc("POST /api/v1/login", b.handleLogin)
	mux.HandleFunc("POST /api/v1/logout", b.handleLogout)
	mux.HandleFunc("GET /api/v1/me", b.handleMe)
	mux.HandleFunc("GET /api/v1/me/textures", b.handleListTextures)
	mux.HandleFunc("POST /api/v1/me/textures", b.handleUploadTexture)
	mux.HandleFunc("GET /api/v1/me/players", b.handleListPlayers)
	mux.HandleFunc("POST /api/v1/me/players", b.handleCreatePlayer)
	mux.HandleFunc("PUT /api/v1/me/players/{id}/skin", b.handleSetSkin)
	mux.HandleFunc("PUT /api/v1/me/players/{id}/cape", b.handleSetCape)
	mux.HandleFunc("GET /api/v1/stats", b.handleStats)
	// Install wizard (empty-DB init): state-gated, one-way door after done.
	mux.HandleFunc("GET /api/v1/setup/status", b.handleSetupStatus)
	mux.HandleFunc("POST /api/v1/setup", b.handleSetup)
	// Union user surface: only when the module is enabled (zero surface off).
	if b.union != nil && b.union.Enabled() {
		mux.HandleFunc("GET /api/v1/union/servers", b.handleUnionServers)
		mux.HandleFunc("GET /api/v1/union/detail", b.handleUnionDetail)
		mux.HandleFunc("GET /api/v1/union/unmapped", b.handleUnionUnmapped)
		mux.HandleFunc("POST /api/v1/union/bind", b.handleUnionBind)
		mux.HandleFunc("POST /api/v1/union/bindto", b.handleUnionBindTo)
		mux.HandleFunc("POST /api/v1/union/unbind", b.handleUnionUnbind)
		mux.HandleFunc("POST /api/v1/union/remapuuid", b.handleUnionRemapUUID)
	}
	return mux
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": http.StatusText(status), "message": msg})
}

// writeErrCode adds a stable machine-readable code on top of the legacy
// message shape (i18n skeleton: frontends translate `code`, fall back to
// `message`). Existing call sites keep writeErr; new errors use this.
func writeErrCode(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, map[string]string{
		"error": http.StatusText(status), "message": msg, "code": code,
	})
}

// decodeJSON is a one-shot JSON body decoder.
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (b *Backend) currentUser(r *http.Request) (int64, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return 0, store.ErrNotFound
	}
	return b.Store.SessionUser(r.Context(), c.Value)
}

func randomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// ---- auth handlers ----

func (b *Backend) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Nickname string `json:"nickname"`
		Password string `json:"password"`
		// invitationCode is the legacy field name (blessing plugin register form).
		InvitationCode string `json:"invitationCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if !nickRe.MatchString(req.Nickname) {
		writeErr(w, http.StatusBadRequest, "昵称需 2-24 位字母/数字/下划线/中文")
		return
	}
	if req.Email != "" && !mailRe.MatchString(req.Email) {
		writeErr(w, http.StatusBadRequest, "邮箱格式不正确")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "密码至少 8 位")
		return
	}
	hash, err := auth.HashArgon2id(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	code := strings.TrimSpace(req.InvitationCode)
	if code == "" && b.RequireInvite {
		writeErr(w, http.StatusBadRequest, "邀请码不能为空")
		return
	}
	var uid int64
	if code != "" {
		uid, err = b.Store.CreateUserWithInvitation(r.Context(), req.Email, req.Nickname, hash, b.MaxChars, code)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusBadRequest, "邀请码无效")
			return
		}
	} else {
		uid, err = b.Store.CreateUser(r.Context(), req.Email, req.Nickname, hash, b.MaxChars)
	}
	if err != nil {
		// unique constraint violation -> nickname/email taken
		writeErr(w, http.StatusConflict, "昵称或邮箱已被占用")
		return
	}
	b.issueSession(w, uid)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": uid, "nickname": req.Nickname})
}

func (b *Backend) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier string `json:"identifier"` // email or nickname
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	u, err := b.Store.UserByIdentifier(r.Context(), req.Identifier)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	ok, err := b.Store.VerifyPassword(r.Context(), u.ID, req.Password)
	if err != nil || !ok {
		writeErr(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}
	// Lazy rehash: legacy digest verified -> upgrade in place (official
	// strategy: login success triggers changePassword).
	if algo, err := b.Store.UserPasswordAlgo(r.Context(), u.ID); err == nil && strings.HasPrefix(algo, "legacy") {
		if nh, err := auth.HashArgon2id(req.Password); err == nil {
			_ = b.Store.RehashPassword(r.Context(), u.ID, nh)
		}
	}
	b.issueSession(w, u.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"id": u.ID, "nickname": u.Nickname})
}

func (b *Backend) issueSession(w http.ResponseWriter, uid int64) {
	tok := randomToken()
	exp := time.Now().Add(30 * 24 * time.Hour)
	_ = b.Store.CreateSession(context.Background(), tok, uid, exp)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Expires: exp,
	})
}

func (b *Backend) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = b.Store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (b *Backend) handleMe(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	u, err := b.Store.UserByID(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id": u.ID, "nickname": u.Nickname,
		"emailVerified": u.EmailVerified, "playerLimit": b.MaxChars,
	})
}

// ---- textures ----

func (b *Backend) handleListTextures(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	rows, err := b.Store.ListTextures(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	type item struct {
		ID          int64  `json:"id"`
		Type        string `json:"type"`
		Hash        string `json:"hash"`
		Model       string `json:"model"`
		Description string `json:"description"`
		URL         string `json:"url"`
		UploadAt    string `json:"uploadAt"`
	}
	out := make([]item, 0, len(rows))
	for _, t := range rows {
		out = append(out, item{
			ID: t.ID, Type: t.Type, Hash: t.Hash, Model: t.Model,
			Description: t.Description,
			URL:         b.Store.TextureURL(t.Hash),
			UploadAt:    t.UploadAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"textures": out})
}

func (b *Backend) handleUploadTexture(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "multipart parse failed")
		return
	}
	typ := texture.Type(r.FormValue("type"))
	if typ != texture.TypeSkin && typ != texture.TypeCape {
		writeErr(w, http.StatusBadRequest, "type must be skin|cape")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()

	res, err := texture.Process(file, typ, r.FormValue("model"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := b.Textures.Save(res); err != nil {
		writeErr(w, http.StatusInternalServerError, "save failed")
		return
	}
	tid, err := b.Store.AddTexture(r.Context(), uid, string(typ), res.Hash, res.Model, r.FormValue("description"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id": tid, "type": typ, "hash": res.Hash, "url": b.Store.TextureURL(res.Hash),
	})
}

// ---- players ----

func (b *Backend) handleListPlayers(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	players, err := b.Store.PlayersByUID(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"players": players})
}

func (b *Backend) handleCreatePlayer(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !nickRe.MatchString(req.Name) || len(req.Name) > 16 {
		writeErr(w, http.StatusBadRequest, "角色名需 2-16 位")
		return
	}
	pid, uuid, err := b.Store.CreatePlayer(r.Context(), uid, req.Name)
	if err != nil {
		writeErr(w, http.StatusConflict, "角色名已被占用")
		return
	}
	// Union: announce the new profile to the hub (async, 5s bound, never
	// blocks or fails the user response). Rename/delete endpoints don't exist
	// yet in webapi — wire SyncProfileRename/SyncProfileDelete when they land.
	if b.union != nil && b.union.Enabled() {
		name, u := req.Name, uuid
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := b.union.SyncProfileCreate(ctx, u, name); err != nil {
				b.Store.InsertAudit(ctx, 0, "union_sync_create_failed",
					map[string]interface{}{"uuid": u, "name": name, "error": err.Error()})
			}
		}()
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"id": pid, "name": req.Name, "uuid": uuid})
}

// handleSetSkin / handleSetCape point a player at one of the user's textures.
func (b *Backend) handleSetSkin(w http.ResponseWriter, r *http.Request) {
	b.setPlayerTexture(w, r, "skin")
}
func (b *Backend) handleSetCape(w http.ResponseWriter, r *http.Request) {
	b.setPlayerTexture(w, r, "cape")
}

func (b *Backend) setPlayerTexture(w http.ResponseWriter, r *http.Request, kind string) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	pid, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		TextureID int64 `json:"textureId"` // 0 = unset
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	// Ownership: texture must belong to this user.
	if req.TextureID != 0 {
		owner, err := b.Store.TextureOwner(r.Context(), req.TextureID)
		if err != nil || owner != uid {
			writeErr(w, http.StatusForbidden, "not yours")
			return
		}
	}
	// Ownership: player must belong to this user.
	players, err := b.Store.PlayersByUID(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	found := false
	for i := range players {
		if players[i].ID == pid {
			found = true
			break
		}
	}
	if !found {
		writeErr(w, http.StatusNotFound, "player not found")
		return
	}
	skinTid, capeTid, err := b.Store.PlayerTextureIDs(r.Context(), pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	if kind == "skin" {
		skinTid = req.TextureID
	} else {
		capeTid = req.TextureID
	}
	if err := b.Store.SetPlayerTextures(r.Context(), pid, skinTid, capeTid); err != nil {
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "playerId": pid})
}

func (b *Backend) handleStats(w http.ResponseWriter, r *http.Request) {
	weak, _ := b.Store.WeakHashCount(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{"weakHashUsers": weak})
}

var _ = errors.Is
