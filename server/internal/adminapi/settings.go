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
	// Union（blessing yggdrasil-connect 原名；union_enabled 为本项目新增键
	// ——老站用插件启停表达，无 option 行。热切换 2026-10-24 拍板）
	{"union_enabled", false, "Union 联盟模块开关（热切换：关=全路由 404、签名回本地密钥）"},
	{"union_api_root", false, "Union hub API 根（默认 https://skin.mualliance.ltd/api/union，立即生效）"},
	{"union_member_key", false, "Union 成员密钥（联盟下发；切勿泄露，立即生效）"},
	{"union_enable_update", true, "允许 hub 远程轮换成员密钥（updatebackendkey，重启后生效）"},
	{"union_enable_oauth2", true, "Union OAuth2 联合登录（重启后生效）"},
	{"union_queryemail_mode", true, "Union 查询邮箱模式：full/off（重启后生效）"},
	{"union_oauth2_sig_private_key", true, "Union OAuth2 用户信息签名私钥（PEM；留空首启自动生成，重启后生效）"},
	{"union_oauth2_sig_public_key", true, "Union OAuth2 用户信息签名公钥（PEM；留空首启自动生成，重启后生效）"},
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
	case "invite_required", "require_verification", "union_enabled", "union_enable_update", "union_enable_oauth2":
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
	case "union_api_root":
		// same shape gate as union.Config.Validate: absolute http(s) URL
		if val != "" {
			if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "union_api_root 需为 http(s) 绝对地址"})
				return
			}
		}
	case "union_queryemail_mode":
		switch val {
		case "full", "off":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "查询邮箱模式需为 full/off"})
			return
		}
	case "union_oauth2_sig_private_key", "union_oauth2_sig_public_key":
		// PEM sanity only (full parse happens at boot; restart-marked key)
		if val != "" && (!strings.Contains(val, "-----BEGIN ") || !strings.Contains(val, "-----END ")) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "需为 PEM 格式密钥（或留空自动生成）"})
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
	if b.OnSettingChanged != nil {
		b.OnSettingChanged(req.Key, val) // hot-apply (union keys)
	}
	// Credential discipline: never persist secret values to the audit log —
	// record shape only. (union_member_key is a shared hub credential and the
	// sig private key signs user-info; their VALUES stay in the options row.)
	auditVal := val
	switch req.Key {
	case "union_member_key", "union_oauth2_sig_private_key":
		auditVal = "<redacted>"
	}
	b.Store.InsertAudit(r.Context(), actorUID(r), "settings_update", map[string]any{
		"key": req.Key, "value": auditVal,
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
