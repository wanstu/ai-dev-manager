# ai-dev-manager

`ai-dev-manager` 是一个面向本地 AI Coding 的 Workspace / Runtime 管理核心。

它不绑定 CodexPro、DevSpace、Codex CLI、Claude Code 或某个桌面 UI，而是提供统一的：

- Workspace 管理与隔离
- Global / Profile / Project / Runtime 配置继承
- MCP Registry 与项目私有 MCP
- Skill / GSD / Agent 能力发现与继承
- Shell / Git / Worktree / Docker 执行能力
- Test / Lint / Build / Debug / Verification
- Runtime capability 与权限策略
- 对 CodexPro、DevSpace、外部 MCP Runtime 的适配边界

当前阶段按 GSD 思路开发：先固定项目目标、边界、需求追踪和阶段退出条件，再逐 Phase 实现。

规划入口：`.planning/PROJECT.md`
路线图：`.planning/ROADMAP.md`
当前状态：`.planning/STATE.md`
