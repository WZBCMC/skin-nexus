# SkinNexus / 肤枢

> 下一代 Minecraft 皮肤站：Yggdrasil 外置登录兼容、一键迁移 blessing-skin-server、自托管优先。

**状态**（每轮开发收尾同步 · 2026-10-24）：一期开发完成（M0–M5）· 二期开发完成（免密登录 + Union 可开关模块 + 前端/管理/集成全量）· 三期骨架完成（i18n / SEO / 主题 / Docker / 空库初始化）· 管理面板账号会话化（2026-10-24：管理员账号登录即用管理面板、普通用户不见入口，X-Admin-Token 降为机器/API 回退；设置面板中文标签优先）· 管理面板对齐 blessing B1–B5 全批完成（2026-10-24：B1-B4 侧边栏/统计卡/状态页/用户表三护栏/角色表 UUID 零漂移/白名单 8 键+消费点；B5 ygg 双档令牌 TTL（3d 软窗 7d 硬窗+改名失效）+ ygg 三键接线 + Union 运维按钮 + 联合黑名单四动作（审计+双确认），CI 全绿）· **Union 配置 options 化 + 热切换完成（2026-10-24）：配置对齐 blessing options 存储面（UI 面板可配、迁移零搬运）、union_enabled 热开关（关=全路由 404+签名回本地）、hub 验签公钥现取带缓存、OAuth2 密钥对自动生成入库** · 管理面板用户/角色表 UI 打磨（2026-10-24：admin 容器 1280px、表头不换行、行内编辑透明化、验证态独立按钮、危险描边删除、窄屏横滚） · **Flash 段完成（2026-10-24）：公告 Markdown 双层消毒（服务端 goldmark+bluemonday、前端 marked+DOMPurify）、site_url 接线（option 优先 env 兜底、http(s) 白名单）、用户中心 A 级批次（侧边栏多页+仪表盘使用情况/启动器快速配置+衣柜 tab/即输即搜/设为头像+角色管理 PID/改名/删号+个人资料改昵称邮箱/头像/自助删号）** · **纹理展示与皮肤库批次完成（2026-10-24）：服务端头像/预览渲染（internal/avatargen Go 立方体光栅化器，/avatar/{player|user|hash|tid}+preview 对齐 TextureController 语义含 cape 422、size 钳位、PNG 输出）、皮肤库全栈（SkinlibController 对齐：可见性三档/筛选/搜索/排序/分页 + 点赞=入柜 + 上传者改名/可见性，点赞数 COUNT user_closet 零 schema 变更）、textarea 融入暗色主题；同批修复 PlayerByName COLLATE 列侧（5.7 prepared 协议 1267 全量 500 潜伏故障）、skinlib NULL description/model COALESCE、api.js skinlib 方法误挂 union 对象** · **全量迁移闭环（2026-10-24）：清测试库→migrate preflight/dry-run/import 实跑→对账 users.skipped==0、7 表 67/67/61/67/87/84/394 逐一对齐、纹理 61 文件字节级 MATCH、UUID 零漂移三处逐字节一致（老站 uuid 表=ygg 名查=profile）、惰性重哈希待登录实证；公开迁移手册 docs/migration.md** · 真机验收（A2）待执行 · 切流窗口待排期（域名切换步骤见 docs/migration.md §六）
**许可**：AGPL-3.0 · **发起人/著作权人**：YMZS

## 这是什么

SkinNexus（肤枢）是对年久失修的 [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) 的现代化重写：皮肤、身份、联合认证三线汇聚的中枢。

- **Yggdrasil API 完整兼容**：authlib-injector 直连，既有 MC 客户端与服务器**零改配置**，玩家 UUID 零漂移
- **启动器免密登录（yggc/OIDC 内置）**：设备码流 + refresh 轮换，OAuth token 直通进服（SJMCL / WZBC-Launcher 兼容），取代旧架构 Janus 外挂
- **MUA Union 成员站模块**：**可开关（默认关）**，未接入联盟的部署零负担；三头验签、绑定/同步、OAuth2 grant、共享纹理签名密钥
- **一键数据迁移**：用户/皮肤/披风/角色绑定平滑迁移（多算法密码兼容 + 惰性重哈希）
- **现代技术栈**：Go + Vue 3，单二进制部署
- **多语言（i18n）**：内置 zh-CN/en，界面全量键值化，语言切换即时生效
- **SEO 一键配置**：meta/OG/canonical 注入、sitemap.xml、robots.txt、可整站 noindex（预发）
- **自定义 CSS/JS 与主题包**：admin 贴样式（blessing 同款）+ `data/themes/` 主题包（社区分发、AI 可生成，规范见 [`docs/theme-spec.md`](docs/theme-spec.md)）
- **Docker Compose 部署**：一键编排（可选内置 MySQL 或外部数据库），数据/主题/密钥卷持久化，升级不丢数据；空库自动建表 + 首访安装向导 `/setup`
- **对外数据接口**：CustomSkinAPI / USM / legacy 裸 PNG / 公开只读 API
- **自托管与数据主权**：零账号依赖、本地直连、单一控制点
- **插件扩展**：hook 合约已冻结，宿主运行时后置

## 路线图（实时）

| 阶段 | 内容 | 状态 |
|---|---|---|
| 一期 M0–M5 | 技术验证 / 骨架 / Yggdrasil 全量 / 迁移工具 / 前端管理 / 部署切流 | ✅ 开发完成（2026-10-22）；迁移切流窗口待排期 |
| 二期 S0 | 契约侦察（Janus 快照、SJMCL/WZBC-Launcher 客户端分析）与选型 | ✅ 完成（选型 zitadel/oidc v3.51） |
| 二期 H1 | 免密登录核心：OIDC provider、token 双态、契约测试 | ✅ 完成 |
| 二期 H2 | Union 可开关模块：验签 / 入站出站 / OAuth2 grant / UnionSigner | ✅ 完成 |
| 二期 L1–L4 | 免密前端、Union 面板、管理集成、回归与验收辅助 | ✅ 完成（2026-10-23）：device 验证页/授权管理页、Union 绑定页+admin 面板、OIDC client CRUD、审计落库、H2 集成钩子、CI（含 MySQL 集成 job）、验收脚本 `scripts/acceptance-yggc.ps1` |
| 真机验收 | SJMCL + WZBC-Launcher OAuth 免密登录→选角→进服 | 🔶 测试实例运行中（`https://test-skin.wzbcmc.cn`）：A2 预检已修 4 处契约断点（见兼容矩阵），待 YMZS 按清单执行（开发过程文档 phase2 计划 §7.1） |
| 兼容矩阵 | blessing/yggdrasil-connect 1:1 复刻缺口（2026-10-23 实测基线） | ✅ 36 端点全量只读实测完成：Yggdrasil/yggc-OIDC/兼容面协议全对齐（join 双形态、refresh 选角、discovery 双挂载已修）；**P1 主干全清**——邀请码 + 改密 + 衣柜 + 管理面（设置面板 env 迁 UI、用户改密/删除）+ 认证限流（滑动窗口+失败锁定）均落地实测过闸；**迁移导入器落地**（`migrate import`，ID 1:1、加密兼容 `$2y$` 直迁直验、生产 dry-run 对账全一致）；追踪式迁移让既有库自动收新 DDL；邮箱验证流延后为独立线 |
| 三期骨架 | i18n / SEO 一键配置 / 主题与自定义 CSS-JS / Docker Compose / 空库初始化 | ✅ 骨架完成（2026-10-23）：vue-i18n 双语言包、SEO 注入+sitemap/robots、theme.json 主题包（docs/theme-spec.md 含 AI 提示词模板）、Dockerfile+compose 双 DB 模式、自动迁移+安装向导；功能填充见开发过程文档 phase3 计划 §6 |
| 部署管线 | CI 构建生产镜像→GHCR，部署机只拉不编 | ✅ 完成（2026-10-23）：CI docker job（多阶段 Dockerfile 每轮实证，`:latest`+`:sha-*`），升级=`compose pull && up -d`，数据卷持久化 |
| 管理面板对齐 B1+B2 | 侧边栏布局 / 仪表盘统计卡 / 运行状态页 / 邀请码布局 / 用户管理表 | ✅ 完成（2026-10-24）：侧边栏七视图（`components/admin/*`）、四统计卡（存储=纹理目录实测字节）、`GET /api/admin/v1/status`（版本/Go/MySQL/迁移水位/模块开关）、用户表搜索+分页+行内编辑（邮箱/昵称/验证/权限）+ blessing 权限三护栏（禁改自己/目标须更低/授 ADMIN 需 SUPER_ADMIN，Token 路径禁改权限） |
| 管理面板对齐 B3/B4 | 角色管理表 / 站点配置扩白名单 | ✅ 完成（2026-10-24）：B3 角色表搜索+改名（UUID 零漂移防护）+转手+换材质+删除（连带清理 uuid_map/selected_pid）；B4 白名单 +8 键（announcement / regs_per_ip / max_upload_file_size / player_name_rule 组 / texture_name_regexp，全 blessing 原名对齐迁移数据）+ 消费点接线（注册按 IP 计数+记录、玩家名规则组替换硬编码、上传大小钳制、公告卡纯文本渲染）+ 反代 XFF 实测核验与 clientIP 语义测试锁 |
| 管理面板对齐 B5 | ygg 双档令牌 + Union 运维 + 联合黑名单 | ✅ 完成（2026-10-24，Pro 段）：令牌双档 TTL（expire_1 软窗签发固化/expire_2 硬窗 refresh 门+改名软过期挤压，零 schema）；ygg_rate_limit 同名间隔闸 / ygg_tokens_limit LRU+审计 / ygg_search_profile_max 去重超限拒；Union 运维四按钮（服务器列表/密钥对/花名册同步/诊断）；黑名单四动作代理（查询/添加/释放/删除，响应原样透传+审计+双确认，本地进服拦截对齐上游不做） |
| 正版绑定 / 插件宿主 | Auth Source 第二实现 / goja 宿主运行时 | ⬜ 后置规划 |

## 文档

- [`docs/blueprint.md`](docs/blueprint.md) —— 设计蓝图：目标、架构、迁移方案、技术路线对比、开源协议（设计决策记录，实时状态以本文为准）
- [`docs/standards.md`](docs/standards.md) —— 协作开发规范与 Agent 开发指南
- [`docs/deploy.md`](docs/deploy.md) —— 部署、环境变量与切流预案
- [`docs/compat-matrix.md`](docs/compat-matrix.md) —— blessing / yggdrasil-connect 兼容矩阵与缺口优先级（实测基线）
- [`AGENTS.md`](AGENTS.md) —— 仓库协作说明、结构与铁律

## 作者与署名

本项目由 **YMZS（[GitHub @YimingZhanshen](https://github.com/YimingZhanshen)）** 发起并享有原始著作权。治理结构见 [`GOVERNANCE.md`](GOVERNANCE.md)，引用格式见 [`CITATION.cff`](CITATION.cff)。

## 许可

AGPL-3.0（见 [`LICENSE`](LICENSE)）。版权行为：`Copyright (c) 2026 YMZS (YimingZhanshen) and SkinNexus contributors`。
