# Phase 15 Context — Agent Run Lifecycle

## Goal

建立 v0.4 的第一个真实纵向切片：Agent Run 由 v0.3 daemon 长期持有，CLI 只是 local Control API 的薄客户端。一个 `agent run` 创建的 Run 在启动 CLI 退出后仍保持可查询，并可由另一个 CLI `list/status/cancel`。

## Locked Scope

Phase 15 只解决 **Run ownership / state / cancellation**，不提前实现 Planner / Executor / Reviewer、GSD phase 自动执行、通用 Process Manager 或多 worktree orchestration。

### Public lifecycle

- `agent run [path|workspace-id]`
- `agent list`
- `agent status <run-id>`
- `agent cancel <run-id>`
- 支持现有 `--json` 输出。
- `agent run` 的 path 解析沿用 `up`：省略 target 时使用 cwd；未知 path 可自动注册 Workspace。
- `agent run` 自动确保 daemon 已启动。

### Run model

Run 至少包含：

- `run_id`
- `workspace_id`
- `executor`
- `state`
- `created_at`
- `started_at`
- `finished_at`
- `error`

状态机第一版：

`running -> completed | cancelled | error`

Run ID 使用 `run_` + crypto/rand hex，不能依赖 PID 或数组序号。

### Ownership / persistence decision

- Run 是 **daemon observed state**，不是 desired state。
- Phase 15 不持久化 goroutine、context、executor 或 running Run。
- CLI 退出不影响 Run。
- daemon stop/restart 会取消当前 active Runs；新 daemon 不把旧 Run 伪装成仍在运行。
- 后续 GSD 恢复应从 `.planning/` 和明确 checkpoint 重新计划，而不是序列化恢复旧执行对象。

### Executor boundary

Phase 15 必须有真实被使用的 Executor contract，不能创建空接口。

第一版 production executor 名为 `lifecycle`：

- 它明确只是 v0.4 Phase 15 的 lifecycle acceptance executor，不声称执行 LLM/代码任务。
- 启动后保持运行直到 context 被取消。
- 它让真实 CLI / daemon / Control API 可以验证跨进程 ownership 和 cancel。
- Phase 16 将通过同一 contract 接入真正 Planner/Executor workflow。

测试可注入 deterministic fake executor 来验证 completed/error/cancel race，而不依赖外部 AI CLI。

## Security / Architecture

- Agent Control API 继续只存在于 daemon loopback Control API 上，不暴露到 Workspace MCP 的 Docker/public listen。
- Workspace 必须在 Registry 中真实存在后才能创建 Run。
- Agent owner 在 daemon shutdown 时先取消 Runs，再关闭 Workspace Runtime / MCP resources。
- 不新增任意 shell / generic process execution入口。
- 不修改 `D:\projects\codexprov4`。

## Exit Criteria

1. `agent run .` 返回 `run_*`，启动 CLI 退出后另一个 invocation 仍能 `agent status` 看到 running。
2. `agent list` 能列出 daemon 当前拥有的 Run，并保持 Workspace identity。
3. `agent cancel <run-id>` 后状态变为 cancelled，重复 cancel 不产生第二个 Run/异常状态。
4. 不存在的 Workspace / Run 返回明确错误；两个 Workspace Run 不串状态。
5. daemon shutdown 会取消 active Runs；restart 后旧 Run 不被误报 running。
6. unit + CLI/process acceptance 覆盖 running/completed/error/cancel 基础状态机。
7. `go fmt ./...`、`go test ./...`、`go vet ./...`、`go build ./cmd/ai-dev-manager` 通过。

## Deferred

- 真正 Planner / Executor / Reviewer — Phase 16+
- GSD `.planning` phase execution / auto-advance — Phase 17+
- multi-worktree / subagent parallel orchestration — Phase 18+
- 通用 Process Manager / Docker / Debug — v0.5
