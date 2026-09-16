# Phase 5 Plan 05-01 Summary — Persistent Runtime Ownership

## Delivered

- Added an app-owned `ResolveMCPActivation` boundary so Environment selection, catalog transport checks, unresolved env refs, and activation-time secret expansion have one canonical implementation shared by transient health probing and the persistent Gateway owner.
- Added `runtimeOwner` as process-instance state owned by the existing Gateway. It has a stable per-instance owner ID/PID/start time, owns external MCP client sessions keyed by Environment+MCP, reuses live sessions, verifies health before reporting healthy, evicts dead sessions, reconciles persisted desired selections on startup, and closes sessions idempotently.
- Routed real HTTP/stdio Gateway `environment_mcp_status`, `environment_mcp_tools`, and `environment_mcp_call` through the owner. Disable, MCP removal, and Environment removal evict corresponding resources.
- Exposed runtime owner identity through `gateway_info`, `/healthz`, `gateway status`, and `doctor` without persisting it.
- Added owner-bound local HTTP graceful shutdown for current Gateways. The request must present the exact current owner ID; wrong IDs are refused. `gateway stop` uses this path when owner identity is available and retains the pre-existing safe legacy process-termination fallback for older/no-owner health responses.
- Preserved Phase 3 four-state MCP health and secret-leakage behavior and Phase 4 Windows short-command process-tree cancellation unchanged.

## Verification highlights

- Deterministic owner tests prove one connect across repeated use and independent Agent sessions, fresh owner reconstruction, dead-session eviction, disable cleanup, and idempotent shutdown.
- A red restart-identity test initially reproduced an owner-ID collision for two same-process owner instances; adding a monotonic process-local sequence fixed it.
- Cross-process real HTTP restart reconciliation passed, then passed five consecutive runs.
- Independent current-source CLI dogfood used private `ADM_V2_HOME`, detached Gateway `127.0.0.1:43761`, real MCP fixture `127.0.0.1:43762`, two independent clients, graceful stop/restart, desired-state persistence inspection, and dead-upstream error/eviction.
- Final regression target: `go test ./...`, `go vet ./...`, `git diff --check`.

Evidence: `05-VERIFICATION.md`, `05-UAT.md`, and `evidence/`.

## Non-goals retained

No dev process start/list/status, log storage/query, port discovery, worktree lifecycle, Agent Run lifecycle, orchestration, Desktop expansion, packaging, compatibility layer, or new daemon was added. Phase 6 remains not started.

## State

Plan 05-01 is locally verified and ready for the normal integration/review decision. No automatic Phase 6 transition was performed.
