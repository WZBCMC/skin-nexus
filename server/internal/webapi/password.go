package webapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"skin-nexus/server/internal/auth"
)

// handlePasswordChange: POST /api/v1/me/password
// {currentPassword, newPassword}. Legacy semantics (phase2 A2 step 8):
// after a successful change ALL launcher/OIDC authorizations die
// (RevokeUserTokens) — wired via OnPasswordChange in main. The caller's own
// web session stays (they just proved control).
func (b *Backend) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	uid, err := b.currentUser(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	ok, err := b.Store.VerifyPassword(r.Context(), uid, req.CurrentPassword)
	if err != nil || !ok {
		writeErr(w, http.StatusUnauthorized, "当前密码不正确")
		return
	}
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if len(req.NewPassword) < 8 {
		writeErr(w, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeErr(w, http.StatusBadRequest, "新密码不能与当前密码相同")
		return
	}
	hash, err := auth.HashArgon2id(req.NewPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "hash error")
		return
	}
	// RehashPassword sets password + password_algo='argon2id' in one shot —
	// legacy weak-hash rows are upgraded by the change itself.
	if err := b.Store.RehashPassword(r.Context(), uid, hash); err != nil {
		writeErr(w, http.StatusInternalServerError, "update failed")
		return
	}
	// Kill every yggdrasil token this user holds (join/validate credentials).
	if err := b.Store.InvalidateUserTokens(r.Context(), uid); err != nil {
		// non-fatal: password already changed; tokens still die on next TTL
		_ = err
	}
	// Launcher/OIDC grants die too (legacy RevokeUserTokens semantics).
	if b.OnPasswordChange != nil {
		b.OnPasswordChange(uid)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
