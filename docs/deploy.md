# SkinNexus 部署（N100 / 宝塔）与切流预案

> 目标环境：N100（Debian）+ 宝塔面板；MySQL 8 已有；反代域名 skin.wzbcmc.cn。
> 回滚原则：旧站只读保全量备份，任何 P0 触发即切回（docs/phase1-dev-plan.md §5）。

## 一、构建

```bash
# 后端（在仓库根）
cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o skin-nexus ./cmd/skin-nexus

# 前端
cd web && npm ci && npm run build   # 产物 web/dist
```

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
Environment=SKIN_SKIN_DOMAINS=skin.wzbcmc.cn,（其余 54 个域名，逗号分隔，见蓝图 §1 实测清单）
Environment=SKIN_TEXTURES_DIR=/opt/skin-nexus/data/textures
Environment=SKIN_SIGN_KEY=/opt/skin-nexus/data/sign-key.pem
Environment=SKIN_LEGACY_ENV=/opt/skin-nexus/legacy.env
Environment=SKIN_ADMIN_TOKEN=（随机 64 位 hex，openssl rand -hex 32）
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

## 四、数据迁移执行序（见 phase1-dev-plan §5）

1. `preflight`：读旧 `.env` → 哈希分布抽样（migrate.ClassifyHashes）→ 确认情景甲/乙
2. 全量备份（mysqldump + 纹理目录 tar）
3. 执行数据导入（SQL 脚本，迁移工具 CLI 待补全）
4. re-hash + IHDR 扫描报告（migrate.ScanTextureTree）
5. 抽验：原密码登录、UUID 逐位比对、hasJoined 验签
6. 双跑观察 → 切流（改反代指向新站端口）
7. 观察期后旧站下线（只读保留 30 天）

**回滚触发**：登录失败率 >1% / UUID 不一致 >0 / 验签失败 >0 → 立即切回旧站。

## 五、运维要点

- **密钥备份**：`sign-key.pem` 是纹理签名私钥，丢失=全网皮肤失效，纳入备份清单
- **弱哈希看板**：`GET /api/admin/v1/overview` 的 `weakHashUsers` 归零前保持关注
- **备份**：每日 mysqldump + 纹理目录增量；sign-key.pem 单独加密备份
- 日志：`journalctl -u skin-nexus -f`
