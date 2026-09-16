# Phase 6 Research

## Existing reusable boundaries
- `internal/runtime.Runtime.Exec` already owns allowlist validation, Environment-relative cwd containment and Windows hidden-process cancellation behavior.
- `internal/runtime/process_windows.go` upgrades `exec.CommandContext` cancellation to bounded `taskkill /T /F`, which is suitable for owned long-running process-tree stop/owner shutdown.
- `internal/app.Service.Exec` proves the writer-gated + heartbeat pattern. Phase 6 must preserve that safety while moving process lifetime from request context to Gateway owner context.
- Phase 5 `internal/gateway/runtime_owner.go` is already the single long-lived resource owner with idempotent `Close`; dev processes should extend it rather than introduce another manager.

## Minimal design
1. Add a Runtime command-construction seam that validates allowlist/cwd and returns a context-bound `exec.Cmd` without running it.
2. Extend `runtimeOwner` with owned process records keyed by ADM-generated `proc_` identity. Each record holds process state, bounded stdout/stderr tails, cancel function, wait completion, start/exit timestamps and observed owned-PID ports.
3. Keep the original writer lease alive while a process is running. A heartbeat failure cancels the process rather than allowing unsafe concurrent mutation.
4. Expose Gateway tools `process_start`, `process_list`, `process_status`, `process_logs`, `process_stop`; all are Environment-ID scoped. Start/stop require the matching writer owner; list/status/logs are read-only.
5. Port observation is platform-specific and restricted to the owned PID. On Windows parse listening TCP rows from the OS network table/utility; other platforms may return an empty fact set until a native implementation is added, but no generic PID input is exposed.

## Security / privacy
- Do not return raw command arguments in normal process status/log metadata because arguments may contain secrets.
- Bound logs in memory; no log files or persisted process state in `state.json`.
- Do not accept arbitrary PIDs or paths.
- Environment removal must stop that Environment's owned processes before/after record removal as appropriate.

## Non-goals
No restart resurrection, process adoption, arbitrary process discovery, OS-wide port scanning, log persistence, worktree management, Agent Runs, Desktop UI, installer, compatibility layer or LLM subagent.
