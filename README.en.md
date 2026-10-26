# SkinNexus

<p align="center"><img src="docs/assets/logo.svg" width="140" alt="SkinNexus pixel-nexus N logo"></p>

> A next-generation Minecraft skin hosting server: full Yggdrasil API compatibility, one-command migration from blessing-skin-server, self-hosting first.

[中文文档](README.md) | **English README**

**Status** (2026-10-26): Phase 1 (core) ✅ · Phase 2 (yggc/OIDC passwordless login + MUA Union module) ✅ · Phase 3 skeleton (i18n / SEO / themes / Docker / fresh-DB setup wizard) ✅ · Admin panel parity with blessing B1–B5 ✅ · Full production data migration proven (67 users / 61 textures, zero row loss, zero UUID drift) ✅ · 36-endpoint blessing / yggdrasil-connect compatibility matrix fully aligned ✅ · Native-resolution HD skin rendering ✅ · SSR landing + per-page SEO meta ✅ · Site logo (SVG identity + favicon/PWA icons) ✅ · SSR skin detail pages (`/skinlib/{tid}`) + launcher deep links ✅ · A1 password reset (SMTP flow, enumeration-proof, single-use tokens) ✅ · A2 Mojang official fallback (hardened outbound, panel hot-switch) ✅ · C1 ops observability (login + server/union traffic logs, 90-day retention) ✅ · Competitor comparison matrix (source-verified, bilingual) ✅ · Auth forms follow site config (invite required-or-absent; email + reset hidden when SMTP is unconfigured) ✅ · Grouped admin settings page (seven sections; fallback switch as checkbox; SMTP masked) ✅ · Per-round details in [`docs/CHANGELOG.md`](docs/CHANGELOG.md) (Chinese).

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

## Comparison with mainstream open-source skin servers

> Compared against [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) (Laravel/PHP, upstream now maintenance-only), [element-skin](https://github.com/water2004/element-skin) (Go + PostgreSQL + Vue 3), and [vSkin](https://github.com/iJunecn/vSkin) (FastAPI + Vue 3).
> Every cell was re-verified against the opponents' **source code** (route tables / grep, 2026-10-25–26); method and per-item evidence in [`docs/competitor-analysis.md`](docs/competitor-analysis.md) (Chinese). We credit rivals where they lead and never guess a ❌ — errata are corrected in place and logged in the CHANGELOG (e.g. the 2026-10-26 blessing skinlib fix).

| Capability | SkinNexus | blessing | element-skin | vSkin |
|---|---|---|---|---|
| Search-engine visibility (sitemap / robots / per-page meta·OG·JSON-LD) | ✅ complete | ⚠️ per-page title + one site-wide description only (no sitemap/OG/JSON-LD) | ❌ | ❌ |
| Server-rendered content pages (indexable, no JS required) | ✅ landing + skin detail pages | ⚠️ Twig shell — title server-side, body rendered client-side | ❌ SPA shell | ❌ SPA shell |
| Per-skin shareable URLs | ✅ `/skinlib/{tid}` | ✅ `/skinlib/show/{tid}` (corrected 2026-10-26) | ❌ list + modal only | ❌ list only |
| Launcher deep links (PCL-style "change skin / change password") | ✅ `/user/closet`·`/user/profile` | ✅ origin of the convention | ❌ | ❌ |
| Mojang official fallback (local miss → official sessionserver) | ✅ hardened outbound + panel hot-switch | ❌ | ✅ | ✅ |
| Password reset + SMTP mail flow | ✅ enumeration-proof, single-use tokens | ✅ | ✅ | ✅ |
| Built-in OIDC passwordless login (no Janus sidecar) | ✅ built in; all four grants incl. device_code announced | ⚠️ needs Janus + yggdrasil-connect plugin | ⚠️ shipped but discovery under-announces | ⚠️ device flow solid, OAuth details weak |
| MUA Union federation | ✅ built-in opt-in module with outbound sync | ✅ via yggdrasil-connect plugin | ❌ absent from source | ⚠️ config shell only |
| One-command blessing migration (ID-preserving / zero UUID drift) | ✅ preflight → dry-run → import | — (migration source) | ❌ | ❌ |
| Ban enforcement | ✅ login / authenticate / hasJoined | ✅ permission-level ban | ✅ | ❌ hasJoined only |
| Ops observability (login + server/union traffic logs, 90 days + audit) | ✅ full-chain, viewable in panel | ❌ | ⚠️ permission audit only | ❌ |
| Deployment shape | Go single binary / Docker, CI-published images | PHP 8.1 + Laravel 10 (PHP 8.1 is EOL) | Go + PostgreSQL + Redis | FastAPI + SQLite |

**Leads** (per-cell, evidence-graded): the **complete SEO system** — body-level SSR + per-page OG/JSON-LD + sitemap/robots — has no full counterpart among the three (blessing ships skinlib detail pages and per-page titles, but renders bodies client-side and lacks OG/JSON-LD/sitemap/robots; the two newer rivals have none); the **ID-preserving blessing migration chain** and **full-chain ops observability** (login + server/union traffic logs + audit) are absent from all three (element-skin only has a permission audit). On top of that, a measured performance ledger from a **2-core / 3.5 GB production VPS**: ~600 RPS on the landing page (CPU-capped), 5182 RPS texture delivery, 2365 RPS Yggdrasil metadata, zero errors throughout (see [`docs/CHANGELOG.md`](docs/CHANGELOG.md)).

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
