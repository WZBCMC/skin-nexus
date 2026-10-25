# 变更日志

> 粒度 = 每轮开发收尾。README「状态」行只保留一句话摘要，逐轮细节沉淀在此文件（与 README 状态行每轮同步维护）。

## 2026-10-26（A1 找回密码：SMTP 邮件流 + 忘记/重置密码全链路）

- **背景**：竞品评估（docs/competitor-analysis.md）确认「无密码找回、全站无 SMTP」是两家竞品与本站在易用性上的共同硬缺口——`verification_token` 列早已有之但邮件流从未实现。本轮补齐端到端。
- **邮件传输 `internal/mailer`**（纯标准库 net/smtp，零第三方依赖——数据主权/最小依赖红线）：465 隐式 TLS 直连、其它端口自动 STARTTLS 升级；PLAIN 认证由 stdlib 强制「非 TLS 不发凭据」；发件人支持 `Name <a@b.c>` 形式；主题 RFC2047 编码、正文 base64（76 列折行）；整会话 30s 硬 deadline 防黑洞服务器吊死请求；地址 CR/LF 注入直接拒绝（信封与头双重校验）。
- **重置令牌语义**（`store/password_reset.go` + 迁移 `0010_password_reset.sql`）：32 字节 crypto/rand 原文**永不落盘**，只存 SHA-256 摘要（库泄露≠可用水滴链接）；30 分钟 TTL 写入与校验同用 DB 时钟（`DATE_ADD(NOW(3))`/`NOW(3)`，Go 与 MySQL 时钟漂移无法延寿）；**单次使用**靠原子 UPDATE（`used_at IS NULL`）——重放双击/泄露链接二次使用直接找不到；再次申请自动作废旧令牌（最新邮件生效）；过期/已用行顺手清理，表有界无需 cron。
- **防枚举与限流**：`POST /api/v1/password/forgot` 已知/未知邮箱同形恒 200（发信失败也保持同形，SMTP 故障不变成存在性预言机）；未配置 SMTP 返回 503 明示配置态（非用户数据）；共享 Limiter 新键空间 `pwf-ip:`/`pwf-mail:`/`pwr-ip:` 滑窗限流（IP+地址双维度）。`reset` 端点 unknown/expired/used 三种失败**坍缩为同一条 400 文案**，不泄露状态。
- **重置成功 = 镜像改密吊销链**：`SetPasswordHash` 无条件写（chain-trace P0 守卫沿用）→ `InvalidateUserTokens` 杀全部 Yggdrasil 令牌 → `OnPasswordChange` panic 屏障杀 OIDC/启动器授权 → `DeleteUserSessionsExcept` 清全部网页会话（带本会话 cookie 则保留，与改密同规则）→ 审计 `password_reset` 行。
- **设置面板 SMTP 热配置**：`smtp_host/port/user/pass/from` 五键入白名单（发信时逐键读取，option 覆盖 env 默认，**保存即生效无重启**）；`smtp_pass` 审计脱敏（与 union_member_key 同列）；env `SKIN_SMTP_*` 作面板占位默认。
- **前端**：登录卡「忘记密码？」→ 找回卡（输入邮箱→统一「已发送」文案）；`/reset-password?token=…` 真实 URL（邮件里回跳，boot 按路径识别，与 /device 同款探法）→ 重置卡（双输密码前端校验→成功态→去登录）；`/forgot-password` 可分享直链；中英 i18n 全键。
- **回归测试**：mailer 进程内假 SMTP 会话端到端（信封去显示名/主题 RFC2047/base64 正文回环/CRLF-only）、注入拒绝（收件人 CRLF 不拨号）、禁用态/nil 接收者、envelope 解析表、隐式 TLS 端口判定；store 集成（单次消费/重放拒绝/未知摘要/过期回拨/最新邮件胜出/UserByEmail 大小写与「昵称不是邮箱」）；前端 `vite build` 通过。

## 2026-10-25（站点 Logo：SVG 矢量标识接入 + 访客首页入场编舞）

- **官方标识定稿**：「像素枢纽 N」——圆角玻璃底板 + 蓝→绿渐变块面 N + 枢纽节点电路纹理（SVG 矢量为唯一事实源，AI 位图不入库）；16px 眯眼测试、单色可复现、白/深双底兼容、双色渐变均验证通过。
- **全链接入**：`web/public/favicon.svg`（现代浏览器主图标）+ `favicon.ico`（16/32/48 兼容回退）+ `apple-touch-icon.png` + PWA `manifest.webmanifest`（192/512 图标、theme_color）——栅格兼容件全部从 SVG 几何直接绘制派生（`logo/render_icon.py`，8× 超采样），非位图描摹。
- `web/index.html` 接入 icon/manifest/theme-color 链接；README（中英）首页横幅挂 `docs/assets/logo.svg`。
- 服务侧无需改动：spaHandler 按 `os.Stat` 对 dist 既有文件原样放行，favicon/manifest 均直达；landing head 同步补齐图标/manifest 链接（SPA 壳与访客首页双覆盖）。
- **访客首页 logo 入场编舞**（pixel2motion 工序产出移植）：hero 区内嵌动效标识——底板淡入→N 块面 mask 擦除→电路线 draw-on→枢纽节点 stagger pop→整体收束，1600ms 共此时钟（黄金比 20/50/30、字面缓动值防 Chromium `var()` 静默降级、draw-on 用 butt cap 防起点墨点）；`prefers-reduced-motion` 直接终态；动画 SVG/CSS 零依赖内联，主题仍可经自定义 CSS 覆盖。
- **landing footer 移出 `.wrap`**：footer 原窝在内容容器里，wrap 底距垫在全出血页脚带下方成死角留空——挪为 body 级元素（语义也更对），并加结构回归断言（footer 必须闭合于最后一个 `</div>` 之后）。
- **修复「拖到启动器」拖拽载荷**：原按钮是裸 `<a href>` 拖拽（text/uri-list 裸 URL），HMCL/PCL2 均不认——按 authlib-injector《启动器技术规范》「通过拖拽设置」改为 `dragstart` 写 `text/plain` = `authlib-injector:yggdrasil-server:{encodeURIComponent(API根)}`、`dropEffect=copy`（与老站 yggc `dnd.ts` 同款；PCL2 `FormMain_Drop` 取 DataFormats.Text 前缀匹配后 `WebUtility.UrlDecode`）。
- **超管显示与保护修复**：用户管理表 `permission=2`（SUPER_ADMIN）因下拉无 2 选项显示为空白——改为「超级管理员」徽标（中英 i18n 补 `perm2`）；同时补两个服务端守卫洞——① `DELETE /users/{id}` 原只护 uid 1（迁移库中 uid 1 是普通用户、真超管是别的 uid，等于超管可被任意管理员删除）→ 改为 permission≥2 一律不可删 + `inlineGuard`（禁自删/只能操作严格低于自己的）；② `PUT /users/{id}/password` 原无守卫（普通管理员可重置超管密码=接管）→ 同挂 `inlineGuard`。UI 删除按钮对 uid 1 / permission=2 隐藏。
- **修复启动器深链 `/user/closet`、`/user/profile`**：PCL 由 API 根替换 `api/yggdrasil/authserver` → `user/closet`/`user/profile` 构造跳转（`PageLoginProfileSkin.xaml.cs:151`、`ProfileUi.cs:291`，blessing 路由约定），而 SPA `boot()` 只认 `/device`/`/register`——启动器「换皮肤/修改密码」按钮落到仪表盘而非衣柜/资料页。现按路径直达对应视图；未登录深链先出登录卡、登录后直落目标页。`/user/players`、`/user/skinlib`、`/user/dashboard` 同映射。

## 2026-10-25（皮肤详情页 = SEO 内容页）

- **拍板**：以皮肤详情页取代玩家档案页作为搜索引擎的内容页（原 `/profile/{name}` 70 条 sitemap 条目是 SPA 空壳——匿名访问落到登录框、且不存在的角色名也返回 200，构成软 404 毒药；已从 sitemap 移除）。
- **服务端渲染详情页 `/skinlib/{tid}`**（`texturepage.go`，Go 模板直出正文，不依赖 JS 即完整可读）：名称/上传者/模型/点赞数/上传时间/可见性 + 服务端渲染预览图（skin → `/preview/{tid}`，cape → `/textures/{hash}` 平铺，与皮肤库 UI 同款分流）+ 下载纹理 PNG。经同一套 SEO/主题/自定义注入层。
- **SEO 重点**：逐页 title（`{名称} 的 Minecraft 皮肤 - {站名}`）/description（含名称、类型、模型、上传者、点赞数）/canonical/逐页 og:image（服务端渲染预览图）/JSON-LD（schema.org `CreativeWork`：name/image/author/datePublished/interactionStatistic 点赞数）。
- **隐私与安全**：私密纹理仅上传者/管理员可见且带 `noindex,nofollow`；不可见即 404（品牌化 404 页，非裸 `http.NotFound`）；JSON-LD 注入层对 `</` 做 `<\/` 中和（材质名含 `</script>` 无法提前闭合 script 元素，json.Marshal 双保险）。
- **渐进增强**：无 JS 时页面是完整的静态预览图+元数据；有 WebGL 时 `assets/viewer.js`（固定名，非哈希——服务端模板引用需稳定契约）动态 import skinview3d（~500KB chunk 仅皮肤页按需加载，披风页/老浏览器不载）把静态图升级为自动旋转 3D 预览，并接好喜欢/入衣柜按钮（401 → 跳登录）。
- **sitemap 切换**：`/skinlib/{tid}` 公开材质条目（≤5000，升序）替代原 profile 条目，带 `<lastmod>`（上传时间，`SitemapXML` 支持 `SitemapEntry{Path,LastMod}`）；私密材质永不枚举（`PublicTextureIDs` 强制 `public=1`）。新增 `store.SkinlibOne`（可见性 gate：公开→所有人，私密→仅本人/管理员）。
- **皮肤库卡片接通详情页**：预览图与名称均为链接指向 `/skinlib/{tid}`。
- 回归测试：模板渲染（含披风页无 3D canvas、私密标签）、JSON-LD 注入安全、absURL 拼接、逐页 NoIndex/OG image、sitemap 无 profile 条目 + lastmod 只出现一次、store 可见性 gate 集成测试（匿名/他人/本人/管理员/缺失 × 公开/私密矩阵）。

## 2026-10-25（访客首页 + 逐页 SEO meta）

- **服务端渲染公开首页上线**（拍板：方案 B + 基础三件套 + 皮肤库精选）：根路径 `/` 由 Go 模板直出正文（不依赖 JS，爬虫/AI 抓包即完整内容）——hero + 注册/登录 CTA、实时统计（注册用户/游戏角色/皮肤材质）、站点简介、皮肤库精选（`/preview/{tid}` 服务端渲染图，按点赞取前 8 公开材质）、页脚署名。经同一套 SEO/主题/自定义 CSS-JS 注入层（设计令牌带 fallback，主题切换即整站换肤）。
- **逐页 SEO meta**：`seo.RenderHeadWith(PageMeta)` 落地，`/profile/{name}` 逐页标题（`<角色名> - <站名>`）与逐页 canonical——sitemap 条目不再千篇一律；首页 description 用站点简介兜底。
- **`/register` 深链识别**：SPA 启动按路径直达注册卡（sitemap 早已收录 `/register`，此前落到登录卡）。
- 回归测试：landing 渲染（CTA/统计/精选/注入/转义）、profile 逐页 meta、注入覆盖与级联顺序断言保持全绿。
- **上线即抓即修**：html/template 会剥离模板源里的 HTML 注释，`<!--skin-*-->` 锚点在 landing 渲染时被整体吃掉（SEO/主题/自定义三块全部失去锚点、首页无 title）——锚点改为 `template.HTML` 受信值从数据侧注入，并新增「head 必须含 title/canonical」回归断言。
- **精选卡片披风破图修复**（线上截图取证：`/preview/{id}` 对 cape 一律 404——该端点故意拒披风过全身渲染器）：landing 按 skinlib UI 同款分流——skin 走 `/preview/{tid}` 渲染图、cape 走 `/textures/{hash}` 平铺纹理，卡片图改等比缩放。

## 2026-10-25（UI 自定义体系修复 + 贡献指南）

- **首页注入断裂修复**：`spaHandler` 把根路径 `/` 当静态文件裸发（`os.Stat` 空路径命中 dist 目录），SEO/主题/自定义 CSS-JS 在首页全程未注入——改为「仅真实资产文件裸发，一切 HTML（`/`、`/index.html`、SPA 深链）走注入」，并补全路径回归测试。
- **注入顺序断裂修复（主题契约复活）**：主题/自定义 CSS 原先落在 Vite 编译产物 CSS **之前**，`:root` 变量覆盖被应用样式反压、照规范生成的主题不生效——改为统一注入 `</head>` 前（产物 CSS 之后，主题先、自定义后），补级联顺序断言测试。
- **主题加载器强化**：`name/version/inject` 必填强制（缺失跳过）、`requires.skin_nexus`（`>=x.y`）真校验、所有跳过原因落日志、资产链接带 `?v=<主题版本>` 破缓存。
- **theme-spec.md 整篇重写对齐实现**：注入顺序保证、资源平铺包根/全小写、重启生效与切换/卸载/调试排查清单、JS 时机（DOMContentLoaded 模板）、scoped 特异性说明、64KB 上限、`custom_head_html` 高级注入点文档化、稳定选择器/变量清单扩充、AI 模板补「安装与生效要点」。
- **管理面板文案纠偏**：「刷新即识别」→「重启服务后识别」；自定义 CSS/JS 标注 64KB 上限与 JS 时机提示。
- **CONTRIBUTING.md 上线**：语言翻译（i18n 全量键对齐流程）/ 主题包 / 代码与文档三类贡献路径，兼容红线与 AI 协作条款；README/AGENTS 索引接入。

## 2026-10-25（静态审计修复 + 英文 README + v0.1.0 首发）

- **三路静态审计**（部署构建 / 迁移 / 全文档噪音）对照源码逐条复核，修复文档硬伤：迁移手册签名钥导出 SQL 列名（老站 options 列为 `option_name/option_value`）、`--source-prefix` 必填语义、`skipped>0` 归因与处置（email/nickname CI 唯一键撞车、空邮箱→NULL）、纹理问题文件**不拷贝需人工补拷**、手动建表两处必炸（0009 重叠 ALTER 需 `--force`、import 强查 0008 记标）、`SKIN_LEGACY_ENV` 盐链挂载全流程补写（弱哈希用户登录的前置条件）、dry-run 通过标准改以 WARN 为信号、实跑后对账 SQL 与告警词典、`migrate uuid` 子命令入档、派生列/不迁列语义表。
- **部署文档**：SKIN_LISTEN 在 bridge 网络下的正确语义、模式一 4 处秘密同步、`SKIN_BASE_URL` 首启快照语义（纹理 URL/OIDC issuer 事后改不动）、env/option 分键优先级、GHCR 可见性与 `:sha-` 标签口径、回滚不回滚 schema 警示、两把密钥的生成时机条件、`/setup` 一次性门与迁移场景顺序、内置库 MySQL 8.0 标注、XFF 信任区与端口收敛、`X-Real-IP` 不被读取说明、systemd 完整单元。
- **CI**：PR 构建不再推 `:latest`（docker job push-only）；集成测试加 MySQL **5.7/8.0 双矩阵**（此前两起 5.7 专属 SQL 事故无门禁）；Go 版本钉 1.26 对齐 go.mod。**5.7 矩阵首战即抓两处**：`EmailByPlayerName` 的 `? COLLATE` 参数侧写法（5.7 prepared 协议 Error 1253）修到列侧、迁移 import 影子表 DDL 补 `utf8mb4`（5.7 默认 latin1 塞不下中文夹具）。
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
