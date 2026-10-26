# 变更日志

> 粒度 = 每轮开发收尾。README「状态」行只保留一句话摘要，逐轮细节沉淀在此文件（与 README 状态行每轮同步维护）。

## 2026-10-26（晴空琉璃二修：dock 单行化 + 形态全明暗 + 背景断层 + 缓存版本号事故根治）

- **dock 单行化**：v3.2 的 dock 仍是 wrap（用户截图两行大板）→ `nowrap + overflow-x auto` 推广到全宽度（v2 移动横滚形态），999px 胶囊条，user-chip `flex: none` 行尾。
- **形态与色系分态**：v3「换脸」段（§20）曾把形态规则也限定亮态——系统暗色 + theme.js 跟随系统时只见 dock 不见换脸。追加 §20b：**形态换脸（宽幅/22px 玻璃/发丝线/胶囊/圆角）全明暗生效，色系（亮底/渐变字）才分态**。
- **背景横带断层**（用户亮态截图）：body 线性渐变 480px 收尾后露底色 → 改三层光斑 `background-attachment: fixed` 锚定视口 + 纯色底，页面任意高度无断层。
- **缓存版本号事故（无语根因，如实认账）**：用户多轮强刷仍见旧样式——服务器直读已是新 CSS，但注入链接 `?v=3.0.0` 未随文件变更而变，浏览器按版本参命中旧缓存。**theme-spec 要点③白纸黑字「改样式后升 version」，我连改多轮未 bump，踩了自己写的规范**。根治：`scripts/upload-azure-theme.ps1` 重写为**自动 bump patch 版本**再上传（版本与文件强绑定，杜绝再犯）；线上已验证注入链接换 `?v=3.0.3`。
- **验证**：容器内 `grep 3.0.3` 命中、style.css 34032B、`/login` 注入 href=`?v=3.0.3`、landing 200。

## 2026-10-26（晴空琉璃 v3「换脸」：差异最大化样板主题 + 主题包能力边界终版规范）

用户总纲：晴空琉璃须为自定义主题包**典范/样板**——不看文字、单看布局样式排版，完全认不出与原站是同一个网站；规范可重制。且经记忆召回确认，早前 v3.1「把 dock 恢复垂直侧栏」**违背了 v2 立项初心**（10-25 拍板原文含「左栏改为顶部悬浮导航带」结构层改造），当晚全部返工：

- **azure-glass v3.0.0「换脸」段（§20）**：差异轴逐条对冲原站——**默认亮色晴空**（原站暗色黛青，最大一眼差；明暗开关暗态双套保留）· **超大无衬线 + 晴空渐变标题**（原站宋体月白紧凑）· **1180px 不对称宽幅**（原站 960 居中窄栏，大屏 1380/1560/1780）· **22px 玻璃卡 + 顶部渐变发丝线**（原站 10px 方角暗面）· **顶部悬浮 dock**（初心保住，v3.2 共存契约：`calc(var(--topbar-h)+8px)` 让出顶栏高度与 z 层）· 认证分栏亮空场景板（山色换亮蓝、雾带裁掉）· 胶囊化表单/按钮/徽章/拖放区 · 落地页 CTA/入口胶囊 · 斑马纹表格。theme.json 升 3.0.0。
- **规范终版「主题包能力边界」**（theme-spec 第三拍板，如实串联当日口径演进）：**主题拥有布局与排版的完全改造权**（布局/排版/色板/组件形态/背景/动效/明暗策略全放开）；底线四条（logo 品牌资产、内容语义、注入锚点、可访问性：focus-visible/对比度4.5:1/reduced-motion/无装饰 emoji 图标）；共存契约三条工程纪律（顶栏让位、新类自洽、修共存不砍结构）。
- **生产落地**：`docker exec -i` base64 写入 `/data/themes/azure-glass/`（style.css 32378B），容器重启，线上确认 3.0.0 与换脸段命中、站点 200。

## 2026-10-26（晴空琉璃主题 v3.1 结构适配 + 主题适配口径入规范——批注第六轮）

用户线上启用 azure-glass 主题后截图报修：v2 的「差异最大化」骨架（§10 把侧栏变顶部横向 dock、管理侧栏横排胶囊）与 v3 的 sticky 顶栏 + 垂直图标侧栏**叠成双层混乱**。

- **拍板：修主题不重写规范**（theme-spec v3 节刚立，补口径即可）。azure-glass `style.css` 追加 §19「v3.1 结构适配」段（双类同特异性、后定义者胜）：恢复 `.user-shell` 行向、`.user-side/.admin-nav` 垂直骨架与 v3 位置契约，**保留玻璃材质 + 胶囊渐变 + 活动光晕的美学差异**（active 左条隐藏，渐变胶囊已表意）；`.user-chip/.auth-pane 卡/.drop-zone/.prev-box` 玻璃化；`theme.json` 升 v2.1.0。
- **规范**：theme-spec 新增「主题适配口径：布局骨架归全站，差异走材质」节——`.topbar/.user-shell/.user-side/.admin-shell/.auth-split` 等结构类只做材质与装饰覆盖，布局级变形（dock 化等）会与外壳对撞；主题差异化的安全表达位清单（色板/玻璃/圆角/阴影/动效/纹理/字距）。
- **生产落地**：主题在 named volume `skin-data:/data`（容器内 `/data/themes`，此前查宿主 `/opt/skin-nexus/data/themes` 为空的取证已修正）；base64 分块经 `docker exec -i` 写入（首轮漏 `-i` 导致 0 字节，已修），容器重启后面板重载，线上确认 `v3.1` 命中、站点 200。

## 2026-10-26（上传纹理卡重构：拖放区 + 选肤即 3D 预览——批注第五轮）

批注「上传皮肤 ui 可以优化，另外当用户选择皮肤后可以前端渲染出皮肤的 3d 模型方便预览即将上传的皮肤」。

- **上传卡双栏布局**：左控件列（类型/模型 select + **拖放上传区** + 上传按钮 + 提示），右**即时预览列**（220px 预览框，≤720px 塌缩）。拖放区 dashed 赭石线、dragover 金边高亮、点击即选文件、选中显示文件名 + 清除钮；拖放与点击两条路都写入同一 file input（FormData 提交路径不变）。
- **选肤即 3D 预览**：选中 PNG 后 `URL.createObjectURL` 喂给 `SkinPreview`（skinview3d 动态 import）即时渲染自转模型；**模型切换（Steve/Alex）热更新** `playerModelType`；披风无 body 渲染 → 平面像素图预览；WebGL 不可用时自动降级平面图（探测 + `gl-unavailable` 回落）。objectURL 全生命周期回收（换选/清除/上传成功/卸载），viewer 卸载 `dispose()` 守住 WebGL context 预算。
- **SkinPreview 组件增强**（向后兼容，PlayersPage 共用）：新增 `modelType` prop 热切换、WebGL 探测、卸载释放。
- **验证**：`vite build` 绿；`zh_design_lint` 清零。

## 2026-10-26（Union 黑名单答疑 + 补齐翻页与「有效期至」列——批注第四轮）

批注三问（管理面板 → Union → 联合黑名单）：「就20条？没设计翻页？封禁日期和解封日期老站有吗？」。回源取证（老站 `refs/yggdrasil-connect` + 线上 hub 实测）后回答并就地补齐：

- **答疑结论（源码/线上双实证）**：①上游 MUA hub（`skin.mualliance.ltd/api/union/blacklist/query`）是标准 Laravel 分页，线上实测 `total=30、per_page=15、last_page=2、next_page_url` 齐备；我们 API 层一直透传 `page`，但**UI 翻页控件漏渲染**——页面停留第 1 页；②老站 `views/blacklist.twig` 表头 = ID/email/source/reason/**created_at（加入时间）/valid_until（有效期至）**/操作——**老站有「加入时间 + 有效期至」，没有独立「解封日期」**（释放是即时动作，上游不落解封时间；`UnionBlacklistController.php` 纯透传，结构由 hub 定）。
- **取证插曲（如实记录）**：首轮取证 `total` 读出 ≤0，实为 token 未注入、403 空对象的假信号；带 `X-Admin-Token` 重取后 30/15/2 全字段命中。
- **补齐**：黑名单表加「有效期至」列（`valid_until`，中英 `unionBlUntil`）+ 分页控件（上一页/页码/下一页）；总页数**优先取上游权威 `last_page`**（`per_page` 由 hub 定、非固定，写死 20 会算错），缺失时退化 `total/per_page` 推算；查询时重置首页。
- **验证**：`vite build` 绿；`zh_design_lint` 清零；生产实证 page=1 15 条 / page=2 15 条、total=30。

## 2026-10-26（v3 续②：公告 markdown 渲染器拍板换 markdown-it + 排版升 v3——批注第三轮）

批注「markdown 渲染可以修复，或者找个更棒的渲染器（待拍板）」（仪表盘公告裸 URL 未链接化、排版粗糙）。

- **渲染器拍板：marked → `markdown-it`**（`html:false, linkify:true`）——公告是管理员富内容区，markdown-it 的 linkify 对中文行内裸 URL 识别是内置强项（批注圈中的病灶：`皮肤站(https://…)` 原样输出）；GFM 表格/删除线覆盖更全；体积与维护活跃度同级。**消毒双层不变**：客户端 DOMPurify 仍兜底、服务端 bluemonday 仍是一层，`html:false` 堵注入面。
- **公告排版 v3**：宋体大标题（衬线 + 赭石发丝底线）、1.75 行高、8px 段落节奏、列表项距、青瓷下划线链接、引用赭石左线、表格线框；内容里的 emoji 属公告作者表达，不剥离（zh-ui 禁的是 UI 图标用 emoji，不是内容）。
- **连带归位**：该组件其余样式全部对齐 4px 网格 + token 引用（`.ali`/`.drag-btn`/`h3`）；两处「标题+副行」改事实块结构。
- **验证**：`vite build` 绿；`zh_design_lint` 清零（28 → 0）。

## 2026-10-26（v3 续：皮肤详情页 + 404 页统一入「月下仙山」——用户浏览器批注轮）

浏览器批注「皮肤详情页请优化，按照规范统一设计」（/skinlib/{tid} 截图仍为旧蓝按钮语言）。就地换血：

- **详情页**（`server/cmd/skin-nexus/texturepage.go` 内联样式）：旧蓝按钮/灰卡换 v3 国画色——黛青底、宋体标题（`h1` 衬线）、**朱砂主按钮**（下载纹理，hover 浮起+光晕）、描边次按钮（hover 赭石线）、元数据格加**赭石回纹角饰**（与落地页统计卡同款）、页脚赭石发丝线、青瓷链接、选区/焦点/减弱动效齐备。theme-color `#2f66ff` → `#16202e`。结构与 SEO 要素零改动（JSON-LD / itemscope / viewer.js 渐进增强 / 注入锚点 / 测试断言串全保留——主题与 custom CSS 仍可整体覆盖）。
- **404 页**（材质不存在/私密）同语言刷新：黛青满屏 + 宋体 404 + 青瓷回链。
- **落地页「进入皮肤库」入口升级**（第二条批注）：裸文字链 → 描边次按钮（赭石线、hover 金边 + 嵌套面底），与 CTA 描边款同语言。
- **验证**：`go test ./cmd/skin-nexus/` 绿；`zh_design_lint` 清零；生产实证 `/skinlib/79` theme-color/朱砂/宋体串命中、落地页入口新样式命中（换镜像后核验，Image ID 与容器一致）。

## 2026-10-26（v3 全站视觉革新「月下仙山」：token 架构 + 布局换代 + 动效体系 + 三主题适配）

用户点单「大刀阔斧的视觉革新，布局/动画/拍板/设计要素优化，可酌情改 UI 规范并让现有 3 主题适配」。拍板：

- **token 架构 v3**（`web/src/style.css` 全量重写）：旧 7 契约键保持为唯一真源（默认值换国画色：黛青 `--bg:#101825` / 石面 / 赭石发丝线 / 月白 / 朱砂主操作），**新语义键全部别名到契约键**（`--ink/--ink-2/--line/--moon/--cinnabar` 等）——主题覆盖旧键即自动联动，旧主题零改动兼容。新增派生键：`--ink-3/--line-strong/--jade`（青瓷：链接选中健康）`/--gold/--accent-wash/--sel/--fog/--glow-*/--ring-*`、圆角三档、阴影两层、动效令牌、场景三层山色。
- **布局换代**（`App.vue`）：① sticky 毛玻璃**顶栏**（N 徽品牌区 + 站名宋体 + 语言/会话操作）取代原标题行；② 登录/注册/找回/重置从居中小卡改为 **分栏场景式**（左=远山 SVG + N 牌 + 站名 + 定位句，右=表单卡，≤860px 塌缩；场景装饰 aria-hidden）；③ 用户侧栏**图标导航**（1.7px 线性 SVG 图标，8 枚）+ 活动态朱砂左条 + 底部**用户卡**（昵称 + 退出）；④ 视图切换 fade-rise 一次安静入场。
- **动效体系**：统一 `--ease` 缓动；按钮 hover 浮起 + 光晕、按下回落；输入焦点朱砂环；卡片悬停描线加深；表格行悬停薄雾；全局 `prefers-reduced-motion` 一刀停。
- **管理面板**（`AdminPanel.vue`）：侧栏对齐新语言（朱砂活动左条、圆角刻度、动效令牌）。
- **三主题适配**（`themes/`）：azure-glass / ember-forge / verdigris-atelier 各追加「v3 语义键适配」小节（派生键主题化取值 + `.topbar/.auth-scene` 结构类适配；azure 补玻璃顶栏）。theme-spec 新增「v3 全站设计体系」节（token 架构、新键清单、结构类契约、**logo 品牌资产红线**）。
- **验证**：`vite build` 绿；`zh_design_lint` 清零（含清理本组件新债：内联宽度改类、场景 SVG fill 走 token、标题副行改事实块）；模板链完整性核对（error 横幅与视图条件链解耦）。

## 2026-10-26（前端设计体系落地：SPA token 化 + 落地页「月下仙山」+ 访客免登录逛皮肤库）

- **能力底座**：装齐前端设计技能与插件（frontend-design / extract-design-system / zh-ui-design / unocss 四技能落盘 `~/.dsh/skills`；ui-ux-pro-max、dsh-zh-ui、dsh-web-review 三插件入 desktop profile——zh-ui release tgz 校验和与 README 不符，经内容级核验放行）。
- **SPA 全站样式（`web/src/style.css` 换血）**：「方块工作台」token 体系——材料语义色（绿宝石=可点击/健康、钻石蓝=信息/焦点、金锭=警示、红石=错误）、4px 网格字号阶（12/14/16/20/24/32）、方块圆角、`tabular-nums` 表格数字、全局 `:focus-visible` 焦点环、`prefers-reduced-motion` 全局尊重；**保留全部既有类名**（组件零破坏），主题扩展机制不变。`zh_design_lint` 8 条 findings 清零。
- **落地页「月下仙山」（用户 brief：中国古风仙侠）**：自有 token 命名空间（黛青 `--ink` / 月白 `--moon` / 朱砂 `--cinnabar` 只给主按钮 / 赭石 `--ochre` 纹线，无大红大紫大金）；签名元素 = N 字牌（保留 1600ms 入场编舞，**保持原品牌蓝绿配色**——首版曾单方面换朱砂色，用户拍板回滚：logo 属品牌资产，变色须连同 favicon/PWA/README 全套统一并由本人终审，不单点修改）；新增远山云雾 SVG 场景（fixed 垫底 + 两层雾带缓漂，reduced-motion 停）；统计卡赭石回纹角饰、祥云 repeat-x 分隔、宋体标题栈（系统字体，零外部加载）；theme-color → `#16202e`。注入锚点不动，主题包/custom CSS 仍可整体覆盖——theme-spec 已补「落地页 token 命名空间」节。文案清理：去「无缝」（禁用词）与「— ·」双连接符。
- **访客免登录逛皮肤库**（用户点单）：后端 `/api/v1/skinlib` 本就匿名只读（仅公开材质），本轮拆前端登录墙——`/skinlib` 对匿名直达开放、顶栏给访客「皮肤库 / 登录 / 注册」入口、落地页精选区新增「进入皮肤库」链接；「只看我的」对访客隐藏；未登录点赞后端 401「请先登录」如实提示。登录用户仍走带侧栏布局。
- **验证**：`go vet` 干净、全仓 `go test` 绿（landing 断言全兼容：hero/CTA/注入/编舞串保留，footer 位置约束满足）；`vite build` 通过；`zh_design_lint` 清零。

## 2026-10-26（对照表勘误：blessing 有完整皮肤库，两格就地修正）

- **用户指出**：README 对照表「blessing ❌ 无皮肤库」不实。
- **复核确认（用户对，原表错）**：blessing 核心有完整皮肤库——`/skinlib` 列表（`skinlib/index.twig`）、**单品页 `/skinlib/show/{tid}`**（`SkinlibController::show` + `show.twig`）、`/skinlib/upload`、`/skinlib/list` JSON API、举报/改名/隐私/点赞全套。原表错因：grep 用 element-skin/vSkin 的路由命名 `skin-library`，blessing 的命名是 `skinlib`，跨项目对照把「自家命名先验」当成了检索式。**教训：对照检索必须用对方自己的命名空间找路，不能用自己的命名先验套对方**。
- **连带复核（另两格同步降级/修正）**：① blessing 的 skinlib 页面是 Twig 壳——title 逐页服务端出（`show.twig:3`），但正文由客户端 Vue 从 `blessing-extra` JSON blob 渲染，description 是全站一个值（`HeadComposer.php:75`），OG/JSON-LD/canonical/sitemap/robots grep 零命中 → 「服务端渲染内容页」格由 ❌ 改 ⚠️，「SEO 可见性」格由 ❌ 改 ⚠️；② blessing 核心无 Mojang fallback（grep 仅 locale fallback 噪声）→ 该格 ❌ 维持；③ 「独有领先区」改「领先区」并逐格重写：完整 SEO 体系（blessing 部分有、无完整实现）、迁移链与观测面（三家全无）。
- **交付**：README（zh/en）表格两格修正 + 领先区段重写 + 表头勘误透明注记。

## 2026-10-26（管理面板站点设置分组化 + 交互修正三连）

- **需求**（用户授权自决，点名方向「管理面板选项的分组」）：设置页 35 个键平铺一屏难找；顺带修同类交互。
- **分组化**：后端 `settingsWhitelist` 每键增 `group` 字段（basic 站点基础 / register 注册与验证 / mail 邮件服务 / security 安全与限流 / ygg Yggdrasil 服务 / texture 角色与材质 / union Union 联盟），`GET /api/admin/v1/settings` 响应带 `group`；前端按固定顺序分节渲染（分节标题 i18n 双语、分隔线样式、未知 group id 兜底排尾——API 前向兼容）。
- **交互修正三连**：① `ygg_mojang_fallback` 进布尔复选框白名单（此前是裸文本框填 0/1 易误填；PUT 校验同步纳入 normalizeBool 分支——非法值 400 而非静默存垃圾）；② `smtp_pass` 进密钥掩码集合（此前 SMTP 授权码以明文 input 渲染，与 union_member_key 同等待遇）；③ 分组顺序把邮件(SMTP)排在安全限流之前——找回密码配置路径更顺。
- **验证**：`go vet` 干净、全仓 `go test ./...` 绿（adminapi 重编译重测通过）；`vite build` 通过。

## 2026-10-26（注册/找回表单按站点真实配置收敛）

- **需求**（用户点单）：开启邀请码则注册页出现**必填**邀请码输入框，关闭则整个不出现（不再常驻「选填」死框）；邮箱同理——未配置邮件服务时，注册页邮箱输入与找回密码整条不可用。
- **后端**：新增公开端点 `GET /api/v1/site-config` → `{invite_required, mail_configured}`。`invite_required` 复用 `inviteRequired()` 的面板/env 解析；`mail_configured` = `smtpConfig() != nil`，与 forgot 的判据同源（含 From/User 双缺判不可用的既有口径）。纯配置态，无用户数据泄露——两个旗标本可由 register-400 / forgot-503 行为观测到。
- **前端**（App.vue + api.js + 双语 i18n）：boot 时拉取，失败降级 = 旧 UI 原样。注册卡：`invite_required=1` → 邀请码框带「必填」强调标签 + 提交前本地校验（免打一轮 400）；`=0` → 输入框整个不渲染。`mail_configured=0` → 注册邮箱框隐藏、登录页「忘记密码？」链接隐藏、直达 `/forgot-password` 显示「站点未配置邮件服务」不可用卡、reset 失效分支的找回按钮同步隐藏。
- **边界**：服务端语义零变化（注册邮箱服务端仍可选；invite 门禁仍由 handleRegister 400 把守；forgot 未配置仍如实 503）——本轮只做 UI 与配置对齐。
- **验证**：`go vet ./...` 干净、全仓 `go test ./...` 绿（webapi 包编译通过，无测试文件系仓规现状）；`vite build` 通过。

## 2026-10-26（README 竞品对照矩阵：三家源码实证差异入宣传面，双语同步）

- **需求**（用户点单）：结合竞品研究文档与记忆召回，重新对比 blessing-skin-server / element-skin / vSkin 与本站差异，作为宣传点写入 README，英文版同步修改。
- **复核取证（当日重新 grep，非转抄旧报告）**：两家新秀路由表（`element-skin/src/router/index.ts`、`vskin/src/router/index.ts`）均只有 `/skin-library` 列表、无单品路由；三家（含 blessing PHP/Blade 源码）`sitemap|robots` 零命中；element-skin 全仓 `union|Union` 零命中（Union 未实装）；element-skin 有权限审计（`permission_audit_logs`）但无访问/通信日志；blessing 封禁为权限位 + 纹理上传 403（无审计日志表）；blessing `/user/closet` 实存——PCL 深链约定发源地，对照表如实给 ✅ 不贬低。
- **交付**：README 新增「与主流开源皮肤站的对照」12 行矩阵 + 独有领先区段（SEO/SSR、单品页、迁移链、观测面四张独有牌 + 2C/3.5GB 实测性能账）；README.en.md 同步英文矩阵，并补齐状态行（原停在 2026-10-25，缺兼容矩阵/详情页+深链/A1/A2/C1 五个里程碑）。
- **边界**：只写我方与对方能力事实，不写对方安全弱点评价（沿用 competitor-analysis §4「内部备忘不外宣」口径）；对照结论全部可回溯到 [`docs/competitor-analysis.md`](competitor-analysis.md)。

## 2026-10-26（C1 运维观测面：访问与通信日志全链路）

- **需求**（用户点单）：网页/MC 启动器登录、MC 服务器↔皮肤站、MUA Union↔皮肤站通信全部落日志，运维在管理面板前端即可了解情况（UA/IP/时间等）。
- **数据面**：`0011_access_logs` 迁移新增 `access_logs` 表（kind/outcome/uid/who/ip/ua/detail/created_at，kind×time、who、uid、time 四索引）；**保留 90 天**，启动期 6h 定时清理。与 `audit_log` 分工：audit 记「安全动作」，access_logs 记「流量事实」。
- **旁路记录铁律**：`internal/accesslog.Logger` nil 安全（零值与 nil 均不记录）、sink 写失败吞掉、字段 rune-safe 截断（who 64/ip 45/ua 255/detail 255 字节，绝不劈开 UTF-8）、ctx detach（客户端断连仍留痕）——**协议面响应语义零变化**（ygg 处理器级测试断言无 logger 时行为不变）。
- **采集点**：
  - 网页登录六结局（成功/未知账号/密码错/锁定/限流/校验错误）、注册四结局（成功/占用/邀请码无效/IP 上限）。
  - 启动器侧：authenticate（含 auth_gap/锁定/限流）、refresh、validate；join 三因（email_unverified/player_mismatch/invalid_token）；hasJoined 与 profile 标记数据来源 `src=local/official/none`——**A2 兜底何时命中官方一目了然**。
  - Union 入站：`requireUnion` 包装捕获状态码（模块禁用时的调用也记——misconfig 信号）；出站：`doJSON`/`doProxy` 单点包裹（method/path/status/耗时）。
- **面板**：新增「访问日志」页（类型下拉 + who/IP/详情子串搜索 + 分页 + 彩色结局徽章）；admin API `GET /api/admin/v1/access-logs?kind=&q=&page=`。
- **测试**：accesslog 包单元测试（nil 安全/rune 对齐截断/sink 失败吞/断连留痕）；ygg 处理器级四测（profile/hasJoined 三来源、authenticate 结局、join 三因、无 logger 不变）；store 集成测试（插入/类型筛选/子串搜索/分页序/uid 回传/过期清理）。全仓 `go test` 绿、`vite build` 通过。
- **上线即测即修（用户看图反馈）**：① UA/详情列 `word-break: break-all` 被自动表格布局压成一字一行（行高爆表）→ `table-layout: fixed` 定宽列预算 + 单行省略 + hover title 看全文 + 全单元格统一防溢出，时间列加宽 160px 防截秒；② 时间显示偏 8 小时——DSN 无 `loc=`，驱动把 DB 墙钟数字标成 UTC，前端 `Date()` 按 Z 重解读 → 改为面板惯例直显数字（不重解读时区）。
- **部署实况修正（取证）**：`/opt/skin-nexus` 无 `.git`，compose 为 `image: ghcr.io/yimingzhanshen/skin-nexus:latest`——真链路是 **CI 推镜像 → 部署机 `docker compose pull && up -d`**（deploy.md 一直如此写）；此前各轮脚本的 `git pull | tail` 失败被管道吞掉、实际部署一直由 `up -d` 隐式 pull 完成。本轮起脚本改 pull 语义，阶段拆分防任务过期丢证据。
- **线上验证（2026-10-26 01:47 真实输出）**：迁移水位 `0011_access_logs.sql`；真实流量四类入表——`web_login/fail/c1r2-nobody/C1R2-Firefox` + `ygg_hasjoined/miss/Notch/Paper-1.20.4/src=none` + `ygg_profile/miss/HMCL/3.9` + **`union_out/ok/GET /blacklist/query status=200 dur=39ms`（真 hub 出站自动入账）**；kind 筛选 total=2；landing/metadata/skinlib 全 200；累计 24 行；容器 healthy；时间显示实钟（01:47:21，修复前为 +8h 的 09:47）。

## 2026-10-26（A2 Mojang 官方兜底：profile/hasJoined 本地未命中 → 官方 sessionserver）

- **动机**：两家竞品都有 fallback 链，本站在「服务器指向本站 authlib-injector、玩家却用官方账号」的场景会把人踢下线（本地未命中即 204）。补最小版：`profile/{uuid}` 与 `hasJoined` 本地未命中时向 `sessionserver.mojang.com` 串行查询一次，命中即原样透传官方 JSON（字节兼容），未命中维持原 204——**只扩展原本为空的分支，本地命中路径一字未动**。
- **威胁模型（element-skin 逃逸路径教训 + 本站证据铁律）**：目标 origin 是编译期常量；用户输入只进 path（严格 32 位小写 hex 校验——`NormalizeUUID` 只做小写去横线**不证明 hex**，URL 拼接前必须自证）或 query（`url.Values` 转义 + 用户名限制官方 `[A-Za-z0-9_]{1,16}` 字符集，站内中文名**永不出网**）；**不跟随重定向**（`ErrUseLastResponse`，被劫持的边缘无法把请求弹回内网）；**仅公网拨号**（`publicDialContext`：loopback/RFC1918/链路本地/CGNAT 100.64/10/未指定地址一律拒接——污染的解析器也打不进宿主或校内网）；响应 **1MiB 上限 + 必须是含 id/name 的 profile JSON**（ captive portal 的 HTML 死在门口）；**4s 硬超时**。任何失败一律坍缩为「无兜底」——空 204 好过错数据。
- **热开关**：面板键 `ygg_mojang_fallback`（默认开，env `SKIN_MOJANG_FALLBACK` 可改首启默认），保存即生效；关=行为与旧版完全一致。
- **签名策略镜像**：本地 handler 默认剥签名（`unsigned` 非 `false` 即 strip），兜底请求同样仅在 `unsigned=false` 时向官方要签名（`unsigned=true` 反向透传），客户端无感。
- **测试**：进程内 stub 覆盖透传/路径归一化/非 hex 拒发（含 `../` 穿越）/hasJoined 参数转发与坏 IP 丢弃/中文名零出网/禁用态零请求/七种响应门（404、500、204、HTML、无 id-name JSON、超限、302 不跟随且跳转目标零命中）/公网拨号器九地址拒接表/`isPublicIP` 十四例/处理器级（本地未命中→官方200、本地命中不惊动上游、开关关→回204）。全仓 `go test` 绿。
- **线上实测（2026-10-26 取证）**：出网可达 `sessionserver.mojang.com`（DNS 正常、同族 `hasJoined` 204 应答正常、往返 0.3-0.9s），但 `session/profile/{uuid}` 对全部受测真实账号（jeb_、Hypixel、Dream、tommyinnit、sapnap、Notch）一律返回 Azure FD 404 HTML——浏览器/自定义 UA、IPv4/IPv6、HTTP/1.1、`unsigned` 参数逐一排除，系**该路径从本网络区域被软 404**（区域策略级）。实证结论：① 兜底机器确实发起上游请求（0.29-0.44s 往返后闸门正确拒掉 HTML 404 → 204，不编造数据）；② profile 兜底在本部署只能走「快败」路径（每次未命中 +0.3s，可接受，开关可关）；③ hasJoined 所在端点族可达，是实际有价值的一腿（正向 200 需真实官方 join 记录、无法按需复现，由单测覆盖）；④ 本地优先（真实角色 `wine_fox` 本地 200 名字精确匹配、单测证明不惊动上游）、垃圾输入闸门、面板热开关（1↔0↔1）均线上实证。

## 2026-10-26（A1 找回密码：SMTP 邮件流 + 忘记/重置密码全链路）

- **背景**：竞品评估（docs/competitor-analysis.md）确认「无密码找回、全站无 SMTP」是两家竞品与本站在易用性上的共同硬缺口——`verification_token` 列早已有之但邮件流从未实现。本轮补齐端到端。
- **邮件传输 `internal/mailer`**（纯标准库 net/smtp，零第三方依赖——数据主权/最小依赖红线）：465 隐式 TLS 直连、其它端口自动 STARTTLS 升级；PLAIN 认证由 stdlib 强制「非 TLS 不发凭据」；发件人支持 `Name <a@b.c>` 形式；主题 RFC2047 编码、正文 base64（76 列折行）；整会话 30s 硬 deadline 防黑洞服务器吊死请求；地址 CR/LF 注入直接拒绝（信封与头双重校验）。
- **重置令牌语义**（`store/password_reset.go` + 迁移 `0010_password_reset.sql`）：32 字节 crypto/rand 原文**永不落盘**，只存 SHA-256 摘要（库泄露≠可用水滴链接）；30 分钟 TTL 写入与校验同用 DB 时钟（`DATE_ADD(NOW(3))`/`NOW(3)`，Go 与 MySQL 时钟漂移无法延寿）；**单次使用**靠原子 UPDATE（`used_at IS NULL`）——重放双击/泄露链接二次使用直接找不到；再次申请自动作废旧令牌（最新邮件生效）；过期/已用行顺手清理，表有界无需 cron。
- **防枚举与限流**：`POST /api/v1/password/forgot` 已知/未知邮箱同形恒 200（发信失败也保持同形，SMTP 故障不变成存在性预言机）；未配置 SMTP 返回 503 明示配置态（非用户数据）；共享 Limiter 新键空间 `pwf-ip:`/`pwf-mail:`/`pwr-ip:` 滑窗限流（IP+地址双维度）。`reset` 端点 unknown/expired/used 三种失败**坍缩为同一条 400 文案**，不泄露状态。
- **重置成功 = 镜像改密吊销链**：`SetPasswordHash` 无条件写（chain-trace P0 守卫沿用）→ `InvalidateUserTokens` 杀全部 Yggdrasil 令牌 → `OnPasswordChange` panic 屏障杀 OIDC/启动器授权 → `DeleteUserSessionsExcept` 清全部网页会话（带本会话 cookie 则保留，与改密同规则）→ 审计 `password_reset` 行。
- **设置面板 SMTP 热配置**：`smtp_host/port/user/pass/from` 五键入白名单（发信时逐键读取，option 覆盖 env 默认，**保存即生效无重启**）；`smtp_pass` 审计脱敏（与 union_member_key 同列）；env `SKIN_SMTP_*` 作面板占位默认。
- **前端**：登录卡「忘记密码？」→ 找回卡（输入邮箱→统一「已发送」文案）；`/reset-password?token=…` 真实 URL（邮件里回跳，boot 按路径识别，与 /device 同款探法）→ 重置卡（双输密码前端校验→成功态→去登录）；`/forgot-password` 可分享直链；中英 i18n 全键。
- **回归测试**：mailer 进程内假 SMTP 会话端到端（信封去显示名/主题 RFC2047/base64 正文回环/CRLF-only）、注入拒绝（收件人 CRLF 不拨号）、禁用态/nil 接收者、envelope 解析表、隐式 TLS 端口判定；store 集成（单次消费/重放拒绝/未知摘要/过期回拨/最新邮件胜出/UserByEmail 大小写与「昵称不是邮箱」）；前端 `vite build` 通过。
- **上线即测即修（实证驱动）**：生产翻转测试发现只配 `smtp_host` 不配发件人时，From 为空导致每次发信死于 `empty address`——但用户侧收到的是 200「已发送」**假成功**（失败只落日志）。修法：`smtpConfig` 在 `smtp_from` 与 `smtp_user` 皆空时视为未配置返回 503（配置态与地址无关，不泄露用户存在性）；面板 `smtp_host` 描述同步「须同时配发件人」。

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
