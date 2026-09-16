# Phase 5: Persistent Runtime Ownership — Context

## Scope and requirements
Implement LIFE-01, LIFE-02 and LIFE-03 as the first R2 runtime slice. The existing long-lived ADM Gateway process is the persistent local control boundary; do not introduce a second daemon or generic process manager.

## Locked decisions
- The HTTP/stdio Gateway process owns long-lived runtime resources. Short CLI invocations remain management clients and must not become resource owners.
- The first concretely owned resource is the external MCP client session that Phase 3 deliberately kept connect-per-operation. Environment selections and catalog definitions are persisted desired state; live sessions and observed health are in-memory owner state only.
- Restart creates a new owner instance and reconciles from persisted desired state. Never serialize SDK sessions, PIDs of future dev processes, or observed healthy state into `state.json`.
- `environment_mcp_status` must verify the currently owned session (or establish a fresh one) before returning healthy. A cached dead session must be evicted and cannot remain reported healthy.
- `environment_mcp_tools` and `environment_mcp_call` reuse the Gateway-owned session while the desired Environment selection remains enabled. Disable/removal closes or evicts the owned session.
- `/healthz`, `gateway status`, and `gateway_info` expose one runtime owner identity for the running Gateway so independent clients can observe the same owner. Owner identity is process-instance state and changes on restart.
- Clean Gateway shutdown closes all owned MCP sessions. Windows short-command tree cancellation from Phase 4 remains separate and unchanged.
- Phase 6 process start/list/log/port APIs are explicit non-goals. Phase 5 may add a reusable owner lifecycle abstraction, but it must not add a generic process manager or dev-server API.
- Desktop/package work remains frozen. No OpenCode, CodeBuddy, GSD LLM planner/checker/verifier, or other LLM subprocesses.

## Product contract mapping
- ADM-GOAL-001: external Agents use one reliable local development Gateway.
- ADM-CORE-003 / ADM-CORE-004: ownership cannot become a prerequisite for unrelated file/Git-less development operations.
- ADM-CORE-012: Environment selection remains the activation gate for external MCPs.
- ADM-GW-003: broken owned MCP resources fail locally and do not break unrelated Gateway tools.
- ADM-GW-004: reuse the same foreground/detached HTTP Gateway lifecycle; no second daemon.
- ADM-DEV-003: include negative acceptance for disabled/broken MCPs and restart stale-state handling.
- LIFE-01 / LIFE-02 / LIFE-03: persistent owner, desired-vs-observed reconciliation, deterministic cleanup.

## Acceptance boundary
1. Two independent MCP/HTTP clients see the same non-empty owner identity while one Gateway instance runs.
2. Two separate Agent client sessions use the same owned external MCP session rather than reconnect-per-operation.
3. After Gateway restart, owner identity changes, desired MCP selection persists, and a fresh live session is rebuilt from persisted configuration.
4. If the upstream dies, status becomes error; restart never resurrects a previously healthy observed state without a successful new connection/probe.
5. Disabling/removing an MCP or clean Gateway shutdown closes the owned session.
6. Plain file operations and non-Git Environment behavior remain usable with no external MCP configured.

## Explicit non-goals
No dev process lifecycle, logs, ports, worktree isolation, Agent Run lifecycle, orchestration, Desktop feature work, packaging, compatibility/migration layer, or new installed prerequisite.
