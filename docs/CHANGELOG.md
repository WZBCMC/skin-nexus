# 变更日志

> 粒度 = 每轮开发收尾。README「状态」行只保留一句话摘要，逐轮细节沉淀在此文件（与 README 状态行每轮同步维护）。

## 2026-10-25（静态审计修复 + 英文 README + v0.1.0 首发）

- **三路静态审计**（部署构建 / 迁移 / 全文档噪音）对照源码逐条复核，修复文档硬伤：迁移手册签名钥导出 SQL 列名（老站 options 列为 `option_name/option_value`）、`--source-prefix` 必填语义、`skipped>0` 归因与处置（email/nickname CI 唯一键撞车、空邮箱→NULL）、纹理问题文件**不拷贝需人工补拷**、手动建表两处必炸（0009 重叠 ALTER 需 `--force`、import 强查 0008 记标）、`SKIN_LEGACY_ENV` 盐链挂载全流程补写（弱哈希用户登录的前置条件）、dry-run 通过标准改以 WARN 为信号、实跑后对账 SQL 与告警词典、`migrate uuid` 子命令入档、派生列/不迁列语义表。
- **部署文档**：SKIN_LISTEN 在 bridge 网络下的正确语义、模式一 4 处秘密同步、`SKIN_BASE_URL` 首启快照语义（纹理 URL/OIDC issuer 事后改不动）、env/option 分键优先级、GHCR 可见性与 `:sha-` 标签口径、回滚不回滚 schema 警示、两把密钥的生成时机条件、`/setup` 一次性门与迁移场景顺序、内置库 MySQL 8.0 标注、XFF 信任区与端口收敛、`X-Real-IP` 不被读取说明、systemd 完整单元。
- **CI**：PR 构建不再推 `:latest`（docker job push-only）；集成测试加 MySQL **5.7/8.0 双矩阵**（此前两起 5.7 专属 SQL 事故无门禁）；Go 版本钉 1.26 对齐 go.mod。
- **主题规范对齐真实 CSS 契约**（7 个设计令牌 + 稳定选择器，按模板生成主题真正生效）；blueprint / standards / GOVERNANCE 过时项清零（openapi.yaml 标注规划中、M4 里程碑口径注记、blessing 许可修正为 MIT、回滚出处改指 migration.md 等）。
- **隐私清尾**：文档与测试夹具中残留的院校缩写、主机面板名、硬件型号、校园语境注释全部泛化；面板 `site_url` 描述与代码行为对齐。
- **英文 README**（`README.en.md`）上线，与中文 README 互链。
- **v0.1.0 首个 GitHub Release**：`version` 定稿 0.1.0；产物口径沿用既有拍板——GHCR `:sha-<commit>` 镜像为唯一部署产物，Release 页附源码包与镜像指引。

## 2026-10-25（文档规整）

- **文档体系规整**：README 瘦身重构（一句话状态 + 站长快速开始三路径 + 兼容契约红线）、新增构建指南 `docs/build.md` 与逐轮变更 `docs/CHANGELOG.md`、迁移手册 `docs/migration.md` 扩充（checklist / 命令落地 / 迁移后运营检查）。
- **Docker 部署文档成为主线**：`docs/deploy.md` 重写为 Docker 优先手册（数据库双模式、环境变量表、启动验收、升级回滚、卷备份恢复、排错表）；`docker-compose.yml` 默认改拉 GHCR 现成镜像（`pull_policy: always`，源码构建降为离线备选）；systemd 降为 §五备选。
- **仓库清理**：移除手工打包的部署产物 `dist-deploy/`（12MB 二进制 + web-dist 快照）与 `dist-deploy.tar.gz`（5MB）——GHCR 镜像为唯一部署产物来源，避免产物与代码漂移；`.gitignore` 补 `dist-deploy*`。
- 公开文档去部署环境/站点身份信息：示例统一 `skin.example.com`，示例库名 `legacy`，清除测试夹具与注释中的站点痕迹（Go 测试全绿）。

## 2026-10-25

- **正式域名切流完成**：生产实例运行中；签名钥连续性实测——union 模式对外 `signaturePublickey` 与老站 `ygg_private_key` 同指纹，客户端零改配置、缓存公钥不失效。
- union 热切换「关→开」误报修复：路由注册期判 `Enabled()` 导致后开的开关拿不到路由 → 常驻注册 + 请求级门禁（关仍是 404 零暴露）。
- 皮肤库预览纯白修复：逐卡 skinview3d 爆浏览器 ~16 WebGL context 上限 → 卡片改服务端 `/preview/{tid}` PNG lazy 加载。
- 管理面板「自助诊断」1:1 对标 yggdrasil-connect：三态模态 + 9 行结果表 + 导出 HTML 报告（`{status,data}` 形态含非 2xx 的 status_code/headers/body 证据）。
- **高清皮肤渲染 v3**：保留原生分辨率，64 空间 UV 按倍数映射到高清网格并逐像素亚纹素采样（v2 降采样方案丢细节，撤销）；2:1 旧版高清皮无叠加层（`HasLayer2` 修正）；单测覆盖 128×128 / 128×64 / 细节保留。
- 预览 URL 版本化（`?v=RenderVersion`）：规避一年 immutable 缓存吃旧图。
- 皮肤库「只看我上传的」失效修复：`/api/v1/me` 返回 `id` 而前端取 `uid` → uploader 参数被跳过、canEdit 同步修正。
- `common.rename` i18n 补键（zh-CN / en）。

## 2026-10-24

- **全量迁移闭环**：清测试库 → `migrate preflight/dry-run/import` 实跑 → 对账 `users.skipped==0`、7 表 67/67/61/67/87/84/394 逐一对齐、纹理 61 文件字节级一致、UUID 零漂移三处逐字节一致；惰性重哈希待登录实证。
- 管理面板对齐 blessing **B1–B5 全批**：侧边栏七视图 / 四统计卡 / 运行状态页 / 用户表三护栏 / 角色表 UUID 零漂移防护 / 站点白名单 8 键 + 消费点 / ygg 双档令牌 TTL（3d 软窗 7d 硬窗 + 改名失效）/ Union 运维四按钮 / 联合黑名单四动作（审计 + 双确认）。
- **Union 配置 options 化 + 热切换**：配置对齐 blessing options 存储面（UI 可配、迁移零搬运）；`union_enabled` 热开关（关 = 全路由 404 + 签名回本地）；hub 验签公钥现取带缓存；OAuth2 密钥对自动生成入库。
- 管理面板账号会话化：管理员账号登录即用管理面板，普通用户不见入口，X-Admin-Token 降为机器/API 回退。
- 纹理展示与皮肤库批次：服务端头像/预览渲染（`internal/avatargen` Go 立方体光栅化器，`/avatar/{player|user|hash|tid}` + `/preview/...`）；皮肤库全栈（可见性三档 / 筛选 / 搜索 / 排序 / 分页 + 点赞=入柜）。
- 用户中心 A 级批次：侧边栏多页 + 仪表盘 + 衣柜 tab / 即输即搜 / 设为头像 + 角色管理 + 个人资料 + 自助删号。
- 公告 Markdown 双层消毒（服务端 goldmark+bluemonday、前端 marked+DOMPurify）；`site_url` 接线（option 优先 env 兜底、http(s) 白名单）。
- 修复批次：PlayerByName COLLATE 列侧（MySQL 5.7 prepared 协议 1267 潜伏故障）、skinlib NULL description/model COALESCE、api.js skinlib 方法误挂 union 对象。
- 公开迁移手册 [`docs/migration.md`](migration.md) 上线（2026-10-24）；切流签名钥 union 模式例外实测说明次日补充。

## 2026-10-23

- 二期 L1–L4 收官：device 验证页 / 授权管理页、Union 绑定页 + admin 面板、OIDC client CRUD、审计落库、CI（含 MySQL 集成 job）、验收脚本 `scripts/acceptance-yggc.ps1`。
- 兼容矩阵 36 端点全量只读实测完成，P1 主干全清（邀请码 + 改密 + 衣柜 + 管理面 + 认证限流）；`migrate import` 落地（ID 1:1、`$2y$` 直迁直验、生产 dry-run 对账全一致）。
- 三期骨架：i18n 双语言包、SEO 注入 + sitemap/robots、theme.json 主题包（`docs/theme-spec.md`）、Dockerfile + compose 双 DB 模式、追踪式自动迁移 + `/setup` 安装向导。
- 部署管线：CI docker job（多阶段 Dockerfile 每轮实证，推 GHCR `:latest` + `:sha-*`），部署机只拉不编。
- 邀请码模块（`invitation_codes` 实表 + `SKIN_INVITE_REQUIRED` 开关）。
- **数据兼容铁律（AGENTS.md 铁律 7）拍板**：持久化数据必须同构可直迁或有无损导入适配。

## 2026-10-22

- **一期 M0–M5 全部完成**：M0 插件运行时 PoC 裁决（goja 冻结为 v1 运行时）、S1+S2 骨架、S3 密码 12 算式移植、S5 Yggdrasil 全端点契约测试、S4 纹理管线、S7 migrate 引擎、M1 接线、S8 公开 API、S6 Vue 前端、管理面板、S9 + CI + 文档、migrate CLI（preflight/uuid）。
- 立项完成：三仓库（blessing-skin-server / yggdrasil-connect / authlib-injector）源码级分析、两轮多 AI 评审、技术栈冻结 **Go + Vue 3**、插件 v1 = goja。
- 建仓与开源边界拍板：主仓 `YimingZhanshen/skin-nexus`（BDFL）、AGPL-3.0、署名 `YMZS (YimingZhanshen)`、开发过程文档不入库。
- 线上侦察：部署环境实测确认（小规格云主机 + 面板机、MySQL 已有）、迁移规模实测（66 用户级小库）、老站基线 = yggdrasil-connect 6.1.0-0.3.2。
