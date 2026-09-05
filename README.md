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

## 当前日常用法（v0.4）

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

Agent Run lifecycle（v0.4 Phase 15）：

```powershell
.\ai-dev-manager.exe agent run
.\ai-dev-manager.exe agent list
.\ai-dev-manager.exe agent status <run-id>
.\ai-dev-manager.exe agent cancel <run-id>
```

`agent run [path|workspace-id]` 会沿用 `up` 的 Workspace 解析方式并自动确保 daemon 已启动。Run 由 daemon 持有，因此创建 Run 的 CLI 退出后，其他 CLI invocation 仍可查询或取消同一个 Run。

Phase 15 的默认 production executor 名为 `lifecycle`：它只用于验证 daemon-owned Run 的 ownership / status / cancel，不执行 LLM 或代码任务。

Phase 16 增加第一条真实 Planner → Executor → Reviewer workflow：

```powershell
.\ai-dev-manager.exe agent run --workflow verify
```

`verify` planner 会根据 Runtime capabilities 生成 `git.status → git.diff → verify.run_many` 计划，Executor 通过现有 Runtime Adapter 执行，Reviewer 根据结构化 verifier 结果给出 `pass/fail`。`agent status <run-id> --json` 可查看完整 plan、step results 和 review。Reviewer `fail` 表示验证未通过，但 orchestration 本身仍可正常 `completed`；capability/runtime 系统错误则进入 Run `error`。

Phase 17 增加 GSD workflow：

```powershell
.\ai-dev-manager.exe agent run --workflow gsd
```

`gsd` 从 Workspace `.planning/STATE.md` 解析当前 Phase/Plan，读取 PROJECT/CONTEXT/PLAN，并只执行 PLAN `## Execution Spec` 中经过 allowlist + Runtime capability 双重校验的结构化 operations。`shell.exec`、worktree 和未知 operation 不会进入执行器。Review `pass` 后，只会把 STATE 推进到仓库中已经存在且身份明确的下一 Plan；缺失、歧义或无 `files.edit` capability 时记录 `advance=blocked`，不会猜测下一步。

Phase 18 增加真实 managed-worktree 并行验证：

```powershell
.\ai-dev-manager.exe agent run --workflow parallel-verify `
  --lane check-a:adm-check-a `
  --lane check-b:adm-check-b
```

`parallel-verify` 要求 2–8 个唯一 lane。每个 lane 先通过现有 managed Git worktree API 创建隔离 checkout，再用 base Workspace 的 EffectiveConfig 构建 observed-only derived Runtime，并发执行 verify workflow。Parent Run 聚合 lane review；默认执行后删除 worktree，增加 `--keep-worktrees` 可显式保留。不会自动 merge、删除 branch 或切换 main checkout。

v0.5 Phase 19 开始把这套 managed worktree 能力产品化为持久 Environment：

```powershell
.\ai-dev-manager.exe env create --name coupon-share
.\ai-dev-manager.exe env create --name hotfix --base master
.\ai-dev-manager.exe env list
.\ai-dev-manager.exe env inspect <env-id>
.\ai-dev-manager.exe env destroy <env-id>
```

Environment 默认从创建时当前 checkout HEAD 建立独立 `adm/<name>` branch 与 managed worktree；也可以显式指定 branch/tag/commit 作为 base。默认不会把主 checkout 的 dirty 变动偷偷带入，而是返回 `changes_not_included`。Phase 20 已支持显式 `--include-changes`，会保留 staged / unstaged 区别并复制 untracked non-ignored regular files；ignored 文件和 untracked symlink 不会被静默复制进 Environment。普通 destroy 会拒绝 dirty、未推送工作或 active writer，`--force` 才允许显式丢弃 managed worktree，并且两种 destroy 都不会删除 branch。

```powershell
.\ai-dev-manager.exe env create --name coupon-share --include-changes
.\ai-dev-manager.exe env destroy --force <env-id>
```

Phase 21 补齐长期 Environment 的状态与协作边界。`env inspect` 会报告 dirty、upstream、base `ahead/behind/diverged/base_moved`、activity/stale 和 writer facts；base 明显前进或分叉时最多返回轻量确认 hint，不会自动 rebase/merge/push/commit。Environment 默认 single-writer，writer lease 持久化并跨 daemon restart：

```powershell
.\ai-dev-manager.exe env writer acquire --owner codex-task-1 <env-id>
.\ai-dev-manager.exe env writer release --owner codex-task-1 <env-id>
.\ai-dev-manager.exe env writer release --force <env-id>
```

同 owner 再 acquire 是 renew；不同 owner 会得到冲突。7 天 inactivity 目前只作为 advisory stale 状态，不会自动删除 Environment，也不会自动释放 writer。

Agent Run 当前是 observed state：daemon stop/restart 会结束 active Runs，新 daemon 不会把旧 Run 伪装成仍在运行。

原有 `workspace`、`runtime`、`start/status/stop`、`inspect`、`serve`、`mcp` 等底层命令继续保留，适合脚本、调试和详细控制。
