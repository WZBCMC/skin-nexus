package adminapi

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"skin-nexus/server/internal/auth"
	"skin-nexus/server/internal/plugin"
	"skin-nexus/server/internal/store"
)

// Admin user management (P1): reset password / delete account. B2 adds the
// blessing-parity inline edits (email / nickname / verification / permission).

// Local copies of webapi's validation patterns (same rules, admin path must
// reject what registration rejects — see webapi.nickRe / webapi.mailRe).
var (
	adminNickRe = regexp.MustCompile(`^[A-Za-z0-9_一-龥]{2,24}$`)
	adminMailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// idParam parses the {id} path value.
func idParam(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// writeStoreErr maps store sentinel errors to the HTTP envelope.
func writeStoreErr(w http.ResponseWriter, err error, notFoundMsg, conflictMsg string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": notFoundMsg})
	case errors.Is(err, store.ErrDuplicate):
		writeJSON(w, http.StatusConflict, map[string]any{"error": conflictMsg})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "update failed"})
	}
}

// inlineGuard enforces the blessing user-management guards (archive §6.2)
// on an inline edit:
//  1. no self-edit (禁改自己)
//  2. the target's permission must be STRICTLY below the actor's
//     (只能操作严格小于自己的)
//
// The token/API path (actorUID == 0) is the operator's machine fallback and
// bypasses these identity guards — except the permission endpoint below,
// which refuses it outright. Returns an error message to render, or "".
func (b *Backend) inlineGuard(r *http.Request, target int64) string {
	actor := actorUID(r)
	if actor == 0 {
		return "" // machine fallback: operator authority, no identity guards
	}
	if actor == target {
		return "不能修改自己的管理项"
	}
	actorPerm, err := b.Store.UserPermission(r.Context(), actor)
	if err != nil {
		return "权限校验失败"
	}
	targetPerm, err := b.Store.UserPermission(r.Context(), target)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "用户不存在"
		}
		return "权限校验失败"
	}
	if targetPerm >= actorPerm {
		return "只能操作权限严格低于自己的账号"
	}
	return ""
}

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
	// Same blessing guards as the inline edits: without this, any admin could
	// reset a SUPER_ADMIN's password and take over the account.
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
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
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_password_reset", map[string]any{"target": id})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /api/admin/v1/users/{id} — remove a user and their working
// credentials (transaction in store.DeleteUser; textures/audit kept).
// Protection is permission-based first: SUPER_ADMIN (permission >= 2)
// accounts are never deletable — migrated databases do NOT have the operator
// at uid 1 (here uid 1 is a legacy regular user), so the old uid==1-only
// guard left the real super admin deletable by any admin. uid 1 stays
// undeletable too (blessing parity, belt and suspenders). inlineGuard applies:
// no self-delete, only strictly-lower-permission targets.
func (b *Backend) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	if id == 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "uid 1（管理员账号）不可删除"})
		return
	}
	if targetPerm, err := b.Store.UserPermission(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "用户不存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "权限校验失败"})
		return
	} else if targetPerm >= 2 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "超级管理员账号不可删除"})
		return
	}
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
		return
	}
	// Union profile lifecycle: the (uuid, name) pairs must be captured BEFORE
	// DeleteUser — the transaction removes the players and their uuid_map
	// rows. Best-effort: a lookup failure just skips the hub announcement
	// (DeleteUser itself must not be blocked by union bookkeeping).
	var players [][2]string
	if b.OnUserDeleted != nil {
		if ps, err := b.Store.PlayersByUID(r.Context(), id); err == nil {
			for _, p := range ps {
				if p.UUID != "" { // no uuid_map row => no union identity to retire
					players = append(players, [2]string{p.UUID, p.Name})
				}
			}
		}
	}
	if err := b.Store.DeleteUser(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "用户不存在"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "delete failed"})
		return
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_delete", map[string]any{"target": id})
	if b.OnUserDeleted != nil {
		b.OnUserDeleted(id, players) // async inside the hook; never blocks this response
	}
	if b.OnUserLifecycle != nil {
		b.OnUserLifecycle(r.Context(), plugin.UserLifecycleEvent{Kind: "deleted", UserID: id})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- inline edits (B2, blessing 管理面板 用户管理 parity) ----

// PUT /api/admin/v1/users/{id}/email {email} — set or clear ("") the email.
// Same-value writes are a no-op success (changed-rows semantics would make
// a failed save indistinguishable from an unchanged one in the UI).
func (b *Backend) handleUserSetEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email != "" && !adminMailRe.MatchString(req.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "邮箱格式不正确"})
		return
	}
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
		return
	}
	if err := b.Store.UpdateUserEmail(r.Context(), id, req.Email); err != nil {
		writeStoreErr(w, err, "用户不存在", "该邮箱已被其他账号使用")
		return
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_email_update", map[string]any{"target": id, "email": req.Email})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PUT /api/admin/v1/users/{id}/nickname {nickname} — set the nickname
// (nickRe + uniqueness; the column is NOT NULL so empty is rejected).
func (b *Backend) handleUserSetNickname(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if !adminNickRe.MatchString(req.Nickname) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "昵称需 2-24 位字母/数字/下划线/中文"})
		return
	}
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
		return
	}
	if err := b.Store.UpdateUserNickname(r.Context(), id, req.Nickname); err != nil {
		writeStoreErr(w, err, "用户不存在", "该昵称已被其他账号使用")
		return
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_nickname_update", map[string]any{"target": id, "nickname": req.Nickname})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PUT /api/admin/v1/users/{id}/verification {verified} — flip the
// email_verified flag (blessing 用户管理「验证」列).
func (b *Backend) handleUserSetVerification(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var req struct {
		Verified bool `json:"verified"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
		return
	}
	if err := b.Store.SetEmailVerified(r.Context(), id, req.Verified); err != nil {
		writeStoreErr(w, err, "用户不存在", "该邮箱已被其他账号使用")
		return
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_verification_update", map[string]any{"target": id, "verified": req.Verified})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// PUT /api/admin/v1/users/{id}/permission {permission} — change the legacy
// blessing permission. Three guards (archive §6.2, blessing source-verified):
//   - only strictly-lower-permission targets (inlineGuard, incl. no self)
//   - granting ADMIN (1) requires the actor to be SUPER_ADMIN (2)
//   - value must be one of -1 (banned) / 0 (normal) / 1 (admin) — the
//     blessing UI never grants 2 (setup-created only), neither do we
//
// The X-Admin-Token machine path (actorUID == 0) is refused outright: a
// shared fallback credential must not be able to reshape the permission
// surface. Session-authenticated admins only.
func (b *Backend) handleUserSetPermission(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var req struct {
		Permission *int `json:"permission"`
	}
	if err := jsonDecode(r, &req); err != nil || req.Permission == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	perm := *req.Permission
	if perm < -1 || perm > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "permission 仅支持 -1（封禁）/0（正常）/1（管理员）"})
		return
	}
	actor := actorUID(r)
	if actor == 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "权限变更需管理员会话（不支持 X-Admin-Token）"})
		return
	}
	if msg := b.inlineGuard(r, id); msg != "" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": msg})
		return
	}
	// Guard 2: granting ADMIN requires SUPER_ADMIN (permission = 2).
	if perm == 1 {
		actorPerm, err := b.Store.UserPermission(r.Context(), actor)
		if err != nil || actorPerm != 2 {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "授予管理员权限需要超级管理员（permission=2）账号操作"})
			return
		}
	}
	if err := b.Store.SetUserPermission(r.Context(), id, perm); err != nil {
		writeStoreErr(w, err, "用户不存在", "该邮箱已被其他账号使用")
		return
	}
	// A demoted/banned admin's session must die now — the session gate
	// re-checks permission per request, but their tokens/grants linger.
	if perm < 1 {
		_ = b.Store.InvalidateUserTokens(r.Context(), id)
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "admin_user_permission_update", map[string]any{"target": id, "permission": perm})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
