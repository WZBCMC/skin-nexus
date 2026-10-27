package adminapi

import (
	"context"
	"errors"
	"net/http"

	"skin-nexus/server/internal/plugin"
)

// PluginsDeps wires the plugin host into the admin surface. Mounting is
// optional: without it the /plugins routes do not exist (zero surface).
type PluginsDeps struct {
	Host *plugin.Manager
}

// actorPermission resolves an admin actor's legacy permission. Overridable in
// tests (same seam style as sessionAdmin) because *store.Mysql is concrete.
var actorPermission = func(ctx context.Context, b *Backend, uid int64) (int, error) {
	return b.Store.UserPermission(ctx, uid)
}

// insertAudit records plugin mutations; overridable in tests.
var insertAudit = func(ctx context.Context, b *Backend, actor int64, action string, detail map[string]any) {
	b.Store.InsertAudit(ctx, actor, action, detail)
}

// MountPlugins registers the plugin management routes.
func (b *Backend) MountPlugins(deps PluginsDeps) {
	b.plugins = &deps
}

func (b *Backend) handlePluginsList(w http.ResponseWriter, r *http.Request) {
	list := b.plugins.Host.List()
	if list == nil {
		list = []plugin.Status{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": list})
}

// handlePluginAction serves enable/disable/reload. Enabling executes third
// party code in-process, so it mirrors handleUserSetPermission: session
// SUPER_ADMIN (permission=2) only, and the X-Admin-Token machine path is
// refused outright.
func (b *Backend) handlePluginAction(w http.ResponseWriter, r *http.Request) {
	actor := actorUID(r)
	if actor == 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "插件操作需管理员会话（不支持 X-Admin-Token）"})
		return
	}
	perm, err := actorPermission(r.Context(), b, actor)
	if err != nil || perm != 2 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "插件操作需要超级管理员（permission=2）账号"})
		return
	}
	name := r.PathValue("name")
	action := r.PathValue("action")

	var opErr error
	switch action {
	case "enable":
		opErr = b.plugins.Host.Enable(r.Context(), name)
	case "disable":
		opErr = b.plugins.Host.Disable(r.Context(), name)
	case "reload":
		opErr = b.plugins.Host.Reload(r.Context(), name)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown action"})
		return
	}

	detail := map[string]any{"plugin": name}
	if opErr != nil {
		detail["error"] = opErr.Error()
	}
	insertAudit(r.Context(), b, actor, "plugin_"+action, detail)

	switch {
	case errors.Is(opErr, plugin.ErrPluginNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "plugin not found"})
	case opErr != nil:
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": opErr.Error()})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
