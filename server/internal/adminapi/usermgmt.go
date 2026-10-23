package adminapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/store"
)

// Admin user management (P1): reset password / delete account.

// POST /api/admin/v1/users/{id}/password {password} — force-set a user's
// password. Side effects mirror the self-service change: ygg tokens die
// immediately (InvalidateUserTokens) and OIDC grants are revoked via the
// main-wired OnUserCredentialChange hook.
func (b *Backend) handleUserSetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "密码至少 8 位"})
		return
	}
	hash, err := auth.HashArgon2id(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "hash error"})
		return
	}
	if err := b.Store.AdminSetPassword(r.Context(), id, hash); errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "用户不存在"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
		return
	}
	_ = b.Store.InvalidateUserTokens(r.Context(), id)
	if b.OnUserCredentialChange != nil {
		b.OnUserCredentialChange(id)
	}
	b.Store.InsertAudit(r.Context(), 0, "admin_user_password_reset", map[string]any{"target": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /api/admin/v1/users/{id} — remove a user and their working
// credentials (transaction in store.DeleteUser; textures/audit kept).
// uid 1 (the operator account) is never deletable.
func (b *Backend) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	if id == 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "uid 1（管理员账号）不可删除"})
		return
	}
	if err := b.Store.DeleteUser(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "用户不存在"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
		return
	}
	b.Store.InsertAudit(r.Context(), 0, "admin_user_delete", map[string]any{"target": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
