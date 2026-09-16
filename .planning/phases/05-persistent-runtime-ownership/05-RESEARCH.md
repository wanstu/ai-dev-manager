# Phase 5 local research

- `internal/gateway/lifecycle.go` already defines the one HTTP Gateway process lifecycle and `/healthz` PID/version discovery. Phase 5 should extend this boundary rather than introduce another daemon.
- `internal/gateway/server.go` currently creates and closes a new external MCP `ClientSession` for every `environment_mcp_tools` / `environment_mcp_call`. This is the exact Phase 3 deferred connection-ownership seam.
- `internal/app/mcp_health.go` performs a bounded transient connect + list-tools probe and explicitly says persistent lifecycle belongs to a later phase. Its configuration/secret-resolution semantics must remain canonical.
- `internal/app/service.go` reconstructs ordinary `runtime.Runtime` values per operation; there is no current long-lived generic process state to preserve. Phase 5 therefore owns external MCP sessions first and leaves Phase 6 dev-process APIs untouched.
- Persisted desired MCP state already exists as global catalog definitions plus `Environment.EnabledMCPIDs`. No new persisted session model is needed.
- `RunHTTP` already receives a cancellation context and performs graceful HTTP shutdown; it is the natural place to start reconciliation and `defer` owner cleanup. `RunStdio` can use the same owner lifecycle for the lifetime of its protocol process.
- `gateway_info` is an existing low-cost Agent-facing identity/status surface; enrich it with owner facts instead of adding a new tool solely for owner identity.
- Existing Phase 3 `TestMCPHealthLifecycleEndToEnd` is the semantic regression baseline: enabled/disabled/healthy/error, secret non-leakage, broken-MCP locality and real external call must remain green.
- A small `runtimeOwner` inside `internal/gateway` avoids pushing SDK session ownership into persistence/application state. It can use an interface around `mcp.ClientSession` for deterministic unit tests and the real connector for acceptance.
- `ClientSession.Close()` is idempotent and concurrency-safe in the installed MCP SDK, so owner cleanup can safely close cached sessions once and tolerate races with shutdown.
- GitNexus remains unavailable; use source/call-site search, diff review, Go tests and real HTTP acceptance. No LLM subagents.
