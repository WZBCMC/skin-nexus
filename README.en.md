# SkinNexus

<p align="center"><img src="docs/assets/logo.svg" width="140" alt="SkinNexus pixel-nexus N logo"></p>

> A next-generation Minecraft skin hosting server: full Yggdrasil API compatibility, one-command migration from blessing-skin-server, self-hosting first.

[中文文档](README.md) | **English README**

**Status** (2026-10-25): Phase 1 (core) ✅ · Phase 2 (yggc/OIDC passwordless login + MUA Union module) ✅ · Phase 3 skeleton (i18n / SEO / themes / Docker / fresh-DB setup wizard) ✅ · Admin panel parity with blessing B1–B5 ✅ · Full production data migration proven (67 users / 61 textures, zero row loss, zero UUID drift) ✅ · Native-resolution HD skin rendering ✅ · Site logo (SVG identity + favicon/PWA icons) ✅ · Per-round details in [`docs/CHANGELOG.md`](docs/CHANGELOG.md) (Chinese).

**License**: AGPL-3.0 · **Author / Copyright holder**: YMZS

## What is this?

SkinNexus is a modern rewrite of the unmaintained [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) (Laravel/PHP), in **Go + Vue 3**. It is the hub where skins, identities, and federated authentication converge.

- **Full Yggdrasil API compatibility** — authlib-injector works out of the box; existing MC clients and servers need **zero config changes**; player UUIDs never drift
- **Launcher passwordless login (yggc/OIDC built in)** — device-code flow + refresh-token rotation; OAuth tokens pass straight into servers (SJMCL and other community launchers)
- **MUA Union member module** — **opt-in, off by default**; deployments outside the union carry zero surface: no routes, no overhead
- **One-command data migration** from blessing-skin-server — users/skins/capes/role bindings migrate intact (multi-algorithm password compatibility + lazy rehash)
- **Docker Compose deployment** — CI builds and publishes production images to GHCR on every push to `main`; deployment hosts only pull
- **Fresh-DB bootstrap** — empty database auto-initializes schema, then a `/setup` wizard creates the admin account and site settings
- **i18n** — zh-CN / en built in, switchable at runtime
- **SEO** — meta/OG/canonical injection, `sitemap.xml`, `robots.txt`, optional site-wide `noindex`
- **Custom CSS/JS & theme packs** — blessing-style admin styling plus `data/themes/` theme packs ([theme spec](docs/theme-spec.md), AI-generatable)
- **Texture compatibility APIs** — CustomSkinAPI / USM / legacy bare-PNG / public read-only API
- **Server-side avatar & preview rendering** — `/avatar/...` and `/preview/...` output PNG; HD skins keep their native resolution
- **Self-hosting & data sovereignty** — zero third-party accounts, local data, single control point
- **Plugin extension (planned)** — hook contract frozen (goja runtime chosen); host runtime ships later

## Quick start (for operators)

Path A — **Docker Compose (recommended)**:

```bash
git clone https://github.com/YimingZhanshen/skin-nexus.git && cd skin-nexus
# edit the environment variables in docker-compose.yml (database password, SKIN_BASE_URL, ...)
docker compose up -d                  # first install; upgrades: docker compose pull && up -d
# open your site — the /setup wizard runs on first visit (admin account / site name / skin domains)
```

Full guide (two database modes, complete env-var tables, backup/restore, troubleshooting): [`docs/deploy.md`](docs/deploy.md).

Path B — source build + systemd (no-Docker hosts): [`docs/build.md`](docs/build.md) §2/§3.

Path C — migrate an existing blessing-skin-server: [`docs/migration.md`](docs/migration.md) — `preflight → dry-run → import`, fully rollback-able.

**After launch (mandatory)**: your reverse proxy must forward `X-Forwarded-Proto` and `X-Forwarded-For` (without them, the ALI protocol header is wrong and all users share one rate-limit bucket); back up `data/sign-key.pem` (losing it invalidates every skin signature).

## Documentation

Most detailed documentation is currently in Chinese:

- [`docs/build.md`](docs/build.md) — build guide: requirements, source build, Docker image, CI artifacts
- [`docs/deploy.md`](docs/deploy.md) — deployment, env vars, Nginx reverse proxy, operations
- [`docs/migration.md`](docs/migration.md) — migration from blessing-skin-server + cutover assertions + signing-key alignment
- [`docs/compat-matrix.md`](docs/compat-matrix.md) — blessing / yggdrasil-connect compatibility matrix (measured baseline)
- [`docs/theme-spec.md`](docs/theme-spec.md) — theme pack spec
- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) — per-round change log
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contribution guide: translations, theme packs, code & docs
- [`AGENTS.md`](AGENTS.md) — repo map and the seven compatibility iron rules (start here for AI agents)

## Compatibility contracts (must not break)

1. `/api/yggdrasil/**` semantics are frozen (204 lists, `{error,errorMessage}` bodies, millisecond timestamps, strict PEM format)
2. Player UUIDs never drift (`OfflinePlayer:` md5 variant; renames do not recompute)
3. Persistent data stays structurally compatible with blessing-skin-server storage conventions
4. `internal/union` wire contract and `internal/plugin` hook contract are frozen

## Author & attribution

Created and copyrighted by **YMZS ([@YimingZhanshen](https://github.com/YimingZhanshen))**. Governance: [`GOVERNANCE.md`](GOVERNANCE.md). Citation: [`CITATION.cff`](CITATION.cff).

## License

AGPL-3.0 (see [`LICENSE`](LICENSE)). Copyright (c) 2026 YMZS (YimingZhanshen) and SkinNexus contributors.
