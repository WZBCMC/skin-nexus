package adminapi

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
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
	{"rate_limit_auth", true, "认证端点每 IP 每 10 分钟请求数上限（0=关闭；重启后生效）"},
	{"login_fail_limit", true, "同账号连续失败锁定阈值（0=关闭；重启后生效）"},
	// B4 (blessing 原名 — 迁移导入器原样搬运生产 options，改名会让存量值成死键)
	{"announcement", false, "站点公告（纯文本，用户中心公告卡显示，留空隐藏）"},
	{"regs_per_ip", false, "每 IP 可注册账号数上限（0=不限制，立即生效）"},
	{"max_upload_file_size", false, "材质上传大小上限（KB，不超过服务端硬顶，立即生效）"},
	{"player_name_rule", false, "角色名规则：official/cjk/utf8/custom（仅拦新写入，立即生效）"},
	{"custom_player_name_regexp", false, "自定义角色名正则（player_name_rule=custom 时生效）"},
	{"player_name_length_min", false, "角色名最小长度（字符）"},
	{"player_name_length_max", false, "角色名最大长度（字符）"},
	{"texture_name_regexp", false, "材质备注命名正则（空=不限制，仅拦新上传）"},
	// B5（blessing yggdrasil-connect 原名，callbacks.php:93-98 默认值）
	{"ygg_token_expire_1", false, "ygg 令牌软窗口（秒，默认 259200=3 天）：validate/join 有效期，过后可 refresh 换新"},
	{"ygg_token_expire_2", false, "ygg 令牌硬窗口（秒，默认 604800=7 天）：refresh 有效期，过后须重新登录"},
	{"ygg_tokens_limit", false, "每用户 ygg 令牌数上限（默认 10，0=不限）：超限淘汰最旧令牌并留审计"},
	{"ygg_search_profile_max", false, "批量查角色单次上限（默认 5，1-100）"},
	{"ygg_rate_limit", false, "同名认证最小间隔（毫秒，默认 1000，0=关闭）"},
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
		normalized, ok := normalizeBool(val)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "布尔值需为 1/0/true/false/on/off/yes/no"})
			return
		}
		val = normalized
	case "skin_domains":
		// accept newline/comma separated, store canonical comma CSV
		val = strings.Join(strings.FieldsFunc(val, func(r rune) bool {
			return r == ',' || r == '\n' || r == ' '
		}), ",")
	case "rate_limit_auth", "login_fail_limit", "regs_per_ip", "player_name_length_min", "player_name_length_max",
		"ygg_token_expire_1", "ygg_token_expire_2", "ygg_tokens_limit", "ygg_rate_limit":
		// numeric knobs (review P2: garbage silently desynced panel vs boot
		// fallback, and negatives silently disabled the limiter layers)
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 || n > 1000000 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "数值需为 0-1000000 的整数"})
			return
		}
		val = strconv.Itoa(n)
	case "ygg_search_profile_max":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "批量查询上限需为 1-100 的整数"})
			return
		}
		val = strconv.Itoa(n)
	case "max_upload_file_size":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 65536 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "上传上限需为 1-65536 的整数（KB）"})
			return
		}
		val = strconv.Itoa(n)
	case "player_name_rule":
		switch val {
		case "official", "cjk", "utf8", "custom":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "角色名规则需为 official/cjk/utf8/custom"})
			return
		}
	case "custom_player_name_regexp", "texture_name_regexp":
		// an unparsable regexp would silently fail-closed at every new write
		if val != "" {
			if _, err := regexp.Compile(val); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "正则表达式无效：" + err.Error()})
				return
			}
		}
	case "announcement":
		// plain text only — no markdown/HTML rendering surface (dev plan §7:
		// sanitizer choice stays with Pro); length cap keeps the options
		// TEXT column honest.
		if len(val) > 4096 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "公告最长 4096 字符"})
			return
		}
	}
	if err := b.Store.SetOption(r.Context(), req.Key, val); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "save failed"})
		return
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "settings_update", map[string]any{
		"key": req.Key, "value": val,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "key": req.Key, "value": val})
}

// normalizeBool maps the accepted boolean spellings to "1"/"0"; ok=false on
// unknown input (review P2: typos silently became "0", disabling gates).
func normalizeBool(v string) (string, bool) {
	switch strings.ToLower(v) {
	case "1", "true", "on", "yes":
		return "1", true
	case "0", "false", "off", "no":
		return "0", true
	default:
		return "0", false
	}
}
