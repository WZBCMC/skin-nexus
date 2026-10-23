# 兼容矩阵 —— blessing / yggdrasil-connect 1:1 复刻进度

> 依据：2026-10-23 对线上 legacy 站点全量只读实测（报文在私有工作区 `analysis/live-capture/`）
> 与 `analysis/` 源码级报告。图例：✅ 达成 / 🔧 实测后修复 / ⚠️ 缺口 / ➖ 不复刻。

## 1. Yggdrasil 面（authlib-injector 硬契约）

| 端点 | legacy 实测行为 | SkinNexus | 状态 |
|---|---|---|---|
| `GET /api/yggdrasil/` | meta + `feature.non_email_login` + `feature.openid_configuration_url` + skinDomains + PEM 公钥 | 同形；skinDomains 按旧站公式（配置 ∪ Union 成员站 ∪ site_url host ∪ 请求 host，2026-10-23 对齐） | 🔧 |
| `authserver/authenticate` | 单角色自动绑定 `selectedProfile`；token 为 JWT | 同形 | 🔧 |
| `authserver/validate` / `invalidate` / `signout` | 恒 204 | 同 | ✅ |
| `authserver/refresh` | 旧 token 立废；支持 `selectedProfile` 选角 | 同（选角解析修复于 2026-10-23） | 🔧 |
| `sessionserver/join` | `selectedProfile` 为**裸 UUID 字符串**；强制 token↔角色绑定（403 中文错误体） | 双形态容错（字符串/对象、32hex/连字符）+ 同款错误面 | 🔧 |
| `sessionserver/hasJoined` | 未命中恒 204；命中出签名 profile | 同（含 IP 比对 fork 语义） | ✅ |
| `sessionserver/profile/{uuid}` | 签名矩阵 | 同 | ✅ |
| `api/profiles/minecraft`（POST）/ `api/users/profiles/minecraft/{name}` | 200 / 未命中 204 | 同 | ✅ |
| `api/profiles/minecraft/{name}` 别名、`minecraftservices/publickeys` | 旧站 404 | 未实现（平价；超集待议） | ➖ |

JWT access token 实测形状：`aud="2"（shared_client_id）、sub=uid、scopes[Select,Server.Join]、selectedProfile=<uuid 字符串>、jti、无 iss`。

## 2. yggc / OIDC 免密面（SJMCL / WZBC-Launcher 契约）

| 环节 | 契约要点 | 状态 |
|---|---|---|
| 发现 | 启动器读 meta `feature.openid_configuration_url` → discovery 取 `shared_client_id`+端点；两者非空才出「OAuth 登录」按钮 | ✅ |
| discovery | shared_client_id、device/token/jwks 端点、RS256、S256、claims 面 | ✅ |
| device 流 | RFC 8628 纯净流、`/device` 页、`authorization_pending/slow_down` | ✅ |
| id_token | RS256 + JWKS keys[0] + aud + `selectedProfile{id,name}`（对象） | ✅ |
| userinfo | GET+Bearer、`sub`=uid 字符串、scope 分面 claims、401 `Bearer realm="openid"` | ✅ |
| token 贯通 | OAuth JWT 直通 ygg join/validate（scope 门 + jti 台账） | ✅ |
| discovery 挂载 | 根路径 + `/api/yggdrasil/.well-known/...`（启动器截断启发式）双挂载 | 🔧 |

## 3. 兼容面

| 端点 | legacy 实测 | SkinNexus | 状态 |
|---|---|---|---|
| `GET /{player}.json`（CustomSkinAPI R1） | 200 | 同形 | ✅ |
| `GET /usm/{player}.json`、`/skin|cape/{player}.png` | 旧站 404（插件未装） | 已实现（超集） | ✅ |

## 4. 功能缺口优先级（blessing 1:1 复刻主干）

- **P1+（首位）**：邀请码模块**基础功能 only**——1:1 移植（基准=线上魔改版）：`invitation_codes(id,code,generated_at,used_by,used_at,description)`（`description`=「姓名+学号」存储约定，迁移直导）；线上语义=单条生成必填 description + edit(id,description) + 双注册 filter（有效码旁路其他注册拒绝）+ `invitation_codes_for_union_enabled` 选项位；SkinNexus 口径：注册默认开放选填、`SKIN_INVITE_REQUIRED` 开时=线上必填语义、码 32-hex md5 同款。**CSV 学号姓名一键发码、邀请码查询页（学号+姓名查未用码+审计日志）→ 插件拓展完成后作为插件引入**（行为规格已取证存档）
- **P1**：衣柜 closet（迁移切流前置）· 改密端点 · 邮箱验证流/找回密码（含 SMTP；join 验证门已配置化 `SKIN_REQUIRE_VERIFICATION`，默认关）· 管理面（用户改密/删除、角色管理、选项面板）· ygg_rate_limit
- **P2**：举报/通知/纹理广场 · OAuth 社交登录 · API PAT 面 · 插件宿主 · 正版绑定 · 纹理删除
- **P3**：`/api` 根装饰 · 验证码 · 积分商城 · 2FA

详细版（含逐端点请求/应答、JWT 形状、存疑项）见私有工作区 `docs/blessing-gap-analysis.md`。
