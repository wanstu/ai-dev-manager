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

## 当前日常用法（v0.3.1）

构建：

```powershell
go build -o ai-dev-manager.exe ./cmd/ai-dev-manager
```

在项目目录中，一条命令即可自动注册 Workspace、启动 daemon 和 Runtime：

```powershell
.\ai-dev-manager.exe up
```

如果 MCP Client 运行在 Docker 中：

```powershell
.\ai-dev-manager.exe up --docker
```

输出会同时给出本机与 Docker 可用地址，例如：

```text
local     http://127.0.0.1:31857/mcp
docker    http://host.docker.internal:31857/mcp
```

查看全部 Workspace / Runtime：

```powershell
.\ai-dev-manager.exe ps
```

停止当前 Workspace，并取消后续自动恢复：

```powershell
.\ai-dev-manager.exe down
```

Daemon 管理：

```powershell
.\ai-dev-manager.exe ctl status
.\ai-dev-manager.exe ctl start
.\ai-dev-manager.exe ctl stop
.\ai-dev-manager.exe ctl restart
.\ai-dev-manager.exe ctl shutdown
```

`ctl stop` 只停止 daemon，保留 desired runtime，因此下次 `ctl start` 会自动恢复。`ctl shutdown` 会清除 desired runtime 后再停止 daemon，下次启动不会自动恢复。

原有 `workspace`、`runtime`、`start/status/stop`、`inspect`、`serve`、`mcp` 等底层命令继续保留，适合脚本、调试和详细控制。
