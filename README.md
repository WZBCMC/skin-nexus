# SkinNexus / 肤枢

> 下一代 Minecraft 皮肤站：Yggdrasil 外置登录兼容、一键迁移 blessing-skin-server、自托管优先。

**状态**（每轮开发收尾同步 · 2026-10-23）：一期开发完成（M0–M5）· 二期核心完成（免密登录 + Union 可开关模块）· 轻量池（前端/管理/集成）进行中 · 迁移切流窗口待排期
**许可**：AGPL-3.0 · **发起人/著作权人**：YMZS

## 这是什么

SkinNexus（肤枢）是对年久失修的 [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) 的现代化重写：皮肤、身份、联合认证三线汇聚的中枢。

- **Yggdrasil API 完整兼容**：authlib-injector 直连，既有 MC 客户端与服务器**零改配置**，玩家 UUID 零漂移
- **启动器免密登录（yggc/OIDC 内置）**：设备码流 + refresh 轮换，OAuth token 直通进服（SJMCL / WZBC-Launcher 兼容），取代旧架构 Janus 外挂
- **MUA Union 成员站模块**：**可开关（默认关）**，未接入联盟的部署零负担；三头验签、绑定/同步、OAuth2 grant、共享纹理签名密钥
- **一键数据迁移**：用户/皮肤/披风/角色绑定平滑迁移（多算法密码兼容 + 惰性重哈希）
- **现代技术栈**：Go + Vue 3，单二进制部署
- **对外数据接口**：CustomSkinAPI / USM / legacy 裸 PNG / 公开只读 API
- **自托管与数据主权**：零账号依赖、本地直连、单一控制点
- **插件扩展**：hook 合约已冻结，宿主运行时后置

## 路线图（实时）

| 阶段 | 内容 | 状态 |
|---|---|---|
| 一期 M0–M5 | 技术验证 / 骨架 / Yggdrasil 全量 / 迁移工具 / 前端管理 / 部署切流 | ✅ 开发完成（2026-10-22）；迁移切流窗口待排期 |
| 二期 S0 | 契约侦察（Janus 快照、SJMCL/WZBC-Launcher 客户端分析）与选型 | ✅ 完成（选型 zitadel/oidc v3.51） |
| 二期 H1 | 免密登录核心：OIDC provider、token 双态、契约测试 | ✅ 完成 |
| 二期 H2 | Union 可开关模块：验签 / 入站出站 / OAuth2 grant / UnionSigner | ✅ 完成 |
| 二期 L1–L4 | 免密前端、Union 面板、管理集成、回归与验收辅助 | ⏳ 进行中 |
| 真机验收 | SJMCL + WZBC-Launcher OAuth 免密登录→选角→进服 | ⬜ 待 L 池完成 |
| 正版绑定 / 插件宿主 | Auth Source 第二实现 / goja 宿主运行时 | ⬜ 后置规划 |

## 文档

- [`docs/blueprint.md`](docs/blueprint.md) —— 设计蓝图：目标、架构、迁移方案、技术路线对比、开源协议（设计决策记录，实时状态以本文为准）
- [`docs/standards.md`](docs/standards.md) —— 协作开发规范与 Agent 开发指南
- [`docs/deploy.md`](docs/deploy.md) —— 部署、环境变量与切流预案
- [`AGENTS.md`](AGENTS.md) —— 仓库协作说明、结构与铁律

## 作者与署名

本项目由 **YMZS（[GitHub @YimingZhanshen](https://github.com/YimingZhanshen)）** 发起并享有原始著作权。治理结构见 [`GOVERNANCE.md`](GOVERNANCE.md)，引用格式见 [`CITATION.cff`](CITATION.cff)。

## 许可

AGPL-3.0（见 [`LICENSE`](LICENSE)）。版权行为：`Copyright (c) 2026 YMZS (YimingZhanshen) and SkinNexus contributors`。
