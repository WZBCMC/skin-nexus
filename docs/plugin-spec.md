# SkinNexus 插件创作规范（plugin-spec）—— 宿主 v2

> 面向想为 SkinNexus 写插件的人（包括让 AI 代劳）。**零构建原则**：插件 = `plugin.json` +
> 一个入口 JS（可选静态 UI 目录），不需要 Node/打包/编译；放入 `<data>/plugins/<name>/`
> 后由管理面板启用即生效。
>
> 宿主实现：`server/internal/plugin/`（校验规则以代码为准，本规范与代码逐项对齐）。
> 当前只有**单一插件 API 版本**（`hookApi: 2`）。项目尚无存量第三方生态，不维护旧版兼容层。

## 1. 信任模型与安全边界

- **只装你信任的插件**。插件代码在**服务进程内**的 goja VM 中执行：没有网络、没有文件系统、没有定时器、
  没有 `require`，但**不是沙箱**——它能通过声明的 hook/服务影响响应内容，且与宿主共享进程内存。
- v2 的配置、数据、UI、路由全部走**宿主托管服务**：插件不直接接触 MySQL、宿主表、任意文件或任意网络。
- 启用/停用/重载/配置修改**仅超级管理员（permission=2）网页会话**可操作；`X-Admin-Token` 机器通道一律拒绝。
- 插件默认 **disabled**：放入目录不会自动执行，必须显式启用。
- 所有启停/重载/配置修改写审计（`plugin_enable` / `plugin_disable` / `plugin_reload` /
  `plugin_config_update` / `plugin_guards_bypass`）。
- UI 第三方代码只能运行在 sandbox iframe 中（不带 `allow-same-origin`）；不要尝试把插件 HTML 直接注入主 SPA。

## 2. 安装 / 启用 / 停用 / 重载

```
<data>/plugins/<name>/
  plugin.json       # 必需：清单
  index.js          # 必需：入口（文件名由 entrypoint 指定）
  ui/               # 可选：声明为 ui.assets 的静态资源
```

- 放入目录后**重载或重启**才会被发现（宿主进程内一次加载）。
- 管理面板「插件」页可启用/停用/重载、编辑配置、查看健康/路由/最近日志。
- 修改入口 JS 或 UI 资源后必须**重载**（或重启）才生效。
- 推荐安装方式：上传/市场安装**签名 bundle zip**（含 `plugin.json`、`index.js`、`market.json`、`bundle.json`）。
  宿主校验 Ed25519 签名 + 内容摘要后，staging 解压 → 原子替换 → 默认 disabled；
  升级保留启停状态并保留上一版用于回滚。未签名包一律拒绝。
- 作者打包/签名/发布 registry 的完整流程见 [`plugin-publishing.md`](plugin-publishing.md)。
- 安装状态在 `<data>/plugins/.state/`；手动放目录仍可用于开发，但不会获得签名校验/回滚。

### 2.1 签名 bundle 格式

```text
player-notes_1.2.0.zip
  plugin.json          # hookApi=2 清单
  index.js
  ui/...               # 可选
  bundle.json          # {"name","version","keyId","signature"}
```

`bundle.json` 的 Ed25519 签名覆盖除它自身外的全部 zip 条目内容摘要（按路径排序）；
`dist.sha256`（市场）覆盖 zip 字节，`bundle.json` 签名覆盖解压后的内容。两层都必须通过。
管理员在「插件市场」上传/安装；未签名、签名错误、name/version 不一致、zip 越界一律拒绝。
升级保留原启停状态，并保留上一版用于「回滚」；卸载默认保留插件数据。

## 3. plugin.json 字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `name` | ✅ | 唯一 ID，必须等于目录名；`^[a-z0-9][a-z0-9_-]{1,63}$` |
| `version` | ✅ | 字符串，≤64 字符 |
| `hookApi` | ✅ | 必须为 `2`（单一 API 版本） |
| `entrypoint` | ✅ | 目录内单个 `.js` 文件名（拒绝 `..`/绝对路径/符号链接） |
| `permissions` | ✅ | 声明的能力数组（见 §4）；未知权限直接拒绝加载 |
| `runtime` | ⬜ | v2 目前只接受 `goja`（native/sidecar 为后续路线） |
| `minServer` | ⬜ | 最低宿主版本，`>=x.y[.z]` / `<x.y` 组合；不满足则标记错误 |
| `title` / `author` / `homepage` / `description` | ⬜ | 展示元数据 |
| `config` | ⬜ | v2 配置 schema（见 §5） |
| `store` | ⬜ | v2 命名空间数据声明（见 §6） |
| `routes` | ⬜ | v2 路由声明（见 §7） |
| `ui` | ⬜ | v2 页面/插槽/资源/i18n（见 §8） |
| `requires` / `conflicts` | ⬜ | P2 依赖解析预留；当前解析但不执行 |

上限：脚本 ≤1 MiB、manifest ≤64 KiB、单插件路由 ≤32、全局路由 ≤64、插件总数 ≤64、UI 页面/插槽各 ≤32。

## 4. 权限声明

权限是**声明式 + 加载时校验**：注册某个 hook/服务却未声明对应权限，插件加载失败并显示原因。

| 权限 | 允许 |
|---|---|
| `profiles:read` | `onUserProfileFilter` |
| `textures:read` | `onTextureURLFilter` |
| `textures:write` | `onTextureUploaded` |
| `auth:hooks` | `onUserLifecycle`、`onAuthPhase` |
| `http:routes` | public 路由 |
| `http:routes:session` | v2 session 路由 |
| `http:routes:admin` | v2 admin 路由 |
| `http:routes:superadmin` | v2 superadmin 路由 |
| `config` | `skin.config` |
| `store:kv` | `skin.store.kv` |
| `store:docs` | `skin.store.collection`、`onInstall`/`onUpgrade` |
| `ui:page` / `ui:slot` / `ui:assets` | UI 页面 / 插槽 / 静态资源 |
| `events:subscribe` | `skin.events.on` |
| `auth:guards` | `skin.guards.on` |
| `net:http` / `mail:send` / `schedule:jobs` / `cli:commands` | P2 预留 |
| `plugins:read` | `skin.plugins.list()`（P1 只读内省） |

## 5. 配置（`skin.config`）

`config.schema` 是 JSON Schema 子集：

```json
{
  "type": "object",
  "required": ["token"],
  "properties": {
    "message": { "type": "string", "default": "hello" },
    "count": { "type": "integer", "minimum": 1, "maximum": 10, "default": 1 },
    "mode": { "type": "string", "enum": ["a", "b"], "default": "a" },
    "token": { "type": "string", "format": "secret" }
  }
}
```

- 支持 `type`（object/string/integer/number/boolean/array）、`properties`、`required`、`default`、
  `enum`、`minimum`/`maximum`、`format: "secret"`、`x-ui`（text/textarea/select/checkbox/password/number/list）。
- 未知字段/类型不符/越界/缺必填会在保存时返回 400；加载时配置非法会把插件标记为 error。
- `format: "secret"` 的字段**加密存储、后台只显示掩码**；不要给 secret 声明 `default`。
- 运行时读取：`skin.config.get(key)`、`skin.config.all()`；配置修改立即生效，无需重载。

## 6. 命名空间数据（`skin.store`）

插件**不能**直接访问数据库或宿主表；只能使用自己声明的命名空间。

```json
"store": {
  "schemaVersion": 1,
  "collections": { "notes": { "maxRows": 1000, "maxBytes": 65536 } }
}
```

```js
skin.store.kv.set("last", "value", { ttlMS: 86400000 });
skin.store.kv.get("last", "default");
skin.store.collection("notes").put("id1", { text: "hello" });
skin.store.collection("notes").list({ prefix: "", limit: 100 });
```

- KV：单值 ≤64 KiB，每插件 ≤1000 keys；支持 TTL。
- 集合：单文档 ≤256 KiB（且受 `maxBytes` 约束），每集合 ≤1 万行（且受 `maxRows` 约束）。
- 宿主托管迁移：`schemaVersion` 递增时，在启用/重载时调用
  `skin.store.onInstall(fn)`（0→N）或 `skin.store.onUpgrade(fn)`（旧→新，`fn` 收到 `{from,to}`）；
  迁移失败会关闭 VM 并把插件标为 error。迁移只能调用 `skin.config` / `skin.store`。
- 禁用不删数据；删除目录即卸载，数据默认保留（P2 提供清理入口）。

## 7. 路由（`skin.route` + manifest `routes`）

所有插件 API 在 `/api/plugins/<name>/...` 下。**manifest 是权限与策略的唯一来源**，运行时注册必须与声明一致：

```json
"routes": [
  { "method": "GET",  "path": "/notes",      "tier": "session" },
  { "method": "POST", "path": "/notes",      "tier": "session", "bodyLimit": 16384, "csrf": true },
  { "method": "GET",  "path": "/notes/{id}", "tier": "session" },
  { "method": "GET",  "path": "/files/{rest...}", "tier": "public" }
]
```

```js
skin.route("GET", "/notes", function (req) {
  return { items: [], user: req.user.id };
});
```

| 档位 | 鉴权 | CSRF | 注入 |
|---|---|---|---|
| `public` | 无 | 不适用 | 无 |
| `session` | `skin_session` | 非 GET 必须 | `req.user = {id,nickname,permission,admin}` |
| `admin` | permission > 0 | 非 GET 必须 | 同上 |
| `superadmin` | permission = 2 | 非 GET 必须 | 同上 |

- 请求对象：`{method, path, params, query, body?, user?, ip, headers}`；**不含 cookies**。
- 只读安全 header 子集（content-type/accept-language/user-agent/origin/x-request-id）。
- 路径参数：`{id}` 单段、`{rest...}` 末尾捕获；禁止 `..` 与 `/_csrf`（宿主保留）。
- body 默认 64 KiB，最大 1 MiB；响应 1 MiB；每路由可声明 `rateLimit`（如 `"60/m"`）。
- CSRF：`GET /api/plugins/<name>/_csrf` 获取 token（同时设置 `skin_csrf` cookie）；
  非 GET 的 session/admin 路由必须带 `X-CSRF-Token`。iframe bridge 会自动处理。
- 错误体：宿主错误统一 `{error, message, code}`；插件返回的 JSON 原样透传。
- 所有路由必须先在 manifest `routes` 中声明，未声明即加载失败。

## 8. UI（`ui`）

v2 的 UI 采用**固定插槽 + iframe/schema 页面**，不允许第三方直接注入 Vue 组件或任意 HTML。

```json
"ui": {
  "assets": "ui/",
  "pages": [{
    "id": "notes", "path": "/ext/player-notes/notes", "area": "user",
    "nav": { "label": "Notes" },
    "render": { "type": "iframe", "src": "notes.html" },
    "perm": 0
  }],
  "slots": [{
    "id": "skinlib.detail.sidebar",
    "render": { "type": "iframe", "src": "slot.html" },
    "height": 180
  }],
  "i18n": { "zh-CN": { "title": "笔记" }, "en": { "title": "Notes" } }
}
```

固定 slot ID：`app.topbar.after`、`app.nav.user`、`app.nav.admin`、`auth.login.after`、
`dashboard.cards`、`skinlib.list.sidebar`、`skinlib.detail.sidebar`、`profile.card.footer`、
`player.card.footer`、`footer.before`。

- 页面落在 `/ext/<name>/<page>`；服务端只对已启用插件的已声明页面返回 SPA shell，未知路径仍 404。
- `iframe` 页面/插槽从 `/plugin-assets/<name>/...` 加载，`sandbox="allow-scripts allow-forms"`；
  **不带 `allow-same-origin`**，iframe 无法读取宿主 cookie/localStorage。
- iframe bridge（`postMessage`，信封 `{v:1, nonce, id, type, payload}`）：
  - 宿主→frame：`init`（plugin/src/origin/locale/user/csrf）；
  - frame→宿主：`ready`、`resize`、`toast`、`navigate`、`confirm`、`request`。
  - `request` 由宿主代发到该插件路由，自动带 cookie/CSRF，返回 `{status, body}`。
  - iframe 首次 `ready` 可用 `targetOrigin: "*"`（此时 origin 为 opaque）；之后用 init 里的 `origin`。
- `schema` 页面/插槽预留给宿主表单渲染；P1 的 schema 页面显示“暂未开放”，配置表单已由 `SchemaForm` 支持。
- 主题 `slots` 与插件 slot 独立；主题仍是管理员信任的 CSS/JS 面。

## 9. 事件与 guard

```js
skin.events.on("player.added", function (e) { /* e.playerId/e.playerName/e.userId/e.ip */ });
skin.guards.on("user.registration.check", function (e) {
  if (blocked(e.data.email)) return { allow: false, code: "x.blocked", message: "不允许注册" };
  return { allow: true };
}, { onError: "deny", timeoutMS: 50 });
```

- 事件目录：`user.registered`、`user.logged_in`、`user.deleted`、`player.added`、`player.renamed`、
  `player.deleted`、`profile.updated`、`texture.uploaded`、`texture.deleted`。事件异步、有界队列，失败 fail-open。
- guard 目录：`user.registration.check`、`auth.login.check`、`auth.join.check`、`texture.upload.check`、
  `texture.delete.check`。guard 同步执行，必须显式 `onError: "deny"|"allow"`；超时/异常按该策略处理。
- guard 不接收密码/cookie；deny 返回通用 403（错误体带插件提供的 message/code）。
- 超管可在管理面板开启 `plugin_guards_bypass` 破窗（审计 + 横幅）；仅用于插件故障救急。

## 10. 基础 hook

```js
skin.hooks.onUserProfileFilter(fn, { timeoutMS })
skin.hooks.onTextureURLFilter(fn, { timeoutMS })
skin.hooks.onTextureUploaded(fn)
skin.hooks.onUserLifecycle(fn)
skin.hooks.onAuthPhase(fn)
skin.log(...args)
```

返回语义：profile 返回 `{SKIN,CAPE}` 覆盖（空字符串删除、undefined 不改）；texture 返回字符串；
事件返回值忽略；所有 hook 超时/异常 fail-open。过滤器严格在 `AssembleTextures` 签名之前执行。

## 11. 示例

`examples/plugins/` 提供 6 个离线示例：

- `texture-cdn`：纹理 URL 改写；
- `lifecycle-log`：生命周期日志；
- `config-greeter`：v2 配置 schema + 公开路由；
- `nav-links`：v2 UI 页面 + 用户导航 + sandbox iframe；
- `signup-guard`：v2 fail-closed 注册 guard；
- `player-notes`：v2 session 路由 + CSRF + 命名空间集合 + UI 页面。

复制整个目录到 `<data>/plugins/` 后在管理面板启用即可。v2 示例需要宿主包含 v2 插件平台。

## 12. 非目标与后续路线

- **P2 Core / v0.3.0（当前）**：签名 bundle 安装/升级/卸载/回滚、单源签名市场、依赖/冲突检查、能力 consent、后台市场页、`skin-plugin` 发布工具链。
- **后续按需**：多 registry、密钥轮换、插件级 HTTP/mail/cron/CLI、主题 v2、sidecar/native、评分/评论。
- **不做**：插件直接访问数据库/宿主表/任意文件系统/任意网络；不允许第三方 Vue/Twig/HTML 注入主 SPA。
- **兼容策略**：项目尚无存量第三方生态，只维护 `hookApi: 2`；不提供旧版/blessing 插件兼容层。

## 13. 给 AI 的提示词模板

> 为 SkinNexus 插件宿主 v2 写一个插件：目录 `<name>/` 含 `plugin.json`（必要时加 `ui/` 与 `index.js`）。
> 约束：`hookApi=2`；只声明必需权限（config / store:kv / store:docs / http:routes:session 等）；
> 数据只用 `skin.store`，配置只用 `skin.config`，路由必须与 manifest `routes` 完全一致；
> UI 页面/插槽用 iframe + postMessage bridge，不注入主 SPA；guard 必须显式 `onError`；
> 过滤器/事件 fail-open，不改变 Yggdrasil 的 204/错误体/签名顺序。产物需通过 manifest 校验并在管理面板启用。
