# Phase 22 Context — Gateway Host & Discovery

## Goal

建立 v0.6 的第一个真实 vertical slice：daemon 持有一个独立于 per-Workspace Direct MCP 的 **Agent MCP Gateway**，外部 Agent 只需配置这个 Gateway endpoint，就能发现 ADM、Workspace 和 v0.5 Environment 状态。

Phase 22 只做 Gateway host/lifecycle + read-only discovery，不做 Environment create/destroy/writer mutation，也不做 environment-routed files/git/verify。

## Product Boundary

Gateway 是 Agent-facing 推荐入口：

```text
Agent
  │ one MCP config
  ▼
ADM Gateway MCP
  │
  ├─ gateway_info
  ├─ workspace_list
  ├─ environment_list
  └─ environment_inspect
```

现有 Direct MCP 保留：

```text
Workspace Runtime -> existing mcpserver.New(adapter)
```

Phase 22 不修改它的 tool names/schema/transport semantics。

## Stable endpoint semantics

“只配置一次 MCP”要求 Gateway URL 在 daemon restart 后不能随机漂移。

Locked behavior：

1. Gateway desired state 单独持久化，不复用 Workspace Runtime desired store。
2. 首次 `gateway up`：
   - 若显式 `--listen 127.0.0.1:PORT`，使用并持久化该地址。
   - 若未显式 listen，允许从 `127.0.0.1:0` 选择空闲端口；成功 listen 后把 **实际地址**（例如 `127.0.0.1:43127`）持久化为后续 desired listen。
3. daemon restart 后如果 desired running=true，必须优先重新绑定持久化的实际地址。
4. 如果原端口被其他进程占用，Gateway 进入 error/不可用状态并报告事实；**不得静默换新端口**，否则 Agent MCP 配置失效。
5. `gateway down` 显式把 desired running 设为 false 并停止 listener。
6. v0.6 默认只允许 loopback listen；remote/LAN/public exposure/auth/tunnel 后置。

## Daemon ownership

Gateway listener 必须由 daemon 持有：

- CLI 退出后 Gateway 继续运行。
- daemon restart 根据 desired state 恢复。
- daemon clean stop 会停止 observed listener，但保留 desired state；下次 daemon start 恢复。
- 后续如有 `ctl shutdown` 清 desired semantics，可在对应 phase/acceptance 中决定；Phase 22 优先保持与 Runtime desired-state 的 stop/start 心智一致。

不要把 Gateway server 生命周期放进一次性 foreground CLI goroutine。

## Gateway core dependencies

Gateway discovery 只需要窄接口，不依赖 UI/Agent Manager：

- WorkspaceLister: `List() ([]model.Workspace, error)`
- EnvironmentLister: `List() ([]environment.Environment, error)`
- EnvironmentInspector: `Inspect(ctx, envID) (environment.InspectResult, error)`

Gateway protocol adapter不应该自行读取 `environments.json` 或任意 worktree path；`environment_inspect` 必须继续走 Environment Manager 的 managed worktree validation。

## Tool schema

### gateway_info

返回：

- product/name
- milestone/API identity（版本字符串可保持简单，不绑定 Git tag 自动探测）
- gateway capabilities / tool categories
- direct MCP compatibility note

不得返回 secret/config env values。

### workspace_list

返回稳定 Workspace identity 和 Agent 需要的最小事实：

- workspace_id
- path
- profile/runtime selection（如果 model 中已有）

不返回 EffectiveConfig secret values。

### environment_list

返回持久 Environment summaries：

- environment_id
- workspace_id
- name
- branch/base_ref/base_commit
- state
- worktree path（当前 v0.5 CLI/inspect 已公开；Phase 22 保持一致）
- writer owner（如有）
- timestamps

`environment_list` 是 registry list，不因为列出而 touch activity。

### environment_inspect

输入：

```text
environment_id
```

返回 v0.5 `InspectResult`：

- Environment
- facts
- warnings
- hints
- capabilities

不得新增 required_action / next_step。

## Error semantics

Discovery domain error要让 Agent 能看懂：

- invalid input由 MCP input schema 拒绝或返回明确错误。
- `environment_not_found`、`worktree_missing` 等 Environment domain error不能被改写成泛化的“gateway failed”。
- Phase 22可以先通过 MCP tool error content暴露稳定 `code: message`；Phase 23再形成统一 structured domain-error envelope用于 lifecycle tools。

不要泄露原始内部路径以外的敏感系统错误、stack trace或 secret values。

## HTTP Host

新增 Gateway handler/server，不把一个 fake Runtime 塞给现有 `mcpserver.New`。

推荐边界：

- `internal/adapter/mcpgateway`：MCP protocol/tool schema + HTTP handler。
- `internal/daemon/gateway_owner.go`：desired state、listener/server ownership、reconcile。

如果复用 Host Manager 会迫使它假装有 WorkspaceID/RuntimeID，则不要为了复用而扭曲现有类型；Gateway 是全局 control-plane MCP，不是 Workspace Runtime MCP。

## CLI

Phase 22 增加：

```powershell
ai-dev-manager gateway up [--listen 127.0.0.1:PORT]
ai-dev-manager gateway status
ai-dev-manager gateway down
```

行为：

- `gateway up` 自动确保 daemon running。
- 首次动态端口选择后输出稳定 endpoint。
- repeated up应幂等，除非请求的显式 listen 与 persisted listen冲突；冲突返回明确错误，不静默搬迁。
- status返回 desired/observed/listen/endpoint/state/error。
- down不删除任何 Environment。

## Exit Criteria

1. Direct MCP tests保持全绿，现有 per-Workspace tool surface无回归。
2. `gateway up` 首次可以从 loopback动态端口启动并返回 `/mcp` endpoint。
3. persisted desired listen保存为实际端口，而不是 `:0`。
4. independent CLI `gateway status` 能看到同一 endpoint。
5. daemon stop/start 后 Gateway恢复到完全相同 endpoint。
6. 端口被占用时 daemon/Gateway不静默换端口，status报告 error/unavailable。
7. real MCP Streamable HTTP client调用 `gateway_info/workspace_list/environment_list/environment_inspect` 成功。
8. `environment_inspect` 继续触发 managed worktree validation并保留 facts/warnings/hints语义。
9. Gateway默认拒绝non-loopback listen。
10. targeted + full fmt/test/vet/build通过。
11. Phase 22完成后自动进入 Phase 23；不提前实现 lifecycle mutation或 routed Runtime tools。
