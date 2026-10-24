# 从 blessing-skin-server 迁移（migrate 子命令手册）

> 目标读者：自行部署 SkinNexus 并要把既有 blessing-skin-server（老皮肤站）数据
> 全量迁过来的运营者。全流程实测于 2026-10-24（生产库 67 用户 / 67 角色 /
> 61 纹理 / 394 邀请码，零丢行、UUID 零漂移）。
>
> 部署新站本体见 [`deploy.md`](deploy.md)；构建见 [`build.md`](build.md)。

## 〇、开始前 checklist

- [ ] SkinNexus 已能启动并连上**空的目标库**（`SKIN_AUTO_MIGRATE=true` 自动建表，或手动执行 `migrations/0001…0009`）
- [ ] 手上有老站 `.env`（至少 `APP_URL`、`PWD_METHOD`/`PASSWORD_METHODS`、`SALT`）与老站纹理目录 `storage/textures/` 的读权限
- [ ] 老站数据库只读账号（或 mysqldump 快照）
- [ ] 目标机磁盘余量 ≥ 纹理目录体积 × 2（备份 + 拷贝各一份）
- [ ] 目标纹理目录（`SKIN_TEXTURES_DIR`，compose 默认 `/data/textures`）可写
- [ ] 已读完本文 §三「切流断言」与 §六「切流到正式域名」（含签名钥步骤）

**原则**：老站只读、目标先备份、先 dry-run 后实跑、跑完对账。

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

> 老站是**魔改/分叉版** blessing 也适用：import 按表名+列名搬运，缺列的表
> 引擎会明确报错而非静默丢数据；先跑 §二第 0 步 preflight 与 dry-run 即可暴露。

## 二、执行序（实测路径）

### 0. preflight（只读体检）

解析老站 .env 的密码算式/盐槽，审计纹理树：

```bash
skin-nexus migrate preflight --env /path/to/legacy/.env \
  --textures /path/to/legacy/storage/textures \
  --dump hashes.txt
#   预期：SALT slots aligned OK；texture scan: ok=<N> hashMismatch=0 badIHDR=0
#   hashes.txt = 老站 users.password 列逐行导出（mysql -N -e 'SELECT password FROM users' > hashes.txt）
```

读三处输出做决策：

1. **方法/盐对齐表**（盐打码显示）——与老站 `.env` 声明一致才继续；
2. **哈希分布**：`weak-share=0%` → 情景甲（仅惰性重哈希）；`weak-share>0%` → 情景乙（MD5/SALTED 族存量，仍走惰性重哈希 + 管理面板「弱哈希余量」看板跟踪）；
3. **纹理树审计**：`hashMismatch`（文件名≠内容 hash）与 `badIHDR`（解码异常）逐个列出——本策略为**原样搬迁、report-only**，问题文件记录在案但不阻断，新上传才走重编码管线。

### 1. 备份

```bash
# 目标库与纹理目录（旧站侧不需要备份——全程只读）
mysqldump -u root -p skin_nexus | gzip > backup-skin_nexus-$(date +%Y%m%d-%H%M%S).sql.gz
tar czf backup-textures-$(date +%Y%m%d-%H%M%S).tgz /data/textures
```

### 2. 停目标站，清空业务表（重迁场景）

保留 `union_kv`、`oidc_clients`、`schema_migrations` 这些站点身份/基建表。
注意 **MySQL 5.7 的 `TRUNCATE` 是单表语句**，多表一次写会报错，逐表执行：

```bash
for t in users players textures uuid_map options user_closet invitation_codes; do
  mysql -u root -p skin_nexus -e "TRUNCATE TABLE $t;"
done
```

全新部署跳过本步。

### 3. dry-run（只读预演）

```bash
skin-nexus migrate import \
  --source-dsn 'USER:PASS@tcp(127.0.0.1:3306)/legacy?parseTime=true' \
  --target-dsn 'USER:PASS@tcp(127.0.0.1:3306)/skin_nexus?parseTime=true' \
  --source-prefix 'legacy.' \
  --textures-src /path/to/legacy/storage/textures --textures-dst /data/textures \
  --dry-run
```

通过标准：各表 `selected` 数与老站 `COUNT(*)` 逐一相等、`skipped==0`、零非良性 WARN。

### 4. 实跑

去掉 `--dry-run` 即可；引擎**幂等**（INSERT IGNORE / NOT EXISTS），失败可直接重跑。

参数要点：

- **两侧 DSN 必须带 `parseTime=true` 且 `loc=` 一致**（引擎强制校验，不一致直接拒绝）。
- `--source-prefix` 是老站表前缀/库名限定（如 `legacy.` 表示同实例的 legacy 库，或 `legacy_` 表前缀），留空则同库。
- `--textures-src` 给老站纹理目录；`--textures-dst` 给目标 `SKIN_TEXTURES_DIR`
  （compose 默认 `/data/textures`），不给则只迁库不迁文件。
- 导入会保留主键 ID：目标表必须为空（或与老站同 ID 数据），否则撞主键会 skip。

## 三、切流断言（跑完必须核对）

1. **`users selected==inserted` 且 `skipped==0`**——skip>0 说明目标库有残留行
   主键撞车（引擎 INSERT IGNORE 会保旧弃新），此时清库重跑；
2. 各表 `inserted` 与老站行数逐一相等（本项目实测 67/67/61/67/87/84/394）；
3. 纹理 `files copied=N` 且抽样 `cmp` 老站/目标同名文件逐字节一致：

   ```bash
   cmp /老站/storage/textures/<hash> /目标/data/textures/<hash> && echo MATCH
   ```

4. **UUID 零漂移抽查**：取老站 `uuid` 表一行，对照两处——

   ```bash
   curl -s https://你的域名/api/yggdrasil/api/users/profiles/minecraft/{name}
   curl -s https://你的域名/api/yggdrasil/sessionserver/session/minecraft/profile/{uuid}
   ```

   老站 `uuid` 表 = 名查返回的 `id` = profile 查询路径，三处逐字节一致；

5. `POST /api/v1/login` 用老站原密码登录成功（惰性重哈希生效，
   该用户 `password_algo` 已变 `argon2id`，管理面板用户表可见）。

## 四、已知良性告警

- `WARN invitation id=N used_at is NULL/zero date`：未使用邀请码的正常形态
  （老站 `used_at` 可空/零日期），保留原值，不影响使用。
- `options` 比老站多 1 行是正常的：迁移后应用写入 `setup_done=1`
  （否则空库初始化向导会误触发；引擎不写该键，属预期后置步骤）。

## 五、回滚

目标库 `mysqldump` 快照 + 纹理目录 tar 包直接恢复即可；老站数据全程未写，
无需回滚。跨版本重迁：清库后重跑（引擎幂等）。

切流观察期回滚触发条件：**登录失败率 >1% / UUID 不一致 >0 / 验签失败 >0**
→ 立即把反代切回旧站（旧站只读保留至少 30 天）。

## 六、切流到正式域名

迁移完成≠切流。域名切换（如 `test-skin.example.com` → `skin.example.com`，本项目
2026-10-25 实测完成）：

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
   - 从老站导出该 option 的参考命令（只取公钥指纹比对，不打印私钥）：

     ```bash
     # 老站库导出（值为单行 PEM，\n 是字面转义）
     mysql -N -e "SELECT value FROM legacy.options WHERE k=0x7967675f707269766174655f6b6579" > yggk.txt
     sed 's/\\n/\n/g' yggk.txt > sign-key.pem
     # 双侧公钥指纹比对（应一致）
     openssl pkey -in sign-key.pem -pubout | openssl pkey -pubin -outform DER | sha256sum
     ```

2. 改 `SKIN_BASE_URL` 与 nginx `server_name`、证书；
3. 管理面板改 `site_url`（options 优先于 env）；
4. yggdrasil 客户端的服务器地址需随域名更新（老玩家改一次启动器配置）；
5. 联盟（Union）成员站注册信息若含域名，需在 hub 侧同步。

## 七、迁移后运营检查

- **邀请码**：`invitation_codes` 394 条已随迁；老站「注册开放/必填邀请码」语义由管理面板「站点设置」`invite_required` 控制（默认注册开放、邀请码选填）。
- **皮肤库/衣柜**：用户衣柜 `user_closet` 已迁；皮肤库页点赞数 = `COUNT(user_closet)` 实时统计，零 schema 变更。
- **弱哈希余量**：管理面板仪表盘 `weakHashUsers` 归零前保持关注（用户登录即惰性换 argon2id）。
- **高清皮肤**：原生分辨率保留渲染，预览 URL 带 `?v=` 版本号；若浏览器看到旧缓存图，刷新一次即可。
- **公告/设置**：老站 `options` 已导入，管理面板各设置页直接可见；老站自定义 CSS/JS、主题不随迁（老站为 PHP 模板，语义不兼容），在新站「外观」重新配置。
