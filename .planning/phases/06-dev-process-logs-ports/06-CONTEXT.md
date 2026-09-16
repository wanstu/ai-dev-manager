# Phase 6: Dev Process / Logs / Ports — Context

## Scope
Implement PROC-01, PROC-02 and PROC-03 as the first long-running development-process vertical slice on top of the Phase 5 Gateway runtime owner.

## Locked decisions
- The existing HTTP/stdio Gateway `runtimeOwner` owns dev processes; no second daemon or generic OS process manager.
- `process_start` is Environment-scoped, writer-gated, executable-allowlisted, and cwd-contained exactly like `exec`. The Gateway continues writer heartbeat while the owned process runs so a launching client may exit without weakening single-writer safety.
- Process identity is stable only within the owning Gateway lifetime. Process records/logs/ports are observed in-memory state and are never persisted into `state.json`.
- Later independent clients may list/status/read logs/stop by stable ADM process ID. Query operations do not require the original client connection.
- Logs are bounded tails, separated stdout/stderr, and remain queryable after normal exit until owner shutdown.
- Ports are facts observed only for ADM-owned process PIDs. No arbitrary PID, host-wide port scanner, kill-by-PID, or generic process inspection API.
- Explicit stop and clean Gateway shutdown terminate the owned process tree deterministically. A clean Gateway restart does not resurrect a prior observed dev process.
- Phase 7 worktree isolation, Phase 8 Agent Run semantics, orchestration, Desktop and packaging remain out of scope.

## Acceptance boundary
1. Start a real local dev server through Gateway and return before the server exits.
2. A second independent client lists/statuses the same stable process ID.
3. Logs contain bounded server output and are queryable from the second client.
4. Status reports the real listening port for the owned server process.
5. Wrong writer/forbidden executable/path escape are rejected locally; ordinary file work still functions without a process.
6. Stop terminates the server and releases its port; owner shutdown cleans any remaining owned process.
7. Process/log/port observed state is absent from persisted ADM state and not resurrected after Gateway restart.
