package adminapi

import (
	"context"
	"encoding/json"
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

// MountPlugins registers the plugin management routes.
func (b *Backend) MountPlugins(deps PluginsDeps) {
	b.plugins = &deps
}

func (b *Backend) handlePluginsList(w http.ResponseWriter, r *http.Request) {
	list := b.plugins.Host.List()
	if list == nil {
		list = []plugin.Status{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plugins": list, "guardsBypassed": b.plugins.Host.GuardsBypassed(r.Context()),
	})
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

	ctx := plugin.WithActor(r.Context(), actor)
	var opErr error
	switch action {
	case "enable":
		opErr = b.plugins.Host.Enable(ctx, name)
	case "disable":
		opErr = b.plugins.Host.Disable(ctx, name)
	case "reload":
		opErr = b.plugins.Host.Reload(ctx, name)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown action"})
		return
	}

	switch {
	case errors.Is(opErr, plugin.ErrPluginNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "plugin not found"})
	case opErr != nil:
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": opErr.Error()})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func (b *Backend) handlePluginConfigGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	view, err := b.plugins.Host.PluginConfig(name)
	switch {
	case errors.Is(err, plugin.ErrPluginNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "plugin not found"})
	case errors.Is(err, plugin.ErrPluginNoConfig):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "plugin has no config"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, view)
	}
}

type pluginConfigBody struct {
	Values map[string]any `json:"values"`
}

func (b *Backend) handlePluginConfigSave(w http.ResponseWriter, r *http.Request) {
	actor := actorUID(r)
	if actor == 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "配置修改需超级管理员会话（不支持 X-Admin-Token）"})
		return
	}
	perm, err := actorPermission(r.Context(), b, actor)
	if err != nil || perm != 2 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "配置修改需要超级管理员（permission=2）账号"})
		return
	}
	var body pluginConfigBody
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	ctx := plugin.WithActor(r.Context(), actor)
	view, err := b.plugins.Host.SavePluginConfig(ctx, r.PathValue("name"), body.Values)
	switch {
	case errors.Is(err, plugin.ErrPluginNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "plugin not found"})
	case errors.Is(err, plugin.ErrPluginNoConfig):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "plugin has no config"})
	case errors.Is(err, plugin.ErrPluginNoStore), errors.Is(err, plugin.ErrPluginNoSecrets):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case err != nil:
		var ce *plugin.ConfigError
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": ce.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, view)
	}
}

func (b *Backend) handlePluginGuardBypass(w http.ResponseWriter, r *http.Request) {
	actor := actorUID(r)
	if actor == 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "破窗操作需超级管理员会话"})
		return
	}
	perm, err := actorPermission(r.Context(), b, actor)
	if err != nil || perm != 2 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "破窗操作需要超级管理员（permission=2）账号"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	ctx := plugin.WithActor(r.Context(), actor)
	if err := b.plugins.Host.SetGuardsBypassed(ctx, body.Enabled); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"guardsBypassed": body.Enabled})
}
