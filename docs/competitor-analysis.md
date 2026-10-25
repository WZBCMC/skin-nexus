# 竞品分析：element-skin 与 vSkin（2026-10-25）

> 目标背景：SkinNexus 的目标是成为皮肤站 top1——在稳定性、安全性、易用性、功能多样性上全面超越竞品。本文是对两个主要开源竞品的源码级分析（浅克隆于 2026-10-25），产出可执行的差距清单与优先级。
>
> 分析对象：
> - [water2004/element-skin](https://github.com/water2004/element-skin)（v4.x，Go 1.26 + PostgreSQL 18 + Redis + Vue 3/Element Plus，MIT）
> - [LYOfficial/vSkin](https://github.com/iJunecn/vSkin)（FastAPI + Vue 3 + SQLite，GPL-3.0）
>
> 方法：两份子代理深读报告（结论附文件路径）+ Lead 对关键维度（SEO、路由、封禁、SMTP）的第一手 grep 复核。凡「README 宣称 ≠ 源码实证」处均已标注。

## 1. 总判断

**功能广度：element-skin > vSkin > SkinNexus（当前）。但在「搜索引擎可见性」与「启动器路由兼容」两个维度，两家是真空——SkinNexus 2026-10-25 建成的服务端渲染 SEO 体系是全赛道独有资产。** 领先策略：守住 SEO 独有赛道，补齐真实可用性短板（找回密码、Fallback、头像闭环），装饰性功能按性价比排队，安全性做到两家之上。

## 2. 实证领先区（SkinNexus 有、两家确认没有）

| 能力 | element-skin | vSkin | SkinNexus | 证据 |
|---|---|---|---|---|
| sitemap.xml / robots.txt | ❌ | ❌ | ✅ | Lead 直接 grep 两仓库 `sitemap\|robots` 零命中 |
| 逐页 title/meta/OG/JSON-LD | ❌（全站共用一个 `<title>皮肤站</title>`） | ❌ | ✅ 逐页 + JSON-LD | 两家 `index.html` 均为裸 SPA 壳；element-skin `useAppLayoutState.ts:196` 客户端改名 |
| 服务端渲染页面 | ❌（Nginx 纯静态托管前端） | ❌（仅 FastAPI SPA fallback） | ✅ landing + 皮肤详情页 | 两家均无 SSR/prerender（package.json/router 实证） |
| 皮肤详情页（单品 URL） | ❌ 只有 `/skin-library` 列表 + 页内弹窗 | ❌ 只有列表 | ✅ `/skinlib/{tid}` 服务端渲染 | 两家路由表均无 `:tid`/`:hash` 详情路由 |
| 启动器「换皮肤/改密码」深链兼容 | ❌ | ❌ | ✅ `/user/closet`、`/user/profile` | PCL 由 API 根替换 `api/yggdrasil/authserver` 派生 URL（`PageLoginProfileSkin.xaml.cs:151`、`ProfileUi.cs:291`）；两家 API 根与路由均不含该约定 |
| 登录/认证即查封禁 | 权限化登录 | ❌ 只挡 hasJoined | ✅ | SkinNexus `ygg/service.go:342-346,492`；vSkin `is_banned` 全项目仅 1 处调用 |
| blessing 迁移工具链 | ❌ | ❌ | ✅ 带 ID 保全的导入 CLI | 两家迁移为自建幂等 DDL / 硬编码 SQL 探测补列 |
| OIDC provider 完整度 | discovery 公告不全（device_code/client_credentials 未列入 `grant_types_supported`） | Janus 兼容层 + 设备流完整，但 union 联盟只是配置壳（无出站同步实现） | ✅ 完整 provider + 联盟实装 | element-skin `oidc_routes.go:24`；vSkin grep union 仅配置存取 |

## 3. 差距清单与决策（2026-10-25 拍板：目标 top1，按真实价值排序）

### 本轮要做（拍板通过）

| # | 项 | 来源启发 | 理由 | 量级 |
|---|---|---|---|---|
| A1 | **找回密码 + SMTP 邮件能力** | 两家都有（`/reset-password`）；我们 `verification_token` 列在（0008 迁移保留）但无 SMTP、无重置流程 | 当前最真实的可用性短板：用户忘密码只能找管理员 | 1-2 天 |
| A2 | **Fallback 最小版**（profile/hasJoined 未命中→Mojang 串行兜底） | 两家都有完整 Fallback 链（element-skin `service/fallback/`、`internal/service/probe/`；vSkin `backends/fallback_backend.py`） | 跨服可用性（「本站查不到也能进服」）；安全上吸取 element-skin 的出站防护设计（ValidateOutboundURL、禁重定向、响应上限） | ~1 天 |
| A3 | **方形头像一键截脸** | vSkin 闭环（`utils/image_utils.py:114-144`：基础脸 (8,8)-(16,15) + 帽子层 (40,8)-(48,15) alpha 合成，NEAREST 256px） | 体验闭环，成本极低 | ~半天 |
| A4 | **安全加固** | vSkin 弱点反向：内存限流、微软 OAuth state 进程内存、redirect_uri 单值、shared client 刷新免 secret；我们自审 | top1 的安全性底线：session cookie 属性、重置接口限流、令牌单次消费 | 随 A1 做 |

### 记档不抢工（后续批次）

| # | 项 | 来源 | 缓行理由 |
|---|---|---|---|
| B1 | 逐用户通知中心（已读回执/定向投递/OAuth 事件转通知） | element-skin `service/notice/`、`service/oauth/notifications.go` | 当前用户规模收益有限；2-3 天量级 |
| B2 | 设备授权流后台可视化配置（shared_client 多选/有效期/轮询间隔保存即生效） | vSkin `AdminOAuthApps.vue` + `/admin/oauth/device-settings` | 我们已有 device flow + /device 页；先核对差量再定范围 |
| B3 | 农历节日彩蛋框架 | element-skin `src/easter-eggs/index.ts`（`Intl.DateTimeFormat('zh-CN-u-ca-chinese')` 识别农历） | 装饰性，社区好感向 |
| B4 | 浏览器缓存：localStorage 键白名单清脏 + IDB LRU 渲染缓存（内存降级、objectURL 记忆） | element-skin `src/storage/storage.ts:52-65`、`indexedDbCache.ts`、`renderCache.ts` | 性能细节，顺手批次做 |
| B5 | `/v2/capabilities` 互操作发现 + 公开权限目录 | element-skin `httpapi/discovery.go` | 生态向，单站收益低 |
| B6 | webhook-worker 独立进程队列（lease claim/指数退避+抖动/死信/SSRF 防护/投递前权限复核，HMAC-SHA256 `v1=` 签名头） | element-skin `service/webhook/worker.go`、`cmd/webhook-worker/main.go` | 设计优秀但对当前规模过度工程；将来做开放平台时直接参考 |
| B7 | Python SDK | element-skin `python-sdk/` | 生态向 |

### 明确不做

| 项 | 理由 |
|---|---|
| three.js 全景首页/轮播背景 | JS 重、伤 LCP，与服务端渲染 landing 的 SEO 优势冲突 |
| 像素级材质 hash（Littleskin 系） | 与 blessing 兼容迁移冲突（迁移按文件字节 hash 对账）；不追 |
| PostgreSQL/Redis 换栈 | 单站规模 MySQL 足够；换栈无收益 |

## 4. 竞品弱点备忘（内部参考，不对外宣传）

- **vSkin**：封禁只挡 hasJoined（站点登录与 authenticate 均不查）；限流进程内存 dict（重启/多 worker 失效）；微软 OAuth state 存进程内存 dict（源码注释自认「生产应上 Redis」）；OAuth redirect_uri 单值、client_id 自增 int、shared client 刷新免 secret；无皮肤库详情页/搜索/收藏计数；无审计日志；死代码 `AuthCallback.vue` 未注册路由。
- **element-skin**：OIDC discovery `grant_types_supported` 只公告 authorization_code/refresh_token（device_code/client_credentials 实装但未公告）；「材质文件 LRU 缓存」README 宣称与源码不符（实际只有 avatar/viewer-snapshot 两桶）；无皮肤库单品页；前端无独立 CI job。
- **共同**：无皮肤库单品详情页、无 sitemap/robots/OG、无服务端渲染。

## 5. 抄作业重点索引（将来实现时按图索骥）

| 将来要做 | 参考实现 |
|---|---|
| Fallback 完整版（健康检查/公钥轮换/玩家白名单） | element-skin `internal/service/probe/probe.go`、`service/fallback/public_keys.go`（`LongestCommonYggdrasilRoot` 推导发现根、MergePublicKeys）；vSkin `fallback_backend.py`（serial/parallel + as_completed） |
| 通知中心 | element-skin `internal/database/schema.go:158-198`（notices + notice_receipts + notice_targets）、`service/notice/user.go`（详情读取即标已读） |
| Webhook 队列 | element-skin `service/webhook/worker.go`（两阶段 DB claim + 30s lease、30s→6h 退避带 SHA256 抖动、12 次/72h 死信、SSRF 防护 DialContext、投递前权限复核） |
| 权限模型升级（allow/deny 覆盖 + 会话×入口策略） | element-skin `internal/database/permission/effective_compute.go:28-83`（角色并集 ∪ allow − deny）、`session_permission_policies` |
| 浏览器渲染缓存 | element-skin `src/storage/indexedDbCache.ts`（LRU 逐出保护刚写入 key）、`renderCache.ts`（IDB→内存降级） |
| 方形头像 | vSkin `utils/image_utils.py:114-178`（含默认 Steve 头像硬编码头像生成） |

## 6. 会话归档注记

- 竞品浅克隆位置：`refs/element-skin`、`refs/vSkin`（随本仓库工作区，不入 git）。
- 详细事实清单（两份子代理报告，文件路径级证据）留存于当日会话记录；本文为其决策化摘要，实现时以克隆源码为准重新取证。
