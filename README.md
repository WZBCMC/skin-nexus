# SkinNexus / 肤枢

> 下一代 Minecraft 皮肤站：Yggdrasil 外置登录兼容、一键迁移 blessing-skin-server、自托管优先。

**状态**：一期开发中（M0 技术验证阶段） · **许可**：AGPL-3.0 · **发起人/著作权人**：YMZS

## 这是什么

SkinNexus（肤枢）是对年久失修的 [blessing-skin-server](https://github.com/bs-community/blessing-skin-server) 的现代化重写：皮肤、身份、联合认证三线汇聚的中枢。

- **Yggdrasil API 完整兼容**：authlib-injector 直连，既有 MC 客户端与服务器**零改配置**，玩家 UUID 零漂移
- **一键数据迁移**：用户/皮肤/披风/角色绑定从 blessing-skin-server 平滑迁移（多算法密码兼容 + 惰性重哈希）
- **现代技术栈**：Go + Vue 3，单二进制部署，N100 级小主机友好
- **对外数据接口**：供社团官网 / 服务器官网等调用
- **插件扩展**：版本化 hook 合约 + 权限 manifest（社区扩展生态）
- **自托管与数据主权**：零账号依赖、本地直连、单一控制点

## 路线图

| 阶段 | 内容 | 状态 |
|---|---|---|
| M0 | 技术验证（脚本引擎对比、协议冒烟） | 进行中 |
| M1 | 骨架 + 账号 + 纹理上传 | 计划中 |
| M2 | Yggdrasil API 全量 + 真机 E2E | 计划中 |
| M3 | 迁移工具 | 计划中 |
| M4 | 前端 + 管理面板 | 计划中 |
| M5 | 切流上线 | 计划中 |

二期规划：MUA Union 联合认证兼容、正版账号绑定（Auth Source 开关）、启动器免密登录（OAuth2/OIDC）、插件宿主运行时。

## 文档

- [`docs/blueprint.md`](docs/blueprint.md) —— 项目蓝图：目标、架构、迁移方案、技术路线对比、开源协议
- [`docs/standards.md`](docs/standards.md) —— 协作开发规范与 Agent 开发指南

## 作者与署名

本项目由 **YMZS（[GitHub @YimingZhanshen](https://github.com/YimingZhanshen)）** 发起并享有原始著作权。治理结构见 [`GOVERNANCE.md`](GOVERNANCE.md)，引用格式见 [`CITATION.cff`](CITATION.cff)。

## 许可

AGPL-3.0（见 [`LICENSE`](LICENSE)）。版权行为：`Copyright (c) 2026 YMZS (YimingZhanshen) and SkinNexus contributors`。
