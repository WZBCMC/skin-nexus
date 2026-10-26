# SkinNexus / 肤枢

<p align="center"><img src="docs/assets/logo.svg" width="140" alt="SkinNexus 像素枢纽 N 标识"></p>

> 下一代 Minecraft 皮肤站：Yggdrasil 外置登录兼容、一键迁移 blessing-skin-server、自托管优先。

**状态**（每轮开发收尾同步 · 2026-10-26）：一期 M0–M5 ✅ · 二期免密登录（yggc/OIDC 内置）+ MUA Union 可开关模块 ✅ · 三期骨架（i18n / SEO / 主题 / Docker / 空库初始化）✅ · 管理面板对齐 blessing B1–B5 ✅ · 全量迁移闭环并已生产切流（67 用户 / 61 纹理零丢行、UUID 零漂移）✅ · 高清皮肤原生分辨率渲染 ✅ · 文档体系规整 + Docker 部署文档主线化 ✅ · 部署/迁移/文档三路静态审计修复 + 英文 README + **v0.1.0 首发** ✅ · UI 自定义体系审计修复（首页注入 + 级联顺序 + 主题校验）+ 贡献指南 ✅ · 访客首页（服务端渲染 landing）+ 逐页 SEO meta ✅ · 站点 Logo（SVG 矢量标识 + favicon/PWA 图标 + 访客首页入场编舞）✅ · 皮肤详情页（服务端渲染 `/skinlib/{tid}` = SEO 内容页，sitemap 切换）+ 启动器深链 `/user/closet`·`/user/profile` 修复 ✅ · A1 找回密码（SMTP 邮件流 + 忘记/重置密码全链路，防枚举/单次令牌/面板热配置）✅ · A2 Mojang 官方兜底（profile/hasJoined 未命中→官方 sessionserver，仅公网拨号/不跟随重定向/热开关）✅ · 运维观测面（网页/启动器登录 + Yggdrasil 服务器/Union 出入站通信日志，面板「访问日志」筛选分页可查，90 天保留）✅ · 竞品对照矩阵（blessing / element-skin / vSkin 三家源码实证差异，双语宣传面）✅ · 注册/找回表单按站点配置收敛（邀请码必填或隐藏、邮件未配置时邮箱与找回不可用）✅ · 管理面板站点设置分组化（七节分组 + 兜底开关复选框 + SMTP 掩码）✅ · 逐轮变更见 [`docs/CHANGELOG.md`](docs/CHANGELOG.md)，路线图见下。

**许可**：AGPL-3.0 · **发起人/著作权人**：YMZS

## 这是什么

SkinNexus（肤枢）是对年久失修的 [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) 的现代化重写：皮肤、身份、联合认证三线汇聚的中枢。

- **Yggdrasil API 完整兼容**：authlib-injector 直连，既有 MC 客户端与服务器**零改配置**，玩家 UUID 零漂移
- **启动器免密登录（yggc/OIDC 内置）**：设备码流 + refresh 轮换，OAuth token 直通进服（SJMCL 等社区启动器兼容），取代旧架构 Janus 外挂
- **MUA Union 成员站模块**：**可开关（默认关）**，未接入联盟的部署零负担；三头验签、绑定/同步、OAuth2 grant、共享纹理签名密钥
- **一键数据迁移**：用户/皮肤/披风/角色绑定平滑迁移（多算法密码兼容 + 惰性重哈希），手册见 [`docs/migration.md`](docs/migration.md)
- **现代技术栈**：Go + Vue 3，单二进制或 Docker 部署
- **多语言（i18n）**：内置 zh-CN/en，界面全量键值化，语言切换即时生效
- **SEO 一键配置**：meta/OG/canonical 注入、sitemap.xml、robots.txt、可整站 noindex（预发）
- **自定义 CSS/JS 与主题包**：admin 贴样式（blessing 同款）+ `data/themes/` 主题包（社区分发、AI 可生成，规范见 [`docs/theme-spec.md`](docs/theme-spec.md)）
- **Docker Compose 部署**：一键编排（可选内置 MySQL 或外部数据库），数据/主题/密钥卷持久化，升级不丢数据；空库自动建表 + 首访安装向导 `/setup`
- **对外数据接口**：CustomSkinAPI / USM / legacy 裸 PNG / 公开只读 API（社团官网读皮肤数据用）
- **头像与预览服务端渲染**：`/avatar/{player|user|hash|tid}`、`/preview/{tid|hash}` 输出 PNG，高清皮保留原生分辨率
- **自托管与数据主权**：零账号依赖、本地直连、单一控制点
- **插件扩展**：hook 合约已冻结（goja 运行时已裁决），宿主运行时后置

## 与主流开源皮肤站的对照

> 对照对象：[blessing-skin-server](https://github.com/bs-community/blessing-skin-server)（Laravel/PHP，上游仅剩维护性提交）、[element-skin](https://github.com/water2004/element-skin)（Go + PostgreSQL + Vue 3）、[vSkin](https://github.com/iJunecn/vSkin)（FastAPI + Vue 3）。
> 下表每格都经对方仓库**源码复核**（路由表 / grep，2026-10-25–26），方法与逐条证据见 [`docs/competitor-analysis.md`](docs/competitor-analysis.md)——✅ 不吝于人，❌ 不凭印象。

| 能力 | SkinNexus | blessing | element-skin | vSkin |
|---|---|---|---|---|
| 搜索引擎可见性（sitemap / robots / 逐页 meta·OG·JSON-LD） | ✅ 全量 | ❌ | ❌ | ❌ |
| 服务端渲染内容页（免 JS 即出正文、可收录） | ✅ 首页 + 皮肤详情页 | ❌ SPA 壳 | ❌ SPA 壳 | ❌ SPA 壳 |
| 皮肤库单品页（`/skinlib/{tid}` 可分享、可收录） | ✅ | ❌ 无皮肤库 | ❌ 仅列表+弹窗 | ❌ 仅列表 |
| 启动器深链（PCL 系「换皮肤/改密码」直达） | ✅ `/user/closet`·`/user/profile` | ✅ 约定发源地 | ❌ | ❌ |
| Yggdrasil 官方兜底（本地未命中 → Mojang） | ✅ 出站防护 + 面板热开关 | ❌ | ✅ | ✅ |
| 找回密码 + SMTP 邮件流 | ✅ 防枚举 / 单次令牌 | ✅ | ✅ | ✅ |
| OIDC 免密登录（免 Janus 外挂） | ✅ 内置，device_code 等四种 grant 全公告 | ⚠️ 需外挂 Janus + yggdrasil-connect | ⚠️ 实装但 discovery 公告不全 | ⚠️ 设备流完整，OAuth 细节弱 |
| MUA Union 联盟接入 | ✅ 内置可开关模块，出站同步实装 | ✅ 经 yggdrasil-connect 插件 | ❌ 源码无 | ⚠️ 仅配置壳 |
| blessing 一键迁移（ID 保全 / UUID 零漂移） | ✅ preflight → dry-run → import | —（迁移源头） | ❌ | ❌ |
| 封禁执行面 | ✅ 登录 / 认证 / hasJoined 三层 | ✅ 权限位封禁 | ✅ | ❌ 仅挡 hasJoined |
| 运维观测面（登录 + 服务器/联盟通信日志 90 天 + 审计） | ✅ 全链路面板可查 | ❌ | ⚠️ 仅权限审计 | ❌ |
| 部署形态 | Go 单二进制 / Docker，CI 直出生产镜像 | PHP 8.1 + Laravel 10（PHP 8.1 已 EOL） | Go + PostgreSQL + Redis | FastAPI + SQLite |

**独有领先区**（上表 ✅ 且三家对照均为 ❌/⚠️）：服务端渲染 + 逐页 SEO 体系、皮肤库单品页、blessing 一键迁移链、全链路运维观测面——四项目前在整个赛道无同类实现可对标。外加一张实测性能账：**2 核 / 3.5GB 生产 VPS 实测**，落地页 ~600 RPS（CPU 封顶）、纹理分发 5182 RPS、Yggdrasil 元数据 2365 RPS、全程零错误（压测记录见 [`docs/CHANGELOG.md`](docs/CHANGELOG.md)）。

## 快速开始（站长视角）

三条路径任选，详见 [`docs/build.md`](docs/build.md)（构建）与 [`docs/deploy.md`](docs/deploy.md)（部署）：

```bash
# 路径 A：Docker Compose（推荐）。CI 每轮 push 自动构建生产镜像推 GHCR，
# 部署机只拉不编——复制仓库根 docker-compose.yml、改好环境变量后：
git clone https://github.com/YimingZhanshen/skin-nexus.git && cd skin-nexus
docker compose up -d                  # 首装；升级 pull && up -d，数据卷零丢失
# → 打开站点自动进 /setup 安装向导（建管理员/站点名/皮肤域）
# 详细说明（数据库双模式/环境变量表/备份恢复/排错）见 docs/deploy.md §二

# 路径 B：源码构建 + systemd（无 Docker 的内网/面板机）
# 见 docs/build.md §二、docs/deploy.md §五

# 路径 C：从老皮肤站（blessing-skin-server）迁移数据
# 见 docs/migration.md——preflight → dry-run → import 三步，全程可回滚
```

**上线后必做**：反代必须透传 `X-Forwarded-Proto` 与 `X-Forwarded-For`（缺失会导致 ALI 协议头错误、全站共享限流桶）；备份 `data/sign-key.pem`（丢失=全网皮肤失效）。

## 路线图（实时）

| 阶段 | 内容 | 状态 |
|---|---|---|
| 一期 M0–M5 | 技术验证 / 骨架 / Yggdrasil 全量 / 迁移工具 / 前端管理 / 部署切流 | ✅ 开发完成（2026-10-22）；**生产切流完成（2026-10-25）** |
| 二期 H1/H2/L1–L4 | OIDC 免密核心 / Union 可开关模块 / 前端+管理+集成 | ✅ 完成（2026-10-23） |
| 兼容矩阵 | blessing/yggdrasil-connect 1:1 复刻缺口（实测基线） | ✅ 36 端点全量实测全对齐，P1 主干全清；缺口明细见 [`docs/compat-matrix.md`](docs/compat-matrix.md) |
| 三期骨架 | i18n / SEO / 主题与自定义 CSS-JS / Docker Compose / 空库初始化 | ✅ 骨架完成（2026-10-23） |
| 部署管线 | CI 构建生产镜像→GHCR，部署机只拉不编 | ✅ 完成（2026-10-23） |
| 管理面板对齐 blessing | B1–B5 全批（侧边栏/统计/状态/用户表/角色表/白名单/ygg 令牌/Union 运维/黑名单） | ✅ 完成（2026-10-24） |
| 迁移与切流 | 全量迁移闭环 + 正式域名切换 + 签名钥连续性验证 | ✅ 完成（2026-10-24/25） |
| 真机验收（A2） | SJMCL + 社区启动器 OAuth 免密登录→选角→进服 | 🔶 预检已修 4 处契约断点，待按清单执行 |
| 正版绑定 / 插件宿主 | Auth Source 第二实现 / goja 宿主运行时 | ⬜ 后置规划 |
| 邮箱验证流 | 验证邮件发送 + `SKIN_REQUIRE_VERIFICATION` 开门 | ⬜ 后置规划 |

## 文档

面向**部署/迁移**：

- [`docs/build.md`](docs/build.md) —— **构建指南**：环境要求、源码构建（后端/前端）、Docker 镜像、CI 产物、本地开发调试
- [`docs/deploy.md`](docs/deploy.md) —— 部署、环境变量全表、Nginx 反代、运维要点
- [`docs/migration.md`](docs/migration.md) —— **迁移指南**：从 blessing-skin-server 全量迁移（migrate 子命令手册 + 切流断言 + 签名钥对齐）

面向**了解项目/AI agent**：

- [`AGENTS.md`](AGENTS.md) —— 仓库地图、七条铁律、常用命令（AI agent 首读）
- [`docs/standards.md`](docs/standards.md) —— 协作开发规范与 Agent 开发指南
- [`docs/blueprint.md`](docs/blueprint.md) —— 设计蓝图：目标、架构、技术路线对比、协议分析（设计决策记录，实时状态以本文为准）
- [`docs/compat-matrix.md`](docs/compat-matrix.md) —— blessing / yggdrasil-connect 兼容矩阵与缺口优先级（实测基线）
- [`docs/theme-spec.md`](docs/theme-spec.md) —— 主题包规范（含 AI 生成主题的提示词模板）
- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) —— 逐轮开发变更日志
- [`CONTRIBUTING.md`](CONTRIBUTING.md) —— **贡献指南**：语言翻译 / 主题包 / 代码与文档三类贡献
- [`GOVERNANCE.md`](GOVERNANCE.md) / [`CITATION.cff`](CITATION.cff) —— 治理结构（BDFL）与引用格式

## 关键兼容契约（不可破坏）

改动以下任意项前先读 [`AGENTS.md`](AGENTS.md) 铁律并跑对应契约测试：

1. `/api/yggdrasil/**` 全路径原语义（204 清单、错误体 `{error,errorMessage}`、毫秒时间戳、PEM 严格格式）
2. 玩家 UUID 零漂移（`OfflinePlayer:` md5 变体，改名不重算）
3. 持久化数据与 blessing 存储约定同构可直迁（数据兼容铁律）
4. `internal/union` 与 `internal/plugin` 的 wire/hook 契约冻结

## 作者与署名

本项目由 **YMZS（[GitHub @YimingZhanshen](https://github.com/YimingZhanshen)）** 发起并享有原始著作权。治理结构见 [`GOVERNANCE.md`](GOVERNANCE.md)，引用格式见 [`CITATION.cff`](CITATION.cff)。

## 许可

AGPL-3.0（见 [`LICENSE`](LICENSE)）。版权行为：`Copyright (c) 2026 YMZS (YimingZhanshen) and SkinNexus contributors`。
