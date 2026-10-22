# SkinNexus 主题创作规范（theme-spec）—— 含 AI 生成提示词模板

> 面向任何人（包括让 AI 代劳）创作 SkinNexus 主题包。**零构建原则**：主题 = theme.json + 纯 CSS（+ 可选 JS），
> 不需要 Node/打包/编译，放入 `data/themes/<id>/` 即生效。
> 加载器实现：`server/internal/theme/theme.go`（校验规则以代码为准）。

## 1. 主题包布局

```
data/themes/<id>/
  theme.json      # 必需：清单（字段见 §2）
  style.css       # 常规入口样式（inject.css 引用）
  assets/         # 可选：图片/字体等静态资源
  preview.png     # 可选：预览图（面板展示）
```

- `<id>` 只能是 **小写字母/数字/连字符/下划线**，首字符为字母或数字（`^[a-z0-9][a-z0-9_-]*$`），**且必须与 theme.json 的 `id` 字段一致**。
- 资源文件名同规则（允许点号扩展名）；目录穿越（`..`、绝对路径）会被加载器直接拒绝。

## 2. theme.json 字段

```json
{
  "id": "dark-elegance",
  "name": "暗色优雅",
  "version": "1.0.0",
  "author": "your-name",
  "description": "一句话描述主题风格",
  "extends": null,
  "inject": { "css": ["style.css"], "js": [] },
  "slots": {},
  "palette": { "primary": "#6366f1" },
  "requires": { "skin_nexus": ">=0.1" }
}
```

| 字段 | 必需 | 说明 |
|---|---|---|
| `id` | ✅ | 与目录名一致 |
| `name` | ✅ | 展示名 |
| `version` | ✅ | 语义化版本 |
| `author` / `description` | | 展示用 |
| `extends` | | 父主题 id（继承其 CSS，P2 预留） |
| `inject.css` / `inject.js` | ✅ | 按序注入的包内文件列表 |
| `slots` | | 可替换区块（P2 预留，见 §5） |
| `palette` | | 声明式色板（文档/预览用） |
| `requires.skin_nexus` | | 版本约束，加载器校验 |

## 3. CSS 变量契约（设计令牌）——优先改变量，别硬编码

SkinNexus 暴露如下 CSS 自定义属性；**主题应优先覆盖变量**，选择器只在必要时使用：

```css
:root {
  --bg: #0f172a;          /* 页面背景 */
  --fg: #e2e8f0;          /* 主文字 */
  --card-bg: #1e293b;     /* 卡片背景 */
  --primary: #6366f1;     /* 主题色（按钮/链接） */
  --border: #334155;      /* 边框 */
  --danger: #ef4444;      /* 错误/危险 */
  --success: #22c55e;     /* 成功 */
  --radius: 12px;         /* 圆角 */
  --font-sans: system-ui, sans-serif;  /* 字体 */
}
```

变量集以 `web/src/style.css` 的实际定义为准（AI 生成前请对照最新版）。

## 4. 选择器稳定区（安全区）

✅ **允许**：基于语义类名与 data 属性样式化——`.card`、`.btn`、`.secondary`、`.badge`、`.muted`、`.error`、`.success`、`table`、`input`、`select`。
❌ **禁止**：依赖 Vite 构建生成的 hash class（如 `index-BlotKZVo`）——它们随构建变化，主题会碎。

## 5. 插槽协议（P2 预留）

`slots` 声明可替换区块，值为 HTML 片段：

```json
"slots": { "home_hero": "<h1>欢迎来到我的皮肤站</h1>", "footer": "<small>…</small>" }
```

骨架期字段已解析但不渲染；待渲染端落地后另行公告。

## 6. 安全边界

- 主题 JS 与 admin 自定义 JS **同一信任面**：原样注入、无沙箱——只安装/使用你信任的主题。
- 主题包不包含任何服务端代码；加载器只读 theme.json + inject 列出的文件 + assets。
- 分发主题时请附带源码仓库链接，社区主题建议开源（MIT/GPL 均可）。

## 7. 🤖 给 AI 的完整提示词模板（直接粘贴给任意 LLM）

```
请为 SkinNexus 皮肤站生成一个主题包，要求输出两个文件：

1. theme.json —— 遵循以下 schema：
   {"id":"<小写字母数字连字符，如 my-theme>","name":"<中文展示名>","version":"1.0.0",
    "author":"<作者>","description":"<一句话描述>",
    "inject":{"css":["style.css"],"js":[]},"palette":{"primary":"<主色>"}}

2. style.css —— 纯 CSS，无需编译。规则：
   - 优先覆盖 CSS 变量，不要硬编码颜色到具体选择器：
     --bg（页面背景）、--fg（文字）、--card-bg（卡片）、--primary（主题色）、
     --border（边框）、--danger、--success、--radius（圆角）、--font-sans（字体）
   - 只使用语义选择器：.card、.btn、.secondary、.badge、.muted、.error、.success、
     table、input、select、a、h1/h2。禁止使用构建生成的 hash class。
   - 风格描述：<把你的需求写这里，例如「暗紫色赛博朋克风、大圆角、柔和阴影」>
   - 保持可读性：正文对比度不低于 4.5:1。
   - 不要引入外部资源；如需字体只用系统字体栈。

输出格式：两个代码块分别标注 theme.json 和 style.css，不要额外解释。
```

生成后：建目录 `data/themes/<id>/`，放入两个文件，admin 面板「外观」选择该主题保存即生效。

## 8. 版本与兼容

- `requires.skin_nexus` 支持 `">=0.1"` 形式；加载器不满足时跳过该主题（面板可见但不可选）。
- 主题 API（变量/选择器稳定区）遵循语义化版本：破坏性变更只在 minor+1 时发生并在本文档标注。
