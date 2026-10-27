# SkinNexus 插件创作规范（plugin-spec）—— goja 宿主 v1

> 面向想为 SkinNexus 写插件的人（包括让 AI 代劳）。**零构建原则**：插件 = `plugin.json` + 一个入口 JS，
> 不需要 Node/打包/编译；放入 `<data>/plugins/<name>/` 后由管理面板启用即生效。
> 宿主实现：`server/internal/plugin/`（校验规则以代码为准，本规范与代码逐项对齐）。

## 1. 信任模型与安全边界

- **只装你信任的插件**。插件代码在**服务进程内**的 goja VM 中执行：没有网络、没有文件系统、没有定时器、
  没有 `require`，但**不是沙箱**——它能通过声明的 hook 影响响应内容，且与宿主共享进程内存。
- v1 安装方式 = 管理员手动把目录放进 `<data>/plugins/`；**没有上传/市场**（后置）。
- 启用/停用/重载**仅超级管理员（permission=2）网页会话**可操作；`X-Admin-Token` 机器通道一律拒绝。
- 插件默认 **disabled**：放入目录不会自动执行，必须显式启用。
- 所有启停/重载写审计（`plugin_enable` / `plugin_disable` / `plugin_reload` / `plugin_loaded` / `plugin_load_failed`）。

## 2. 安装 / 启用 / 停用 / 重载

```
<data>/plugins/<name>/
  plugin.json     # 必需：清单
  index.js        # 必需：入口（文件名由 entrypoint 指定）
```

- 放入目录后**重载或重启**才会被发现（宿主进程内一次加载，不像 blessing 每请求执行 bootstrap）。
- 管理面板「插件」页可启用/停用/重载；停用立即从 hook 与路由摘除，无需重启。
- 修改入口 JS 后必须**重载**（或重启）才生效。
- v1 无卸载概念：删除目录即卸载；插件不持有数据库，故无数据残留问题。

## 3. plugin.json 字段

| 字段 | 必填 | 说明 |
|---|---|---|
| `name` | ✅ | 唯一 ID，必须等于目录名；`^[a-z0-9][a-z0-9_-]{1,63}$` |
| `version` | ✅ | 字符串，≤64 字符 |
| `hookApi` | ✅ | 必须等于 `1`（宿主只支持当前主版本） |
| `entrypoint` | ✅ | 目录内单个 `.js` 文件名（拒绝 `..`/绝对路径/符号链接） |
| `permissions` | ✅ | 声明的权限数组（见 §4）；未知权限直接拒绝加载 |
| `minServer` | ⬜ | 最低宿主版本，`>=x.y` 语义 |
| `title` | ⬜ | 展示名 ≤128（管理面板优先显示） |
| `author` | ⬜ | ≤64 |
| `homepage` | ⬜ | http(s) URL ≤256 |
| `description` | ⬜ | ≤512 |

上限：脚本 ≤1 MiB、manifest ≤64 KiB、单插件路由 ≤32、全局路由 ≤64、插件总数 ≤64。

## 4. 权限声明与映射

权限是**声明式 + 加载时校验**：注册某个 hook 却未声明对应权限，插件加载失败并显示原因。

| 权限 | 允许注册 |
|---|---|
| `profiles:read` | `onUserProfileFilter` |
| `textures:read` | `onTextureURLFilter` |
| `textures:write` | `onTextureUploaded` |
| `auth:hooks` | `onUserLifecycle`、`onAuthPhase` |
| `http:routes` | `skin.route` |
| `admin:menu` | 预留（v1 仅审计，不生效） |

## 5. JS API

全局对象 `skin`：

```js
skin.hooks.onUserProfileFilter(fn, { timeoutMS })
skin.hooks.onTextureURLFilter(fn, { timeoutMS })
skin.hooks.onTextureUploaded(fn)
skin.hooks.onUserLifecycle(fn)
skin.hooks.onAuthPhase(fn)
skin.route("GET", "/path", fn)
skin.log(...args)
```

事件形状：

| hook | 入参 |
|---|---|
| onUserProfileFilter | `{playerId, playerName, uuid, textures:{SKIN,CAPE}}` |
| onTextureURLFilter | `{hash, url, type}`（type = `skin` \| `cape`） |
| onTextureUploaded | `{userId, playerId, hash, type}` |
| onUserLifecycle | `{kind, userId}`（kind = `registered` \| `logged_in` \| `deleted`） |
| onAuthPhase | `{phase, uuid, player}`（phase = `pre_join` \| `post_has_joined`） |
| skin.route | `{method, path, query}`（**不含 headers/cookies**） |

返回语义：
- profile filter：返回 `{SKIN, CAPE}` 覆盖（空字符串=删除该纹理，`undefined`=不改）。
- texture filter：返回字符串=新 URL，`undefined`=不改。
- 事件 hook：返回值被忽略（fire-and-forget）。
- 路由：返回任意 JSON 值，或 `{status, body}` 指定状态码；仅 GET/HEAD；响应 ≤1 MiB。

`timeoutMS` 只能**收紧**不能放宽：超过该类上限（普通 hook 100ms、auth 类 50ms、路由 1s）会被截断。

## 6. 过滤器与签名顺序（重要）

Yggdrasil 的 textures property 是**先过滤、后签名**：插件改写 URL 后，签名覆盖改写后的值，客户端校验通过。
**不要**假设能在签名后改 URL——那会让客户端拒收。profile/texture 过滤器只影响 URL 字符串，不改 UUID、不改 204 语义。

## 7. 超时 / 错误 / fail-open

- 任何 hook 超时、抛异常、panic 都**fail-open**：保留原值/原响应，绝不改变状态码（尤其 `hasJoined`/`join` 的 204 与错误体）。
- 事件（upload/lifecycle/auth）异步执行，队列满时丢弃，不阻塞请求。
- 最近一次错误显示在管理面板「最近错误」列（截断 512 字节）。

## 8. 路由规则

- 前缀固定 `/api/plugins/<name>/`，插件只能注册该前缀下的子路径。
- 仅 GET/HEAD；路径需 clean、禁止 `..`；同插件内 method+path 不可重复。
- 路由是**公开**的（无会话/cookie 注入）：不要通过插件路由暴露敏感数据。
- 插件停用后其路由立即 404。

## 9. 示例

仓库 `examples/plugins/` 提供两个离线示例：

- `texture-cdn`：把 skin 纹理 URL 改写到 CDN 镜像（改 `index.js` 里的 `CDN_BASE`）。
- `lifecycle-log`：把注册/登录/删除与 join/hasJoined 阶段写入服务日志，用作接线自检。

复制整个目录到 `<data>/plugins/` 后在管理面板启用即可。

## 10. 与 blessing 插件模型对照

| 能力 | blessing-skin-server | SkinNexus v1 |
|---|---|---|
| 语言/运行时 | PHP（Laravel 容器，同进程全信任） | JS（goja VM，无网络/文件/定时器） |
| 入口 | `bootstrap.php`（每请求执行） | `index.js`（进程内一次加载，reload 生效） |
| 清单 | `package.json`（name/version/title/namespace/require/enchants） | `plugin.json`（name/version/hookApi/permissions/entrypoint + 可选元数据） |
| 扩展点 | 事件 + filter + 路由 + 视图 + 菜单 + 中间件 + DB | 6 个 hook + 只读路由 |
| 配置 | `Option::form` 生成管理表单 | v1 无（v1.1 计划） |
| 权限 | 无（全信任） | 声明式权限 + 加载校验 + 超管启停 |
| 失败语义 | 监听器异常可能影响请求 | 全 fail-open + 超时中断 |
| 依赖/冲突 | `require` + `enchants.conflicts`（semver） | v1 无（v1.2 随市场） |

## 11. 非目标与后续路线

- **v1.1**：插件配置 schema + options 持久化 + 生成管理表单（blessing 生态最高频诉求）；视需求引入软否决（veto）hook。
- **v1.2**：安装/卸载生命周期与数据保留策略、插件间依赖/冲突解析、市场元数据（icon/license/checksum/签名）与分发。
- **v2**：前端扩展点（页面级资源注入、事件总线、管理菜单/widget）。
- **不做**：插件直接访问数据库（与无沙箱叠加风险过大）。

## 12. 给 AI 的提示词模板

> 为 SkinNexus（Go + goja 插件宿主）写一个插件：目录 `<name>/` 含 `plugin.json` 与 `index.js`。
> 约束：hookApi=1；只声明必需权限（profiles:read / textures:read / textures:write / auth:hooks / http:routes）；
> 只用 `skin.hooks.*`、`skin.route`、`skin.log`；无网络/文件/定时器/require；过滤器 fail-open、不改 UUID 与 204 语义；
> 路由仅 GET/HEAD 且公开；超时默认 100ms（auth 50ms）不可放宽。产物需能通过 `plugin.json` 校验并在管理面板启用。
