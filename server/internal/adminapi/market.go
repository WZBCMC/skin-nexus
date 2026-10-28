package adminapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"skin-nexus/server/internal/plugin"
)

// sameOriginForAdmin is the P2 market CSRF gate: a browser mutation must carry
// an Origin matching Host (or Sec-Fetch-Site: same-origin). The super-admin
// session is still required; X-Admin-Token is rejected by pluginSuper.
func sameOriginForAdmin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}
	return false
}

func (b *Backend) pluginSuper(w http.ResponseWriter, r *http.Request) (int64, bool) {
	actor := actorUID(r)
	if actor == 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "该操作仅支持超级管理员网页会话"})
		return 0, false
	}
	perm, err := actorPermission(r.Context(), b, actor)
	if err != nil || perm != 2 {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "需要超级管理员（permission=2）账号"})
		return 0, false
	}
	if !sameOriginForAdmin(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "请求来源校验失败"})
		return 0, false
	}
	return actor, true
}

func (b *Backend) handlePluginRollback(w http.ResponseWriter, r *http.Request) {
	actor, ok := b.pluginSuper(w, r)
	if !ok {
		return
	}
	ctx := plugin.WithActor(r.Context(), actor)
	if err := b.plugins.Host.Rollback(ctx, r.PathValue("name")); err != nil {
		if errors.Is(err, plugin.ErrNoRollback) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "没有可回滚的版本"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (b *Backend) handlePluginUninstall(w http.ResponseWriter, r *http.Request) {
	actor, ok := b.pluginSuper(w, r)
	if !ok {
		return
	}
	purge := r.URL.Query().Get("purge") == "1"
	ctx := plugin.WithActor(r.Context(), actor)
	if err := b.plugins.Host.Uninstall(ctx, r.PathValue("name"), purge); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (b *Backend) handleMarketCatalog(w http.ResponseWriter, r *http.Request) {
	list, err := b.plugins.Host.MarketCatalog(r.Context(), r.URL.Query().Get("refresh") == "1")
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": list})
}

func (b *Backend) handleMarketPlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := jsonDecode(r, &body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name required"})
		return
	}
	plan, err := b.plugins.Host.MarketPlan(r.Context(), body.Name)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (b *Backend) handleMarketInstall(w http.ResponseWriter, r *http.Request) {
	actor, ok := b.pluginSuper(w, r)
	if !ok {
		return
	}
	var body struct {
		Name    string `json:"name"`
		Consent bool   `json:"consent"`
	}
	if err := jsonDecode(r, &body); err != nil || body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name required"})
		return
	}
	ctx := plugin.WithActor(r.Context(), actor)
	res, err := b.plugins.Host.MarketInstall(ctx, body.Name, body.Consent)
	switch {
	case errors.Is(err, plugin.ErrConsentRequired), errors.Is(err, plugin.ErrInstallBlocked):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "plan": res.Plan})
	case err != nil:
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

func (b *Backend) handleMarketUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := b.pluginSuper(w, r)
	if !ok {
		return
	}
	maxUpload := b.plugins.Host.UploadMax()
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "multipart parse failed"})
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file required"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil || int64(len(data)) > maxUpload {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "bundle too large"})
		return
	}
	consent := r.FormValue("consent") == "1"
	ctx := plugin.WithActor(r.Context(), actor)
	if r.FormValue("inspect") == "1" {
		insp, ierr := b.plugins.Host.InspectBytes(ctx, data)
		if ierr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": ierr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, insp)
		return
	}
	opts := plugin.InstallOptions{
		Consent:          consent,
		Source:           "upload",
		AcceptUnverified: r.FormValue("acceptUnverified") == "1",
		ExpectedZipSHA:   strings.TrimSpace(r.FormValue("expectDigest")),
	}
	res, err := b.plugins.Host.InstallBytes(ctx, data, opts)
	switch {
	case errors.Is(err, plugin.ErrUnverifiedPlugin), errors.Is(err, plugin.ErrDigestMismatch):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": err.Error(), "inspection": res.Inspection,
		})
	case errors.Is(err, plugin.ErrConsentRequired), errors.Is(err, plugin.ErrInstallBlocked):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": err.Error(), "plan": res.Plan, "inspection": res.Inspection,
		})
	case err != nil:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
