# ADM v1.1 文档索引

这里是 `adm` / `adm-desktop` / Agent Gateway 的权威文档入口。README 只保留产品简介和最短路径；详细使用与机制放在本目录。

## 建议阅读顺序

1. [快速开始](quickstart.md) — 从 release artifact 到第一个 Workspace/Environment。
2. [完整用户手册](USER_GUIDE.md) — Workspace、Environment、Writer、Exec、MCP、Skill、Memory、Verifier、Process、Run、Temporary Environment、Desktop 的实际用法。
3. [CLI 完整参考](cli.md) — `adm` v1.1 所有主要管理命令、flags 与副作用边界。
4. [Agent Gateway / MCP Tool 参考](AGENT_GATEWAY.md) — `/mcp` 工具、参数、Writer 要求、Admin-only 能力和推荐 Agent workflow。
5. [工作机制与架构](ARCHITECTURE.md) — desired state / runtime observation、Agent/Admin surface、Writer、allowlist、MCP/Skill lifecycle、retention/cleanup 等内部机制。

## 人类管理

- [Desktop 管理端](desktop.md) — Wails Desktop、连接 profiles、本地 Gateway 控制、托盘和 autostart。
- [MCP、Skill 与 Memory 专题](catalog-memory.md) — catalog/source/selection/scope 的集中说明。
- [产品语义合同](PRODUCT_CONTRACT.md) — ADM 的产品 invariant、authority、安全边界和 acceptance contract。
- [当前实现状态](STATUS.md) — 历史阶段状态/已知限制；日常使用优先看本页上方 v1.1 文档。

## 构建与发布

- [打包与 GitHub Actions](packaging.md) — `dist/`、Wails build、RC/tag artifact 和 release workflow。

## 核心概念速查

| 概念 | 一句话定义 |
|---|---|
| Workspace | 明确登记给 ADM 的本地目录；Git 可选 |
| Environment | 位于 Workspace 内、以目录为 root 的持久开发上下文 |
| Gateway | 真正运行的 MCP/HTTP 服务进程 |
| Runtime Owner | 当前 Gateway 生命周期内持有 MCP session/process/run/vfrun observation 的 owner |
| Writer lease | 同一 physical root 的单写者、有期限 mutation authority |
| Exec allowlist | 可被 ADM Runtime 启动的 executable 管理状态 |
| MCP catalog | 全局 desired MCP definitions；Environment 只保存启用 ID |
| Skill Source/catalog | 显式 source + refresh 得到的真实 `SKILL.md` artifacts；Environment 只保存启用 ID |
| Memory | Global 与 Environment-private 两个显式持久 scope |
| Verifier | Environment-scoped structured test/lint/build/custom definition |
| Process | Gateway-owned long-running development process (`proc_`) |
| Run | Gateway-owned async single-command resource (`run_`) |
| Verifier Run | Gateway-owned async structured verifier resource (`vfrun_`) |
| Temporary Environment | 带 owner + TTL retention 的普通稳定 `env_`，cleanup preview-first/no-force |

## 历史阶段文档

这些文档保留早期实现追溯，不应代替当前 v1.1 用户手册：

- [Phase 01：Git-independent core](PHASE_01_GIT_INDEPENDENT_CORE.md)
- [Phase 02：Development context](PHASE_02_DEVELOPMENT_CONTEXT.md)
- [Phase 03：Management CLI](PHASE_03_MANAGEMENT_CLI.md)
- [Phase 04：Environment management UX](PHASE_04_ENVIRONMENT_MANAGEMENT_UX.md)
- [Phase 05：Management boundary](PHASE_05_MANAGEMENT_BOUNDARY.md)
- [Phase 06：Desktop UI](PHASE_06_DESKTOP_UI.md)
- [Phase 07：Desktop release readiness](PHASE_07_DESKTOP_RELEASE_READINESS.md)

后续开发 planning 位于 `.planning/`；它记录开发决策，不是用户操作手册。
