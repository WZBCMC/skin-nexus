# SkinNexus — 仓库协作说明

> 供 AI agent / 新贡献者快速上手。权威依据：`docs/blueprint.md`（蓝图）、`docs/standards.md`（规范）。

## 结构

```
server/         Go 后端（模块 skin-nexus/server）
  cmd/skin-nexus/     入口：路由组装（yggdrasil / api/v1 / admin / public / textures / SPA）
  internal/ygg        Yggdrasil 协议核心（models/service/http/signer）—— 兼容面冻结区
  internal/auth       12 条旧密码算式 + argon2id + AuthSource 注册表
  internal/texture    上传管线（IHDR 门禁→解码→重编码→内容寻址）+ 存储
  internal/migrate    迁移引擎（.env 解析 / uuid 五步清洗 / 纹理树审计）
  internal/store      MySQL 实现（ygg.Store 端口 + web/admin 查询）
  internal/webapi     站内 API（注册/登录 session/上传/角色）
  internal/adminapi   管理 API（X-Admin-Token 门禁）
  internal/publicapi  公开只读 API + legacy/usm/CustomSkinAPI 兼容路由
  internal/plugin     P0 HookAPI 合约（冻结）
  migrations/         DDL（InnoDB utf8mb4）
web/            Vue 3 + Vite（npm ci && npm run build → web/dist）
poc/            M0 插件运行时 PoC（goja 已裁决冻结）
docs/           蓝图 / 规范 / 部署（公开）；开发过程文档在上级 docs/（本地）
```

## 铁律（docs/phase1-dev-plan.md §0 拍板）

1. **`/api/yggdrasil` 全路径原语义不可破**：204 清单、错误体 `{error,errorMessage}`、textures 空对象 `{}`、毫秒时间戳、PEM 严格格式——改动必须先跑 `go test ./internal/ygg/` 且不动契约断言。
2. **UUID 零漂移**：`OfflineUUID` 逐字节复刻旧公式；改名不重算。
3. **取证实测优先**：官方文档可能过时，以源码/线上 curl 为准。
4. 隐私：任何文件/提交不得出现真实姓名，署名用 `YMZS (YimingZhanshen)`。

## 常用命令

```bash
# 后端（Windows 开发机，GOPROXY 用国内源）
cd server
go build ./... && go vet ./... && go test ./...

# 前端
cd web && npm ci && npm run build

# 提交规范：feat|fix|chore(scope): 中文/英文摘要
```

## 接口契约优先

新代码通过既有端口（`ygg.Store`、`texture.Store`、`plugin.Host`）扩展，不直接散落 SQL——需要新查询时加方法到 `internal/store`。admin/web/public 各自独立 package，路由在 `cmd/skin-nexus/main.go` 统一组装。
