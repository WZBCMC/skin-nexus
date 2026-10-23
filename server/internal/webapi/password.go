package webapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"skin-nexus/server/internal/auth"
)

// sessionCookieValue returns the caller's raw session token ("" if none).
func sessionCookieValue(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

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
	// Passwords may legitimately contain spaces — never trim (review P3:
	// the old TrimSpace silently changed the credential the user typed).
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
	// Failure is non-fatal to the login session but must leave a trace
	// (review P2: silent swallow hid half-done revocations).
	if err := b.Store.InvalidateUserTokens(r.Context(), uid); err != nil {
		b.Store.InsertAudit(r.Context(), uid, "password_change_token_revoke_failed", map[string]any{"error": err.Error()})
	}
	// Launcher/OIDC grants die too (legacy RevokeUserTokens semantics); the
	// hook is panic-shielded so a bad hook can't cut the response after the
	// hash already changed.
	if b.OnPasswordChange != nil {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					b.Store.InsertAudit(r.Context(), uid, "password_change_hook_panic", map[string]any{"panic": fmt.Sprint(rec)})
				}
			}()
			b.OnPasswordChange(uid)
		}()
	}
	// Other web sessions die too (stolen-session hygiene); the caller's own
	// session survives by design.
	if err := b.Store.DeleteUserSessionsExcept(r.Context(), uid, sessionCookieValue(r)); err != nil {
		b.Store.InsertAudit(r.Context(), uid, "password_change_session_purge_failed", map[string]any{"error": err.Error()})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
