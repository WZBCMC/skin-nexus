# SkinNexus — 仓库协作说明

> 供 AI agent / 新贡献者快速上手。权威依据：`docs/blueprint.md`（蓝图）、`docs/standards.md`（规范）。

## 结构

```
server/         Go 后端（模块 skin-nexus/server）
  cmd/skin-nexus/     入口：路由组装（yggdrasil / oidc / union / api/v1 / admin / public / textures / SPA）
  internal/ygg        Yggdrasil 协议核心（models/service/http/signer）—— 兼容面冻结区
  internal/oidc       yggc/OIDC 免密登录内置（zitadel/oidc 内核 + 自持 discovery/userinfo/device 页 + JWT token 双态）
  internal/union      MUA Union 成员站模块（可开关：三头验签 / 入站出站 / OAuth2 grant / UnionSigner）—— wire 契约冻结区
  internal/auth       12 条旧密码算式 + argon2id + AuthSource 注册表
  internal/texture    上传管线（IHDR 门禁→解码→重编码→内容寻址）+ 存储
  internal/migrate    迁移引擎（.env 解析 / uuid 五步清洗 / 纹理树审计）
  internal/store      MySQL 实现（ygg.Store / oidc.Backend 端口 + web/admin 查询）
  internal/webapi     站内 API（注册/登录 session/上传/角色）
  internal/adminapi   管理 API（X-Admin-Token 门禁）
  internal/publicapi  公开只读 API + legacy/usm/CustomSkinAPI 兼容路由
  internal/plugin     P0 HookAPI 合约（冻结）
  migrations/         DDL（InnoDB utf8mb4；0001 基础 / 0002 会话 / 0003 OIDC / 0004 Union）
web/            Vue 3 + Vite（npm ci && npm run build → web/dist）
poc/            M0 插件运行时 PoC（goja 已裁决冻结）
docs/           蓝图 / 规范 / 部署（公开）；开发过程文档在上级 docs/（本地）
```

## 铁律（docs/phase1-dev-plan.md §0 拍板）

1. **`/api/yggdrasil` 全路径原语义不可破**：204 清单、错误体 `{error,errorMessage}`、textures 空对象 `{}`、毫秒时间戳、PEM 严格格式——改动必须先跑 `go test ./internal/ygg/` 且不动契约断言。
2. **UUID 零漂移**：`OfflineUUID` 逐字节复刻旧公式；改名不重算。
3. **取证实测优先**：官方文档可能过时，以源码/线上 curl 为准。
4. 隐私：任何文件/提交不得出现真实姓名，署名用 `YMZS (YimingZhanshen)`。
5. **每轮开发收尾同步文档状态**：更新 README「状态/路线图（实时）」与受影响的必要文档（`docs/deploy.md` 等），过时即改、不留待办口吻的死条目——防止过时文档污染后续会话记忆。
6. **wire 契约冻结区零改动**：`internal/union` 的字段名/头名/拼接顺序与 hub 互操作冻结（见该包 doc 注）；`internal/plugin` hook 合约同理。确需变更先开偏差备案。
7. **数据兼容铁律（2026-10-23 拍板）**：任何持久化数据（表/列/字段语义）必须满足二者之一——①与 blessing-skin-server 及其插件生态的存量数据**同构可直迁**；②`internal/migrate` 提供**无损导入适配**。对既有代码与未来开发同时生效：密码凭证保持 12 算式可验证 + argon2id 惰性重哈希；新功能先考古老站存储约定（如 `invitation_codes.description`）再建表，不确定老站怎么存的先实测/读线上源码，不自造结构。

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
