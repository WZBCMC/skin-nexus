# 从 blessing-skin-server 迁移（migrate 子命令手册）

> 目标读者：自行部署 SkinNexus 并要把既有 blessing-skin-server（老皮肤站）数据
> 全量迁过来的运营者。全流程实测于 2026-10-24（生产库 67 用户 / 67 角色 /
> 61 纹理 / 394 邀请码，零丢行、UUID 零漂移）。

## 一、迁移能带走什么

`skin-nexus migrate import` 按 blessing 存储约定逐表搬运，**主键保留**：

| 老站表 | 目标表 | 说明 |
|---|---|---|
| `users` | `users` | 密码哈希原样携带；`password_algo` 自动分类（bcrypt→`legacy:bcrypt`、argon2 PHC→`legacy:argon2`、其余→`legacy` 走 12 算式逐个试），登录时惰性重哈希为 argon2id |
| `players` | `players` | 名称/皮肤披风引用原样 |
| `textures` | `textures` | `type` 做 steve/alex→`skin`+`model` 映射；`name`→`description` |
| `uuid` | `uuid_map` | 过五步清洗（同名去重 / 同 UUID 去重 / pid 回填 / 孤儿剔除），**UUID 零漂移** |
| `options` | `options` | 站点配置（`INSERT IGNORE`，重跑不覆盖运营者后来的改动） |
| `user_closet` | `user_closet` | 衣柜（无主键表，`NOT EXISTS` 去重） |
| `invitation_codes` | `invitation_codes` | 邀请码（含 `description` 等老站扩展列） |

纹理文件按内容寻址**字节级拷贝**到目标纹理目录，拷贝后抽样 `cmp` 校验。
不迁：会话/令牌（`web_sessions`、`ygg_tokens` 等，登录态自然重建）、
老站插件表（`ygg_log`、`notifications`、`reports` 等无对应语义）。

## 二、执行序（实测路径）

> 原则：老站只读、目标先备份、先 dry-run 后实跑、跑完对账。

```bash
# 0. preflight（只读）：解析老站 .env 密码算式/盐槽，审计纹理树
skin-nexus migrate preflight --env /path/to/legacy/.env \
  --textures /path/to/legacy/storage/textures
#   预期：SALT slots aligned OK；texture scan: ok=<N> hashMismatch=0 badIHDR=0

# 1. 备份目标库与纹理目录（旧站侧不需要备份——全程只读）

# 2. 停目标站，清空业务/会话表（保留 union_kv、oidc_clients、
#    schema_migrations 这些站点身份/基建；MySQL 5.7 的 TRUNCATE 是单表语句，逐表跑）

# 3. dry-run：只读预演，对账 selected 数与老站 COUNT(*) 逐表一致、零 WARN
skin-nexus migrate import --source-dsn 'USER:PASS@tcp(127.0.0.1:3306)/wzbcmc?parseTime=true' \
  --target-dsn 'USER:PASS@tcp(127.0.0.1:3306)/skin_nexus?parseTime=true' \
  --source-prefix 'wzbcmc.' \
  --textures-src /path/to/legacy/storage/textures --textures-dst /data/textures \
  --dry-run

# 4. 实跑（去掉 --dry-run 即可；幂等，失败可直接重跑）
```

参数要点：

- **两侧 DSN 必须带 `parseTime=true` 且 `loc=` 一致**（引擎强制校验）。
- `--source-prefix` 是老站表前缀/库名限定（如 `wzbcmc.` 或 `legacy_`），留空则同库。
- `--textures-src` 给老站纹理目录；`--textures-dst` 给目标 `SKIN_TEXTURES_DIR`
  （compose 默认 `/data/textures`），不给则只迁库不迁文件。

## 三、切流断言（跑完必须核对）

1. **`users selected==inserted` 且 `skipped==0`**——skip>0 说明目标库有残留行
   主键撞车（引擎 INSERT IGNORE 会保旧弃新），此时清库重跑；
2. 各表 `inserted` 与老站行数逐一相等（实测 67/67/61/67/87/84/394）；
3. 纹理 `files copied=N` 且抽样 `cmp` 老站/目标同名文件逐字节一致；
4. **UUID 零漂移抽查**：取老站 `uuid` 表一行，对照
   `GET /api/yggdrasil/api/users/profiles/minecraft/{name}` 返回的 `id`
   与 `sessionserver/session/minecraft/profile/{uuid}` 返回——三处逐字节一致；
5. `POST /api/v1/login` 用老站原密码登录成功（惰性重哈希生效，
   `password_algo` 已变 `argon2id`）。

## 四、已知良性告警

- `WARN invitation id=N used_at is NULL/zero date`：未使用邀请码的正常形态
  （老站 `used_at` 可空/零日期），保留原值，不影响使用。
- `options` 比老站多 1 行是正常的：迁移后应用写入 `setup_done=1`
  （否则空库初始化向导会误触发；引擎不写该键，属预期后置步骤）。

## 五、回滚

目标库 `mysqldump` 快照 + 纹理目录 tar 包直接恢复即可；老站数据全程未写，
无需回滚。跨版本重迁：清库后重跑（引擎幂等）。

## 六、切流到正式域名

迁移完成≠切流。域名切换（如 `test-skin.example.com` → `skin.example.com`）：

1. **签名钥对齐（必做，最先做）**：老站 `ygg_private_key` option（PEM 文本）
   写入目标 `data/sign-key.pem`（`SKIN_SIGN_KEY` 指向）后再切域名——否则
   authlib-injector 客户端缓存的 `signaturePublickey` 与新钥失配、全网皮肤
   签名校验失败。union hub 键连续性由 options 导入自动保持，无需手工。
   - **union 模式例外（2026-10-25 线上实测）**：`union_enabled=1` 时对外
     `signaturePublickey` 来自 union hub 私钥，与老站 yggdrasil-connect
     插件所用密钥同指纹（实测 `6000acb241f046c1…` 双侧一致），切流无需改
     `sign-key.pem`；一旦关闭 union 回退本地签名，必须先把老站
     `ygg_private_key` 写入 `sign-key.pem`（老站 option 值可能存成单行、
     换行转义为字面 `\n`，写入前需解开转义）。
2. 改 `SKIN_BASE_URL` 与 nginx `server_name`、证书；
3. 管理面板改 `site_url`（options 优先于 env）；
4. yggdrasil 客户端的服务器地址需随域名更新（老玩家改一次启动器配置）；
5. 联盟（Union）成员站注册信息若含域名，需在 hub 侧同步。

回滚触发（切流观察期）：登录失败率 >1% / UUID 不一致 >0 / 验签失败 >0 →
立即切回旧站（旧站只读保留至少 30 天）。
