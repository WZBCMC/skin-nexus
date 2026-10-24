# SkinNexus 部署（腾讯云 / 宝塔）与切流预案

> 目标环境：腾讯云 2C/3.5G（150.158.12.29，Debian + 宝塔，2026-10-22 确认；早期 N100 假设作废）；MySQL 8 已有；反代域名 skin.wzbcmc.cn。
> 回滚原则：旧站只读保全量备份，任何 P0 触发即切回（docs/phase1-dev-plan.md §5）。

## 一、构建

```bash
# 后端（在仓库根）
cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o skin-nexus ./cmd/skin-nexus

# 前端
cd web && npm ci && npm run build   # 产物 web/dist
```

## 一之二、Docker Compose 部署（三期）

```bash
# 主线：CI 每轮 push 自动构建生产镜像推 GHCR（:latest + :sha-*），部署机只拉不编：
#   compose 里 image: ghcr.io/yimingzhanshen/skin-nexus:latest
docker compose pull && docker compose up -d      # 升级同命令，数据卷不动
# 也支持源码构建：docker compose up -d --build（多阶段 Dockerfile，CI 每轮实证）
# 旧 blessing 库 / 迁移后库 / 全新空库均可直接接：空库自动建表（SKIN_AUTO_MIGRATE=true）
# 全新部署：首次打开站点自动进入 /setup 安装向导（建管理员/站点名/皮肤域）
```

- **外部 MySQL 8 / 5.7 均可**（5.7.44 实测通过）；容器推荐 `network_mode: host` 直连面板库 `127.0.0.1:3306`。

- **持久化**：`skin-data` 卷挂 `/data`（textures/sign-key.pem/oidc-key.pem/themes/），`skin-db` 卷存 MySQL——**升级只换容器，卷不动，数据零丢失**。
- **统一数据根**：`SKIN_DATA_DIR`（默认 `./data`）；`SKIN_TEXTURES_DIR`/`SKIN_SIGN_KEY` 等旧变量显式设置时优先。
- **新环境变量**：`SKIN_DATA_DIR`、`SKIN_THEMES_DIR`（默认 `<data>/themes`）、`SKIN_AUTO_MIGRATE`（**追踪式迁移**：`schema_migrations` 逐文件记标，空库全量、既有库自动补新文件；重复对象冲突容忍、其它错误启动失败）、`SKIN_MIGRATIONS_DIR`（默认 `./migrations`）、`SKIN_REQUIRE_VERIFICATION`（join 邮箱验证门，`true` 开启；默认 `false`，待 P1 验证邮件流落地后再开）、`SKIN_INVITE_REQUIRED`（注册强制邀请码，`true` 时=线上必填语义；默认 `false` 注册开放、邀请码选填）。**站点类默认值已迁管理 UI**（「管理 → 站点设置」白名单：site_name/server_name/skin_domains/invite_required/require_verification + ygg 五键 + **Union 八键**（2026-10-24），`options` 表值优先于 env；union_enabled/union_api_root/union_member_key 热切换立即生效、其余重启生效）；凭据类（DSN/admin token）仍只走环境变量，**union 凭据（member_key/OAuth2 密钥对）已 options 化**（blessing 原名同构、迁移零搬运，密钥文件/env 仅作迁移桥与兜底）。
- **健康检查**：`GET /healthz`（DB 探活，compose HEALTHCHECK 已用）。
- **主题包**：放入 `data/themes/<id>/`（含 theme.json，规范见 docs/theme-spec.md），admin 面板「外观」选择启用；自定义 CSS/JS 亦在「外观」区块（JS 原样注入，仅用可信代码）。
- **SEO**：admin 面板「SEO」区块一键配置 meta/OG/sitemap/robots；预发可勾整站 noindex。

## 二、首次部署

```bash
# 1. 上传产物
scp server/skin-nexus root@server:/opt/skin-nexus/
scp -r web/dist root@server:/opt/skin-nexus/web/

# 2. 配置（systemd 单元示例 /etc/systemd/system/skin-nexus.service）
```

```ini
[Unit]
Description=SkinNexus
After=network.target mysql.service

[Service]
WorkingDirectory=/opt/skin-nexus
Environment=SKIN_LISTEN=127.0.0.1:8080
Environment=SKIN_BASE_URL=https://skin.wzbcmc.cn
Environment=SKIN_DATABASE_DSN=skin:PASS@tcp(127.0.0.1:3306)/skin-nexus?parseTime=true&charset=utf8mb4
Environment=SKIN_SERVER_NAME=温州商学院皮肤站
# skinDomains 实测版（2026-10-23 抓旧站 /api/yggdrasil/ 共 53 域；与联盟清单逐字一致，改动会破坏联盟互认）
Environment=SKIN_SKIN_DOMAINS=skin.yitmc.cn,skin.cauc.fun,skin.cduestc.fun,skin.cqumc.cn,csu-mc.magicalsheep.cn,mcweb.unsafe.top,skin.ecnumc.cn,mcskin.ecustvr.top,auth.fdcraft.fishp.xyz,skin.ghm.mualliance.cn,skin.gxucraft.cn,skin.gzszmc.cn,bobbyskin.com,skin.hljumc.cn,skin.starkettle.com,skin.metaconstr.net,skin.imucraft.cn,skin.jsumc.fun,mc.jxufe.edu.cn,mcskin.mevisual.top,skin.mualliance.ltd,skin.twinklestars.top,club.barrierslink.cn,skin.nmo.net.cn,skin.nnumc.com.cn,skin.npucraft.com,skin.nudtmc.cn,skin.nufe.wiki,skin.nuistcraft.com,skin.nwafu.com.cn,skin.chancelethay.top,skin.qutcraft.cn,skin.sducraft.cn,skin.sdustmc.cn,skin.sitmc.club,skin.mc.sjtu.cn,skin.nekoplay.fun,user.suesmc.ltd,union-test.gpa.ac.cn,skin.tongji-mc.cn,hall.mc.taru.xj.cn,skin.mc.taru.xj.cn,www.ustb.world,skin.wzbcmc.cn,skin.xauatcraft.com,www.xducraft.cn,skin.xjtumc.com,skin.xmc.net.cn,cop.imu.nm.cn,skin.ybumua.homes,skin.uemcraft.cn,skin.ysumc.net,skin.zjutmc.top
Environment=SKIN_TEXTURES_DIR=/opt/skin-nexus/data/textures
Environment=SKIN_SIGN_KEY=/opt/skin-nexus/data/sign-key.pem
Environment=SKIN_LEGACY_ENV=/opt/skin-nexus/legacy.env
Environment=SKIN_ADMIN_TOKEN=（随机 64 位 hex，openssl rand -hex 32；机器/API 回退——管理面板首选管理员账号（users.permission>0）登录直用，无需 token）
# —— 二期：yggc/OIDC 免密登录（不启用可整段移除）——
Environment=SKIN_OIDC_ENABLED=true
Environment=SKIN_OIDC_ISSUER=https://skin.wzbcmc.cn
Environment=SKIN_OIDC_SIGN_KEY=/opt/skin-nexus/data/oidc-key.pem
Environment=SKIN_OIDC_SHARED_CLIENT_ID=2
# —— 二期：MUA Union 成员站模块（默认关；未接入联盟保持 false = 零挂载零表面）——
# Environment=SKIN_UNION_ENABLED=true
# Environment=SKIN_UNION_API_ROOT=https://skin.mualliance.ltd/api/union
# Environment=SKIN_UNION_MEMBER_KEY=（Union 下发的成员密钥，轮换后需持久化）
# Environment=SKIN_UNION_HOST_PUBKEY=/opt/skin-nexus/data/union-host-pubkey.pem
# Environment=SKIN_UNION_OAUTH2_SIG_KEY=/opt/skin-nexus/data/union-oauth2-sig.pem
# Environment=SKIN_UNION_QUERYEMAIL_MODE=full
# Environment=SKIN_UNION_ALLOW_UPDATEBACKENDKEY=false
ExecStart=/opt/skin-nexus/skin-nexus
Restart=always
User=www

[Install]
WantedBy=multi-user.target
```

```bash
# 3. 建库 + 迁移表（宝塔 phpMyAdmin 或 mysql 客户端逐个执行）
mysql -u skin -p skin-nexus < migrations/0001_init.sql
mysql -u skin -p skin-nexus < migrations/0002_web_sessions.sql
mysql -u skin -p skin-nexus < migrations/0003_oidc.sql
mysql -u skin -p skin-nexus < migrations/0004_union.sql

# 4. legacy.env（旧站 .env 的拷贝，只留 PASSWORD_METHODS/SALT/TEXTURES_DIR）
#    → 密码兼容链按序试算 + 首登惰性重哈希

# 5. 启动
systemctl enable --now skin-nexus
curl -s https://skin.wzbcmc.cn/api/yggdrasil/ | head -c 300   # 验元数据
```

## 三、Nginx 反代（宝塔站点配置）

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
# /textures/ 大流量可直接 alias 静态目录绕过后端：
# location /textures/ { alias /opt/skin-nexus/data/textures/; add_header Cache-Control "public, max-age=31536000, immutable"; }
```

> `X-Forwarded-Proto` 必传：ALI 响应头的协议判断依赖它。
>
> `X-Forwarded-For` 同样必传（2026-10-24 B4 起）：注册按 IP 计数（`regs_per_ip`）与
> `users.ip` 记录、认证限流窗口都走 `ygg.ClientIP`。缺了它，容器看到的对端全是
> 反代地址——**全站共享一个计数桶，第三个注册就把所有人挡在门外**，且所有账号的
> `ip` 列都会记成同一个值（切流后追溯困难）。服务端只在对端处于
> loopback/RFC1918 信任区时才采信 XFF 的**最后一跳**（防伪造绕限流），所以
> 反代必须用 `$proxy_add_x_forwarded_for` 或 `$remote_addr` 追加真实地址，
> 不要改成透传客户端原样头。

## 四、数据迁移执行序（见 phase1-dev-plan §5）

1. `preflight`：`./skin-nexus migrate preflight --env legacy.env --textures <TEXTURES_DIR> --dump <哈希抽样文件>` → 方法/盐对齐表（盐打码）、哈希分布、情景甲/乙判定、纹理树审计
2. 全量备份（mysqldump + 纹理目录 tar）
3. 执行数据导入（SQL 脚本）
4. **签名钥对齐**：老站 `ygg_private_key` option（PEM 文本）写入 `data/sign-key.pem` 后再启新站——否则 authlib-injector 侧缓存的 `signaturePublickey` 失配、全网皮肤签名校验失败（union hub 键连续性同理由 options 导入自动保持，无需手工）
5. uuid 清洗 dry-run：`./skin-nexus migrate uuid --rows uuid.tsv --players players.tsv`（五步报告，apply 走导入脚本）
6. re-hash + IHDR 扫描报告（preflight --textures 已含）
7. 抽验：原密码登录、UUID 逐位比对、hasJoined 验签
8. 双跑观察 → 切流（改反代指向新站端口）
9. 观察期后旧站下线（只读保留 30 天）

**回滚触发**：登录失败率 >1% / UUID 不一致 >0 / 验签失败 >0 → 立即切回旧站。

## 五、运维要点

- **密钥备份**：`sign-key.pem` 是纹理签名私钥，丢失=全网皮肤失效，纳入备份清单
- **OIDC 签名钥**：`oidc-key.pem` 与纹理签名钥**分离**（免密登录 token 专用），首启自动生成，同样纳入备份；两钥互不代替
- **Union 开关**：`union_enabled` 热切换（管理面板「站点设置」；env `SKIN_UNION_ENABLED` 仅兜底）——关=全 `/api/union|v1/union|admin/union` 路由 404、纹理签名回本地密钥；开=即时恢复。Union 配置八键全在面板（blessing 原名：union_api_root/union_member_key/union_enable_update/union_enable_oauth2/union_oauth2_sig_* 等），**hub 验签公钥每次现取带缓存**（pinned 文件仅兜底），OAuth2 密钥对留空首启自动生成入库。开启前须具备成员密钥；管理面板展示**信任声明**（Union 模式下纹理签名私钥为全网共享）
- **弱哈希看板**：`GET /api/admin/v1/overview` 的 `weakHashUsers` 归零前保持关注
- **备份**：每日 mysqldump + 纹理目录增量；sign-key.pem 单独加密备份
- 日志：`journalctl -u skin-nexus -f`
