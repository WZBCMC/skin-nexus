# SkinNexus / 肤枢

<p align="center"><img src="docs/assets/logo.svg" width="140" alt="SkinNexus 像素枢纽 N 标识"></p>

> 下一代 Minecraft 皮肤站：Yggdrasil 外置登录兼容、一键迁移 blessing-skin-server、自托管优先。

**状态**（每轮开发收尾同步 · 2026-10-25）：一期 M0–M5 ✅ · 二期免密登录（yggc/OIDC 内置）+ MUA Union 可开关模块 ✅ · 三期骨架（i18n / SEO / 主题 / Docker / 空库初始化）✅ · 管理面板对齐 blessing B1–B5 ✅ · 全量迁移闭环并已生产切流（67 用户 / 61 纹理零丢行、UUID 零漂移）✅ · 高清皮肤原生分辨率渲染 ✅ · 文档体系规整 + Docker 部署文档主线化 ✅ · 部署/迁移/文档三路静态审计修复 + 英文 README + **v0.1.0 首发** ✅ · UI 自定义体系审计修复（首页注入 + 级联顺序 + 主题校验）+ 贡献指南 ✅ · 访客首页（服务端渲染 landing）+ 逐页 SEO meta ✅ · 站点 Logo（SVG 矢量标识 + favicon/PWA 图标 + 访客首页入场编舞）✅ · 逐轮变更见 [`docs/CHANGELOG.md`](docs/CHANGELOG.md)，路线图见下。

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
