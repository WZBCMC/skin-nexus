package adminapi

import (
	"context"
	"net/http"
	"strings"
)

// Settings panel (user directive, 2026-10-23): site knobs that used to live
// in env vars move into the admin UI. Request-time keys apply immediately;
// boot-time keys apply after the container restarts (routing/wiring built at
// startup) — the UI shows that per key.

type settingMeta struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Default string `json:"default"` // env-derived fallback (displayed as placeholder)
	Restart bool   `json:"restart"`
	Desc    string `json:"desc"`
}

var settingsWhitelist = []struct {
	key     string
	restart bool
	desc    string
}{
	{"site_name", true, "站点名称（页头/标题）"},
	{"server_name", true, "服务器名称"},
	{"skin_domains", true, "皮肤域名（CSV，ygg metadata 皮肤来源）"},
	{"invite_required", false, "注册强制邀请码（立即生效）"},
	{"require_verification", false, "join 前须邮箱验证（立即生效；邮件流未上线前开了会挡住所有新号进服）"},
}

func (b *Backend) settingValue(ctx context.Context, key, def string) string {
	if v, ok, err := b.Store.GetOption(ctx, key); err == nil && ok {
		return v
	}
	return def
}

// GET /api/admin/v1/settings — whitelist values (option wins over env default).
func (b *Backend) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	out := make([]settingMeta, 0, len(settingsWhitelist))
	for _, s := range settingsWhitelist {
		def := b.SettingsDefaults[s.key]
		out = append(out, settingMeta{
			Key: s.key, Value: b.settingValue(r.Context(), s.key, def),
			Default: def, Restart: s.restart, Desc: s.desc,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": out})
}

// PUT /api/admin/v1/settings {key, value} — whitelist-guarded option write.
func (b *Backend) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	found := false
	for _, s := range settingsWhitelist {
		if s.key == req.Key {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "未知设置项"})
		return
	}
	val := strings.TrimSpace(req.Value)
	switch req.Key {
	case "invite_required", "require_verification":
		val = normalizeBool(val) // "1" / "0"
	case "skin_domains":
		// accept newline/comma separated, store canonical comma CSV
		val = strings.Join(strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == '\n' || r == ' '
		}), ",")
	}
	if err := b.Store.SetOption(r.Context(), req.Key, val); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save failed"})
		return
	}
	b.Store.InsertAudit(r.Context(), 0, "settings_update", map[string]any{
		"key": req.Key, "value": val,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": req.Key, "value": val})
}

func normalizeBool(v string) string {
	switch strings.ToLower(v) {
	case "1", "true", "on", "yes":
		return "1"
	default:
		return "0"
	}
}
