# SkinNexus 部署手册（Docker 优先）

> 推荐所有站长用 **Docker Compose** 部署：升级只换容器、数据卷零丢失、免装运行时，
> 且 GitHub Actions 每次 push main 都会自动构建生产镜像推到 GHCR，部署机**只拉不编**。
> 从老站迁数据见 [migration.md](migration.md)；源码构建细节见 [build.md](build.md)。

## 一、部署方式选择

| 方式 | 适合 | 说明 |
|---|---|---|
| **A. Docker Compose（推荐）** | 绝大多数站长 | 拉镜像 → 改环境变量 → `up -d`；升级 `pull && up -d`；内置 MySQL 可选 |
| B. 源码构建 + systemd | 无 Docker 的面板机/内网机 | 单二进制 + web/dist，见 §五 |

## 二、Docker Compose 部署（主线）

### 2.1 准备 compose 文件

仓库根的 [`docker-compose.yml`](../docker-compose.yml) 即成品，复制到部署目录：

```bash
mkdir -p /opt/skin-nexus && cd /opt/skin-nexus
# 从仓库复制 docker-compose.yml 后编辑环境变量（见 2.2）
```

镜像来源（compose 默认第一行）：

```yaml
image: ghcr.io/yimingzhanshen/skin-nexus:latest   # CI 自动构建，部署机只拉不编
# 需要固定版本时改用 :sha-<commit> 标签（GHCR 同步保留每 commit 镜像，回滚=改回旧 sha 再 up -d）
# 离线/内网环境可源码构建：把 image 行改回 build: . 后 docker compose up -d --build
```

### 2.2 选择数据库模式（同一份文件二选一）

**模式 1：内置 MySQL（默认，全新站点最省心）**

保留 `db` 服务与 `depends_on` 不动，改三个密码即可：

```yaml
db:
  environment:
    MYSQL_DATABASE: skin_nexus
    MYSQL_USER: skin
    MYSQL_PASSWORD: CHANGE_ME          # <- 改掉
    MYSQL_ROOT_PASSWORD: CHANGE_ME_ROOT # <- 改掉
```

**模式 2：外部 MySQL（已有老 blessing 库 / 迁移目标库 / 面板机库）**

注释掉 `db` 服务与 `depends_on`，把 DSN 指向外部库：

```yaml
environment:
  SKIN_DATABASE_DSN: 'skin:PASS@tcp(127.0.0.1:3306)/skin_nexus?parseTime=true&charset=utf8mb4'
# 容器要访问宿主机上的 MySQL（如宝塔 127.0.0.1:3306）时，二选一：
#   network_mode: host        # 整容器走主机网络（此时 ports 段删除；SKIN_LISTEN 建议 127.0.0.1:8080）
#   或 DSN 用宿主机内网 IP / docker 网桥网段（172.17.0.1）
```

外部库 MySQL 5.7 / 8.0 均可（5.7.44 实测通过）；空库启动时 `SKIN_AUTO_MIGRATE=true`
自动建表，随后首访站点进 `/setup` 安装向导（建管理员 / 站点名 / 皮肤域）。

### 2.3 必改环境变量

| 变量 | 说明 |
|---|---|
| `SKIN_BASE_URL` | 站点公开地址，如 `https://skin.example.com`（纹理 URL / ALI 头依赖它，**必须与玩家访问地址一致**） |
| `SKIN_DATABASE_DSN` | 外部库模式必改；内置库模式保持默认 |
| `SKIN_ADMIN_TOKEN` | `openssl rand -hex 32` 生成；机器/API 调用管理接口的回退凭据（管理面板本身用管理员账号登录即可，无需此 token） |

其余可选变量（默认值即可跑通）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `SKIN_LISTEN` | `:8080` | 容器内监听；反代场景建议 `127.0.0.1:8080` + ports 映射 |
| `SKIN_SERVER_NAME` / `SKIN_SITE_NAME` | `SkinNexus` | Yggdrasil 元数据显示名 / 站点名（也可在管理面板改） |
| `SKIN_SKIN_DOMAINS` | 空 | 皮肤域白名单；不接联盟填本站域名，接入 MUA Union 以联盟公布清单为准（改动会破坏联盟互认） |
| `SKIN_DATA_DIR` | `/data`（镜像内） | 统一数据根：textures/、签名钥、themes/ 都在它下面 |
| `SKIN_AUTO_MIGRATE` | `true`（镜像内） | 追踪式建表/补表：`schema_migrations` 逐文件记标，空库全量、既有库自动补新文件 |
| `SKIN_INVITE_REQUIRED` | `false` | 注册强制邀请码（等价老站必填语义）；也可在管理面板「站点设置」改 |
| `SKIN_REQUIRE_VERIFICATION` | `false` | join 邮箱验证门（邮件流落地前保持关） |
| `SKIN_RATE_LIMIT_AUTH` / `SKIN_LOGIN_FAIL_LIMIT` | `120` / `5` | 认证限流：每 IP 10 分钟滑动窗口 / 连续失败锁定次数 |
| `SKIN_OIDC_ENABLED` | `false` | 启动器免密登录（yggc/OIDC）；`SKIN_OIDC_ISSUER` 默认跟 `SKIN_BASE_URL` |
| `SKIN_UNION_ENABLED` | `false` | MUA Union 联盟模块；未接入保持关 = 零路由零表面 |

> **站点类设置大多已迁管理 UI**（「管理 → 站点设置」：站点名/皮肤域/邀请码/验证门/ygg 五键/Union 八键），
> `options` 表值优先于 env；`union_enabled`/`union_api_root`/`union_member_key` 热切换立即生效，其余重启生效。
> 凭据类（DSN、admin token）只走环境变量。

### 2.4 启动与验收

```bash
docker compose up -d
docker compose ps            # 等两个服务 healthy（镜像内置 HEALTHCHECK 探 /healthz，DB 探活）
curl -s http://127.0.0.1:8080/api/yggdrasil/ | head -c 300   # 元数据 JSON
# 浏览器打开 SKIN_BASE_URL → 首访自动进 /setup 安装向导
```

### 2.5 升级

```bash
docker compose pull && docker compose up -d    # 只换容器，卷不动，数据零丢失
# 新版会自动补齐新 DDL（追踪式迁移）；回滚 = image 改回旧 :sha-* 再执行同命令
```

### 2.6 备份与恢复

数据全在两个命名卷（`docker volume ls | grep skin`）：

| 卷 | 内容 | 备份方式 |
|---|---|---|
| `skin-data` | textures/、sign-key.pem、oidc-key.pem、themes/、union 密钥 | 见下 |
| `skin-db` | 内置 MySQL 数据（外部库模式无此卷，用你自己的 mysqldump 方案） | `mysqldump` |

```bash
# skin-data 整卷备份（sign-key.pem 丢失 = 全网皮肤失效，务必纳入定期备份）
docker run --rm -v <项目>_skin-data:/data -v /backup:/backup alpine \
  tar czf /backup/skin-data-$(date +%F).tgz -C /data .

# 内置库逻辑备份
docker compose exec db sh -c 'exec mysqldump -uskin -p"$MYSQL_PASSWORD" skin_nexus' \
  | gzip > /backup/skin_nexus-$(date +%F).sql.gz

# 恢复：停站 → 解包回卷 / 导入 SQL → 起站
```

> `sign-key.pem` 是纹理签名私钥，建议额外**加密后异地**备份一份；
> `oidc-key.pem` 与它分离（免密登录 token 专用，首启自动生成），同样纳入备份，两钥互不代替。

### 2.7 日志与排错

```bash
docker compose logs -f skin-nexus          # 应用日志
docker compose logs --tail 50 db          # 内置库日志
```

| 现象 | 处理 |
|---|---|
| 容器反复 unhealthy | 看日志：DSN 不通 / 密码错最常见；`docker compose exec skin-nexus wget -qO- http://127.0.0.1:8080/healthz` 单独验证 |
| 启动报 migration 失败 | 非空库但缺表/结构漂移时追踪式迁移会报错退出——先备份再看日志定位文件，勿手改 `schema_migrations` |
| 外部库连不上 | 网络模式与 DSN 主机对不上（host 模式配 `127.0.0.1`、桥接模式配 `172.17.0.1` 或内网 IP）；MySQL 账号需有建表权限（自动迁移用） |
| 页面是占位页 | `web/dist` 缺失——确认拉的是 GHCR 镜像而非残缺自构建 |

## 三、Nginx 反代（前置 HTTPS 证书）

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
# /textures/ 大流量可让 nginx 直读数据卷目录绕过后端（可选）：
# location /textures/ {
#     alias /var/lib/docker/volumes/<项目>_skin-data/_data/textures/;
#     add_header Cache-Control "public, max-age=31536000, immutable";
# }
```

> `X-Forwarded-Proto` 必传：ALI 响应头的协议判断依赖它。
>
> `X-Forwarded-For` 同样必传：注册按 IP 计数（`regs_per_ip`）与 `users.ip` 记录、
> 认证限流窗口都走 `ygg.ClientIP`。缺了它，容器看到的对端全是反代地址——
> **全站共享一个计数桶，第三个注册就把所有人挡在门外**，且所有账号的 `ip` 列
> 都会记成同一个值（切流后追溯困难）。服务端只在对端处于 loopback/RFC1918
> 信任区时才采信 XFF 的**最后一跳**（防伪造绕限流），所以反代必须用
> `$proxy_add_x_forwarded_for` 或 `$remote_addr` 追加真实地址，不要透传客户端原样头。

## 四、从老站迁移

独立手册见 [migration.md](migration.md)（preflight → dry-run → import 三步 +
切流断言 + 签名钥对齐）。Docker 场景两个入口差异：

```bash
# 迁移 CLI 直接用镜像跑（不需要在服务器上装 Go）：
docker compose exec skin-nexus /app/skin-nexus migrate preflight --env /data/legacy.env --textures /data/textures
# legacy.env 放进数据卷，或 -v ./legacy.env:/data/legacy.env:ro 临时挂载
```

## 五、备选：源码构建 + systemd（无 Docker 环境）

构建步骤见 [build.md](build.md) §二/§三；产物为单二进制 + `web/dist/` + `migrations/`。
systemd 单元要点（完整示例见 git 历史或按此模式编写）：

```ini
[Service]
WorkingDirectory=/opt/skin-nexus
Environment=SKIN_LISTEN=127.0.0.1:8080
Environment=SKIN_BASE_URL=https://skin.example.com
Environment=SKIN_DATABASE_DSN=skin:PASS@tcp(127.0.0.1:3306)/skin_nexus?parseTime=true&charset=utf8mb4
Environment=SKIN_DATA_DIR=/opt/skin-nexus/data
Environment=SKIN_MIGRATIONS_DIR=/opt/skin-nexus/migrations
Environment=SKIN_ADMIN_TOKEN=<openssl rand -hex 32>
ExecStart=/opt/skin-nexus/skin-nexus
Restart=always
```

要点：工作目录须能读到 `web/dist` 与 `migrations/`（或用 `SKIN_MIGRATIONS_DIR` 指定）；
手动建表按序执行 `migrations/0001…0009`，或设 `SKIN_AUTO_MIGRATE=true` 让启动时自动应用。
Nginx 反代与 §三完全相同。日志 `journalctl -u skin-nexus -f`。

## 六、运维周期检查

- **弱哈希看板**：管理面板仪表盘 `weakHashUsers` 归零前保持关注（用户登录即惰性换 argon2id）
- **Union 开关**：管理面板「站点设置」热切换——关 = 全 union 路由 404 + 纹理签名回本地密钥；开 = 即时恢复。开启前须已有成员密钥；面板展示信任声明（Union 模式下纹理签名私钥全网共享）
- **主题/外观**：主题包放 `skin-data` 卷的 `themes/`（规范见 [theme-spec.md](theme-spec.md)）；自定义 CSS/JS 与 SEO 在管理面板对应区块
- **定期**：备份任务跑 §2.6 两条命令；证书到期前续期；`docker compose pull` 后看 CHANGELOG 再升级
