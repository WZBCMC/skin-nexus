# SkinNexus / 肤枢 — 项目立项蓝图（v0.1 草稿）

> 状态：**立项准备阶段**。本文为蓝图初稿，可行性分析结论、Yggdrasil 兼容清单、数据迁移映射表待 `analysis/` 三份报告回填。
> 生成日期：2026-10-22。

---

## 0. 项目定位与命名

- **英文名**：SkinNexus
- **中文名**：肤枢
- **命名释义**：Nexus（中枢/连接点）——皮肤、身份、联合认证三线汇聚的中枢；皮肤站不再只是贴图托管，而是皮肤 + 外置登录 + 联合认证 + 社区扩展的控制点。
- **仓库名建议**：`skin-nexus`（组织下），核心服务 `skin-nexus-server`、迁移工具 `skin-nexus-migrate`、插件 SDK `skin-nexus-plugin-sdk`。
- **替代使命**：替换年久失修的 blessing-skin-server（PHP 8.1 已出官方维护期），保留其数据资产与 Yggdrasil 生态兼容性。

## 1. 目标与需求基线（已拍板 2026-10-22）

| # | 需求 | 决议 |
|---|------|------|
| 1 | 一键迁移 blessing-skin-server 用户与皮肤数据 | 迁移范围：**用户 + 皮肤/披风 + 角色绑定**；商城/论坛等旧插件数据不迁 |
| 2 | Yggdrasil 外置登录（authlib-injector 配合）+ MUA Union 联合认证 | **内置为主 + 兼容 connect**：皮肤站内置 Yggdrasil API，同时可对接已有 yggdrasil-connect 节点；MUA Union 一期**先按 connect 源码推断协议**，接口预留 |
| 3 | 对外数据接口（社团/服务器官网读取皮肤站数据） | REST API + 只读公开 token；按最小核心实现 |
| 4 | 插件式社区扩展 | 形态待评析（见 §4 多路线对比，交多 AI 评审） |
| 5 | 正版账号绑定，正版/皮肤站双登录开关切换 | **暂不做，仅留开关**：架构上预留认证源（auth source）抽象与 feature flag，二期实现 |

一期功能范围：**最小核心**——账号/皮肤/披风/角色 + Yggdrasil 外置登录 + 迁移工具 + 插件框架 + 对外数据接口。数据库已定：**MySQL 8**。开源协议已定：**AGPL-3.0**。

---

## 2. 可行性分析（三份分析报告已全部回填，2026-10-22）

> 依据 `analysis/blessing-skin-server.md`、`analysis/yggdrasil-connect.md`、`analysis/authlib-injector.md`（全部结论带 文件:行号 证据，推断项已标注）。回填位置：

- 2.1 blessing-skin-server 技术栈与数据模型摘要 → 迁移可行性结论 ✅ 已回填（详见 analysis/blessing-skin-server.md）
- 2.2 Yggdrasil API 兼容面 ✅ 已回填（详见 analysis/authlib-injector.md，P0 15 条 / P1 11 条）
- 2.3 yggdrasil-connect 协议推断与 MUA Union 对接风险 ✅ 已回填（详见 analysis/yggdrasil-connect.md）
- 2.4 许可兼容性结论（AGPL-3.0 与三个上游项目的许可关系）✅ 已回填
- 2.5 Janus 外挂 OIDC 服务端分析 → 免密登录内置化结论 ✅ 已回填（详见 analysis/janus.md）

### 2.5 Janus 与 Yggdrasil Connect 免密登录（分析结论，2026-10-22）

**结论：无需部署 Janus 或任何额外项目，OAuth2/OIDC 免密登录内置进 SkinNexus 即可**。Janus（NestJS + panva/oidc-provider）只是 blessing 生态因「Laravel 缺 OIDC 服务端库」而生的外挂补丁（`refs/janus/README.md:5`），与皮肤站共享 MySQL 以授权码交接认证结果。新栈四条候选路线均有成熟 OIDC Provider 库，单体内置后：会话 → 直接签发 token，胶水层消失。SkinNexus = **blessing + yggdrasil-connect + janus 三合一**。

1. 免密登录链路（内置后）：启动器 → SkinNexus `/authorize`（OIDC）→ 站内登录（免密码外传）→ 选角（`Yggdrasil.PlayerProfiles.Select` scope）→ 签发 ID/Access Token → `yggc/userinfo`。
2. 须保持的 Janus 行为兼容：OIDC discovery 元数据齐全；authorization_code + implicit + refresh_token + **device_code**（公共客户端）；`client_secret_post`；scope 语义与 yggdrasil-connect 一致。
3. 部署面收益：少一个 Node 22 进程、少一份共享 DB 耦合、少一把 oauth-private.key 分发轮换。
4. 各路线 OIDC Provider 库成熟度（进技术路线评析）：TS=panva/oidc-provider（与 Janus 同款，零成本复刻）> .NET=openiddict ≈ Go=ory/fosite / zitadel/oidc > Rust=oxide-auth（短板）。
5. 待验证：既有 MUA Union 启动器对 Janus 特定字段的依赖程度；若要求对既有 Janus 部署「issuer 级无缝替换」需补实测清单。

### 2.1 blessing-skin-server 与数据迁移可行性（分析结论，2026-10-22）

**结论：迁移可行性高**。旧站业务表仅 5 张核心（users / players / textures / options / user_closet）+ 辅助表（reports/notifications/jobs/language_lines/scopes + Passport oauth_*），全部结构可从 22 个迁移文件完整还原（逐表字段清单见 `analysis/blessing-skin-server.md` §2）。

1. **旧栈**：PHP ^8.1 + Laravel 10.48 + Passport 11.8 + Twig + Intervention/Imagick；前端 Webpack5 + React17 + skinview3d。纹理 = 文件系统存 PNG（`TEXTURES_DIR`，默认 `storage/textures`，文件名 = hash、无扩展名），DB 只存元数据；**hash = sha256(PNG 字节) 64hex**，URL `/textures/{hash}`；角色档案为 CustomSkinAPI R1（`/{player}.json`）。type 枚举：`steve`/`alex`/`cape`，`alex` = slim。
2. **密码哈希（迁移第 1 关）**：`PWD_METHOD` + 全局 `SALT` 一套算法走全站（可选 BCRYPT 默认 / ARGON2I / MD5 / SALTED2MD5 / SHA256 / 512 / SALTED2SHA*），users.password **无 per-user 算法标记** → 迁移必须从旧 `.env` 读配置、实现同族 verify、登录成功后惰性重哈希到新算法；上线前抽样旧库 hash 前缀分布（`$2y$` / `$argon2` / hex）验证假设。
3. **email（迁移第 2 关）**：VARCHAR(100)、**无 DB 唯一索引**（仅应用层校验），存在空邮箱用户（OAuth/插件建号）→ 迁移前去重、定大小写折叠规则、允许 NULL。
4. **纹理（迁移第 3 关）**：多条 textures 记录可共享同一 hash 文件（删记录须按引用计数留文件）；`tid_skin` 默认 -1 但代码用 0 表示未穿戴（迁移时统一归一为 null）；时间字段为裸 DATETIME（upload_at / last_modified / register_at / last_sign_at），无 created_at/updated_at 惯例。
5. **其它坑点**（报告 §8 共 14 条）：users.avatar 是 textures.tid 引用；password_resets 表不存在（重置走签名 URL，无 token 可迁）；players.preference 遗留列；options 表 50+ 配置项需整体映射。
6. **插件系统**：package.json manifest + Hook/Filter/字符串事件共 60+ 扩展点（报告 §6 有完整清单）——**新站 hook 清单设计时以它为兼容参照**，降低旧插件移植成本。
7. **Yggdrasil 不自带**：旧站仅靠 yggdrasil-connect 插件（见 §2.3）。
8. **待验证 6 项**（报告 §9）：hash 是否严格 = sha256(磁盘字节)（Intervention `->encoded` 语义未实证）；preference 列语义；源库密码哈希混用分布；目标实例插件附加数据；jwt guard 可用性；email 脏数据/collation。迁移工具设计前需用真实旧库抽样闭合。

### 2.2 Yggdrasil API 兼容面（authlib-injector 分析结论，2026-10-22）

**结论：内置 Yggdrasil API 可行**。完整清单见 `analysis/authlib-injector.md` §7（P0 15 条 / P1 11 条），核心要点：

1. **P0 端点**：GET / 元数据（含 `meta.serverName`、`signaturePublickey`、`feature.*`）；POST `authserver/{authenticate,refresh,validate,invalidate,signout}`；POST `sessionserver/session/minecraft/join`；GET `sessionserver/session/minecraft/hasJoined`（须返回含 properties+signature 的完整 profile 或 204）；GET `sessionserver/session/minecraft/profile/{uuid}[?unsigned=false]`（空 body=查无此人）；POST `api/profiles/minecraft`；纹理静态文件。UUID 规则：32 位小写无符号。
2. **textures property**：value = Base64(JSON)，含 timestamp(ms)/profileId/ profileName/textures.{SKIN:{url,metadata:{model:default|slim}}, CAPE:{url}}。签名总体可选，仅 hasJoined 与 `unsigned=false` 时必须：SHA1withRSA 对 value 字符串签名 + Base64，密钥 = 元数据 `signaturePublickey`。
3. **纹理服务硬约束**：URL 域名须在皮肤域名白名单（`skinDomains`）；**URL 文件名 = 材质 hash = 客户端缓存键，换肤必须换文件名**；Content-Type 必须 `image/png`。
4. **严格度**：元数据 JSON / 公钥 PEM 格式错误直接游戏退出；查无此人必须 204（4xx/5xx 会被当网络错误）；PEM 仅允许换行符空白。
5. **发现机制**：ALI 响应头 `X-Authlib-Injector-API-Location`（相对/绝对均可）+ 启动器 prefetch（Base64 元数据 JSON，不发网络请求）——SkinNexus 对外数据接口和启动器集成两条路都要支持。
6. **许可**：authlib-injector 为 AGPLv3+ 附例外，**HTTP API 交互不受传染**，SkinNexus 内置实现无需开源（但我们本来就 AGPL-3.0）；wiki 文本 CC BY-SA 4.0，引用需署名。协议规范文档直接可用，**不要抄其 Java 代码**（一并 AGPL 无碍，但保持 clean-room 更稳）。
7. **待确认**：版本覆盖范围（推断 1.7.10～最新，待验证）；需求方所说「恒等名」在仓库无对应词条，推断指 `non_email_login` 或 `meta.serverName`，待 YMZS 确认语义。

### 2.3 yggdrasil-connect 与 MUA Union（分析结论，2026-10-22）

**结论：内置实现可行且必要**。yggdrasil-connect 不是独立服务，而是 blessing-skin-server ^6 的 PHP 插件（Laravel + Passport），直接读 Blessing 库——因此**不能作为新站的桥接层沿用**，SkinNexus 必须内置 Yggdrasil API（需求 2 的「内置为主」由此坐实）；「兼容 connect」理解为**兼容其 API 面与 Union 协议**，供 MUA Union 既有节点对接。

1. **API 根 `/api/yggdrasil`**：authserver 五端点、sessionserver join/hasJoined/profile、api/profiles/minecraft、api/users/profiles/minecraft/{name}、minecraftservices 别名、纹理管理（PUT/DELETE `api/user/profile/{uuid}/{type}`）、restore、yggc/*（OIDC/Janus）、api/union/member/*。错误体 `{error,errorMessage,cause?}`；未命中/invalidate/signout 返回 204。实现时以此清单为兼容基线。
2. **数据源映射**：SKIN/CAPE URL = `site_url/textures/{hash}`；旧库 textures.type `steve→default`、`alex→slim`（迁移映射直接复用）。
3. **MUA Union 协议**（hub 默认 `https://skin.mualliance.ltd/api/union`）：
   - 出站带 `X-Union-Member-Key`；入站验签 `X-Message-Signature/-Timestamp/-Nonce`（验签串 body+timestamp+nonce，RSA-SHA256，时间窗 ±10/30s + nonce 60s）；
   - `GET /privatekey` 统一下发**纹理签名私钥（全 Union 共享一把）**——架构含义：Union 模式下签名密钥不本地生成，由 hub 下发；
   - 成员操作：profile 同步/bind/bindto/remapuuid/sync/blacklist/queryemail/updateplugin、OAuth2 grant（base64 userInfo + HMAC-SHA256(member_key) + RSA-SHA256 签名 + hub 公钥分块 PKCS1 加密）；
   - 账号映射以邮箱为锚、绑定语义与安全等级多为**推断（待验证）**，一期按此实现、联调时校准。
4. **纹理签名细节**：textures JSON 含 `isPublic`/`signatureRequired`；properties = [textures, uploadableTextures:"skin,cape"]；openssl_sign 默认 RSA-SHA1 签 value；缺省不签、`unsigned=false` 才签、hasJoined 恒签名（与 §2.2 authlib-injector 侧观察一致，互为印证）。JSON 需 `JSON_FORCE_OBJECT` 语义（textures 空对象不塌成数组）。
5. **兼容基线的坑**（详细见报告 §7）：UUID = OfflinePlayer md5 变体、32hex 无连字符；改名 UUID 不变且旧 token 失效；validate 按 canJoinServer 判；join 会话 TTL 120s；`restore` 端点是签名 oracle、`queryemail` 泄露邮箱、`updateplugin`/`updatebackendkey` 是远程高危口——**新站必须修掉这三处安全问题**（作为「兼容但更安全」的差异化卖点）。

### 2.4 许可兼容性结论（三上游 vs AGPL-3.0，2026-10-22）

| 上游 | 许可 | 对 SkinNexus（AGPL-3.0）的影响 |
|---|---|---|
| blessing-skin-server | **MIT**（composer.json + LICENSE，Copyright The Blessing Skin Team） | ✅ 无碍：可自由重写、迁移数据、甚至参考/复用代码；MIT 与 AGPL-3.0 兼容（并入后整体按 AGPL-3.0 发布，须保留 MIT 版权声明） |
| yggdrasil-connect | **MIT**（Copyright LittleSkin） | ✅ 无碍：Union 协议实现可参考甚至移植代码，保留版权声明即可；一期按 clean-room 重写协议层更稳（代码质量原因，非许可原因） |
| authlib-injector | **AGPLv3+ 附 AUTHLIB-INJECTOR 例外**；wiki 文本 CC BY-SA 4.0 | ✅ 无碍：HTTP API 交互不受传染；SkinNexus 本就 AGPL-3.0，即便参考其代码也兼容，但**协议文档引用 wiki 需署名**（LICENSE-THIRD-PARTY 或文档脚注） |

**总判定**：AGPL-3.0 选型成立，无许可冲突。「仅迁数据不搬代码」的保守策略可以放宽为「可复用 MIT 代码但保留声明、authlib-injector 按协议文档实现不抄 Java」。最终以 `analysis/` 三份报告的许可章节为准。

---

## 3. 技术栈路线对比（交多 AI 评析）

### 3.1 候选路线总表

| 维度 | A. Go + Vue 3 | B. TS NestJS + Vue 3 | C. .NET 8 + Vue 3 | D. Rust + Vue 3 |
|---|---|---|---|---|
| 与现有技能栈（C++/Go/Vue）契合 | ★★★★★ | ★★★ | ★★ | ★★ |
| 单二进制/低依赖部署（N100/宝塔） | ★★★★★ | ★★（Node 运行时 + 依赖链） | ★★（.NET 运行时） | ★★★★★ |
| Yggdrasil/OAuth 生态成熟度 | ★★★★（goth 等） | ★★★★★ | ★★★★ | ★★★ |
| 插件系统可行性 | ★★★（Yaegi/插件进程） | ★★★★★（JS 沙箱天然契合） | ★★★（Roslyn 脚本） | ★★（dylib/WASM） |
| 长期内存/CPU 占用 | ★★★★★ | ★★★ | ★★★★ | ★★★★★ |
| 多 AI/社区协作文档丰富度 | ★★★★ | ★★★★★ | ★★★★ | ★★★ |
| 开发速度 | ★★★★ | ★★★★★ | ★★★★ | ★★ |
| 皮肤图处理（PNG 解码/尺寸校验） | ★★★★（image 库够用） | ★★★★（sharp） | ★★★★ | ★★★★★ |

### 3.2 各路线要点

- **A. Go + Vue 3**：后端 `chi`/`gin` + `sqlc`/`ent` + `goth`（Microsoft OAuth 预留）；纹理处理用 `golang.org/x/image`；Yggdrasil API 手写路由（协议面小，见 §2.2 清单）。部署一个二进制 + 一个前端静态目录，宝塔可托管。插件走「进程外 + 内置 Lua/JS 解释器」混合（见 §4）。
- **B. TypeScript NestJS + Vue 3**：全栈同语言；插件系统可用 `isolated-vm`/QuickJS 做 JS 沙箱，社区插件门槛最低；NestJS 模块化对「认证源抽象/插件宿主」友好；代价是 Node 依赖链与内存占用，N100 上多服务并存需压测。
- **C. .NET 8 + Vue 3**：ASP.NET Core Identity + EF Core，认证/授权最省心；NativeAOT 可减小部署体积；插件用 Roslyn 脚本或进程外；对 YMZS 技术栈是新增语言面。
- **D. Rust + Vue 3**：axum + sqlx；性能与内存安全最优，纹理处理与签名逻辑放心写；但开发速度与协作门槛最高，社区贡献者面窄。

### 3.3 评析建议的评判权重（供各 AI 统一打分口径）

1. 迁移工具与 MySQL 8 配合度（20%）
2. Yggdrasil API + 签名/纹理链路实现风险（20%）
3. 插件系统与所选插件形态的契合度（15%）
4. N100/宝塔部署与长期运维成本（15%）
5. 与 YMZS 现有技能栈（C++/Go/Vue）契合、可持续维护（15%）
6. 社区/多 AI 协作开发的文档与门槛（10%）
7. 二期正版绑定（Microsoft OAuth）实现难度（5%）

> 请各路 AI 按此权重给出评分、风险清单与一句话结论。

---

## 4. 插件形态路线对比（交多 AI 评析）

| 维度 | A. JS/TS 沙箱插件 | B. Lua 脚本插件 | C. 进程外 gRPC/Webhook | D. 混合（内置 Lua + 可选进程外） |
|---|---|---|---|---|
| 社区插件开发门槛 | ★★★★★ | ★★★ | ★★★★（语言无关） | ★★★★ |
| 沙箱安全性 | ★★★★（QuickJS/isolated-vm） | ★★★★★（天然小沙箱） | ★★★★★（进程隔离） | ★★★★☆ |
| 插件可做的扩展面（路由/皮肤钩子/认证源/UI） | ★★★★★ | ★★★★ | ★★★（受限于协议面） | ★★★★★ |
| 热加载/热更新 | ★★★★★ | ★★★★ | ★★★（进程管理） | ★★★★ |
| 对后端语言的耦合 | 低 | 中（需嵌解释器） | 极低 | 中 |
| 性能开销 | 中 | 低 | 低 | 低 |
| 与路线 A（Go）契合 | ★★★ | ★★★★★ | ★★★★★ | ★★★★★ |
| 与路线 B（NestJS）契合 | ★★★★★ | ★★★ | ★★★★ | ★★★★ |

统一要求（无论选哪种）：插件 manifest 声明权限（读皮肤、改皮肤、注册路由、挂认证钩子）；核心扩展点收敛为稳定 hook 清单（用户生命周期、纹理生命周期、认证前后、HTTP 路由注册、管理面板菜单）；插件 API 版本化（semver）。

---

## 5. 架构草案（跨技术栈通用）

```
                ┌────────────────────────────────────────────┐
                │                前端 Vue 3 SPA                │
                │   管理面板 / 皮肤编辑 / 插件市场 / 文档站      │
                └──────────────────┬─────────────────────────┘
                                   │ REST + 只读公开 token
┌──────────────────────────────────▼──────────────────────────────────┐
│                        SkinNexus Server（单体优先）                    │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────────────┐ │
│  │ Identity   │ │ Texture    │ │ Yggdrasil  │ │ Plugin Host      │ │
│  │ 用户/角色   │ │ 皮肤/披风   │ │ API 内置    │ │ 沙箱/进程外       │ │
│  └────────────┘ └────────────┘ └────────────┘ └──────────────────┘ │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │ Auth Source 抽象：站内密码 │ (预留)Microsoft OAuth │ (预留)其它    │ │
│  └────────────────────────────────────────────────────────────────┘ │
│  ┌────────────┐ ┌────────────┐ ┌────────────────────────────────┐ │
│  │ Public API │ │ Migration  │ │ Admin / Audit                  │ │
│  │ 官网/社团   │ │ 迁移工具    │ │ 审计日志                        │ │
│  └────────────┘ └────────────┘ └────────────────────────────────┘ │
└──────────────────┬───────────────────────┬───────────────────────────┘
                   │                       │
            MySQL 8（主存储）        文件存储（纹理原件，可 S3 兼容）
                   │
        ┌──────────▼───────────┐
        │ yggdrasil-connect    │ （可选桥接：既有 MUA Union 节点）
        └──────────────────────┘
```

关键设计决策：

1. **单体优先**，模块化分层（Identity / Texture / Yggdrasil / Plugin Host / Public API / Migration）；微服务化不在一期。
2. **Auth Source 抽象**：登录与 Yggdrasil 会话签发解耦于「身份从哪来」——站内密码为第一个实现，正版绑定（Microsoft OAuth）只是第二个 auth source，挂 feature flag 切换/并存，满足需求 5 的开关语义。
3. **纹理存储两级**：MySQL 存元数据 + 文件系统存原件（路径规则可配置，预留 S3 兼容后端），迁移时直接兼容 blessing 现有磁盘布局或做一次性搬迁。
4. **Yggdrasil API 内置**（需求 2）：authlib-injector 直连皮肤站；同时保留「外部 connect 桥接模式」，兼容 MUA Union 既有节点；两模式由配置切换。
5. **公开数据接口**（需求 3）：`/api/public/v1/*` 只读端点（用户资料、皮肤/披风下载 URL、UUID 解析），配只读 API key + 速率限制。
6. **审计与合规**：登录/迁移/插件启用等敏感操作全量审计日志；AGPL-3.0 下提供源码获取入口。

---

## 6. 数据迁移方案（骨架，映射表待 analysis 回填）

- 迁移工具独立仓库 `skin-nexus-migrate`，CLI 形态，支持 `--dry-run`。
- 步骤：① 只读扫描旧库（表结构以 `analysis/blessing-skin-server.md` 为准）→ ② 生成迁移映射表（users/players/textures/options）→ ③ 密码哈希策略决策（**待分析确认 blessing 哈希算法后定**：同算法直迁 vs 强制重置 vs 双哈希过渡）→ ④ 纹理文件迁移/校验（hash 校验）→ ⑤ UUID 与角色绑定迁移 → ⑥ 新旧并行双跑可选（增量同步，二期）。
- 验收：迁移后抽查用户可登录、皮肤/披风渲染一致、Yggdrasil hasJoined 返回与旧站一致。

## 7. 里程碑（草案）

| 阶段 | 内容 | 退出标准 |
|---|---|---|
| M0 立项（本阶段） | 蓝图 + 可行性分析 + 多 AI 技术栈评审 | 技术栈/插件形态拍板，蓝图评审通过 |
| M1 骨架 | 仓库/CI/核心域模型/登录/皮肤上传渲染 | 新用户可注册上传皮肤并展示 |
| M2 Yggdrasil | 内置 API + authlib-injector 端到端联调 | 皮肤站登录 → 客户端进服皮肤一致 |
| M3 迁移 | migrate 工具 + 一次性迁移演练 | 旧站数据全量校验通过 |
| M4 插件 | 插件宿主 + SDK + 示例插件 | 第三方示例插件跑通 3 个扩展点 |
| M5 发布 | 文档站 + 上线 + 旧站下线评估 | 生产切换，回滚预案在手 |

## 8. 协作开发规范

- **分支模型**：trunk-based，`main` 保护分支 + 短生命周期 feature 分支（`feat/<scope>-<desc>`），PR 合并前必须过 CI + 至少一人（或指定 agent）评审。
- **提交规范**：Conventional Commits（`feat|fix|docs|refactor|test|chore`），scope 用模块名（`auth`/`texture`/`ygg`/`plugin`/`migrate`）。
- **代码质量**：格式化与 lint 强制（Go: golangci-lint；TS: eslint+prettier；Rust: rustfmt+clippy——随技术栈定）；单测覆盖率门槛核心域 ≥70%；CI 矩阵 = lint + test + build。
- **PR 要求**：模板含「动机/改动/测试/迁移影响」；涉及 DB 的 PR 必须附迁移脚本；涉及 Yggdrasil 协议的 PR 必须附兼容性验证记录（对照 §2.2 清单）。
- **文档先行**：协议/接口变更先改 `docs/` 再改代码；ADR（架构决策记录）存 `docs/adr/`，编号递增。
- **安全**：依赖锁定 + 定期审计；密钥不进仓库；对外动作（发版、切库）先示内容再执行。

## 9. Agent 开发指南（多 AI/多 agent 协作）

- **仓库即上下文**：根目录 `AGENTS.md`（等价 `CLAUDE.md`）描述架构地图、常用命令、禁区（migration 不可逆操作需人工确认）；每个模块目录可有子 `AGENTS.md`。
- **任务粒度**：一个 agent 一个 PR 级任务；任务卡包含「动机、验收标准、涉及文件范围（write scope）、依赖任务」；写范围不重叠，重叠需先协商。
- **证据铁律**：结论标注出处（文件:行号或命令输出）；「不存在」类结论必须穷尽检索；不确定标「待验证」。
- **验证门**：每步贴真实命令输出，失败即停、最多重试 1 次；不编造输出。
- **分工边界**：重逆向/重分析（三仓库分析）走独立子代理并落盘 `analysis/`；实现 agent 不改他人 write scope 内文件。
- **AI 可读契约**：API 变更同步更新 OpenAPI schema（`docs/openapi.yaml`）——agent 以 schema 为接口事实源；插件 hook 清单同理版本化。
- **复盘沉淀**：每完成一个里程碑，把踩坑与有效流程沉淀为技能/ADR，不重复交学费。

## 10. 开源协议

- **选定：AGPL-3.0**（需求已拍板）。
- 理由：网络服务条款（§13）防止闭源云服务白嫖部署 SkinNexus 而不回馈，与数据主权/自托管信条一致；自用/社区自托管不受影响。
- 注意事项（待 §2.4 分析确认）：
  - 仅迁移 blessing-skin-server 的**数据**（不搬代码）→ 无 GPL 传染问题；
  - 若迁移工具或兼容层**参考/搬运**了 blessing-skin-server（GPL-3.0）代码，传染性与 AGPL-3.0 是否兼容需在 §2.4 给出结论；风险高则做 clean-room 重写迁移逻辑；
  - authlib-injector / yggdrasil-connect 的许可（待分析报告确认）决定「内置实现」能否直接复用其代码；不行则只实现协议（协议本身不受版权保护）不抄代码；
  - 第三方依赖的许可清单进 `LICENSE-THIRD-PARTY`。

## 11. 待补充与风险清单

| 项 | 说明 | 状态 |
|---|---|---|
| MUA Union 联合认证协议细节 | 二期已实现可开关成员站模块（`server/internal/union`，wire 契约冻结 + 偏差备案 6 项）；hub 侧五问（keyId 轮换等）联调校准 | ✅ 核心已实现，联调待做 |
| blessing 密码哈希算法 | PWD_METHOD+SALT 全站一套、无 per-user 标记；迁移读旧 .env + 同族 verify + 惰性重哈希（§2.1） | 策略已定，待旧库抽样验证分布 |
| 纹理签名链路 | 已明确：仅 hasJoined 与 unsigned=false 必须签，SHA1withRSA，密钥=元数据 signaturePublickey（§2.2/§2.3 互证） | ✅ 已闭合 |
| 技术栈终选 | 多 AI 评析后拍板 | ✅ 已冻结：Go + Vue 3 + goja（2026-10-22，M0 PoC 裁决） |
| 插件形态终选 | 多 AI 评析后拍板 | ✅ 已冻结：goja 内嵌 + hook 合约（P0 15–20 个），宿主运行时后置 |
| 域名/部署目标 | 腾讯云 2C/3.5G + 宝塔（150.158.12.29） | ✅ 已确认（2026-10-22，见 docs/deploy.md） |
| 旧站下线与回滚 | 双跑期时长与回滚触发条件 | ✅ 触发条件已定（docs/deploy.md §四），切流窗口待排期 |
| authlib-injector 版本覆盖 | 推断 1.7.10～最新，待验证；「恒等名」语义（non_email_login 或 meta.serverName）待确认 | 待 YMZS 确认 |
