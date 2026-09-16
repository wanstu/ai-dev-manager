# Phase 8 Research — Agent Run Lifecycle

## Existing seams to reuse

### Persistent owner

`internal/gateway/runtime_owner.go` already provides the correct process-instance ownership boundary. It owns external MCP sessions and long-running dev processes, has an owner context canceled by `Close`, exposes owner identity through health/status, cleans Environment-owned resources on Environment removal, and intentionally does not persist observed owner resources.

Phase 8 should extend this owner with a `runs map[string]*ownedAgentRun` rather than introduce a second daemon/service.

### Runtime authority

`app.Service.Runtime(environmentID)` resolves the Environment, validates managed worktree identity when applicable, and builds the existing Runtime. `runtime.Runtime.Exec` already enforces allowlisted executable resolution, Environment-relative cwd containment, bounded stdout/stderr, timeout, and context-bound OS process-tree cancellation.

A Run can therefore call `Runtime.Exec` asynchronously under an owner-derived context. It does not need a new command-execution primitive.

### Writer lifecycle

Long-running dev processes call `Environment.RequireWriter` before start and periodically heartbeat the writer. Phase 8 should use the same authority pattern so a Run cannot outlive loss of the writer lease while continuing mutation-capable command execution.

### Gateway acceptance pattern

`process_acceptance_test.go` already proves a resource can outlive the launching MCP client, be observed by a later client, be cleaned at real HTTP Gateway shutdown, and not reappear after restart. Phase 8 should reuse the same real Streamable HTTP helper process pattern for Run acceptance.

## Minimal Run model

Owner-local `ownedAgentRun` should contain:

- stable `run_` ID
- Environment ID
- writer owner
- command input metadata sufficient for status/audit (`executable`, args, cwd)
- owner-derived cancellable context
- done channel
- start/completion timestamps
- lifecycle state: running/succeeded/failed/canceled
- bounded `runtime.CommandResult` when command execution completes
- error kind/message for timeout/runtime/heartbeat failures
- explicit cancellation marker so context cancellation is not misclassified as ordinary command failure

No Run state belongs in `model.State` for Phase 8.

## Lifecycle mapping

- start validates writer and Runtime authority before installing the Run
- async execution transitions running -> succeeded for exit 0, failed for non-zero exit or execution error, canceled for explicit cancel/owner shutdown
- list/status snapshot owner-local state and do not renew writer
- cancel validates matching writer and Run ownership, marks cancellation requested, cancels the context and waits boundedly for completion
- Environment removal and owner close cancel affected running Runs
- restart creates a fresh owner with empty Runs; persisted state must contain no `run_` or run observation fields

## Risks / mitigations

1. **Cancellation classified as failed.** Track cancel intent before calling cancel and prefer canceled when completion races with command result.
2. **Duplicate execution policy.** Use `app.Service.Runtime` + `Runtime.Exec`; do not duplicate allowlist/cwd/worktree validation.
3. **Writer expires during Run.** Heartbeat while running; on failure mark error kind and cancel.
4. **Owner shutdown hangs.** Reuse bounded wait semantics similar to dev process cleanup.
5. **Scope creep into Phase 9.** Run accepts one command only; no steps, plan, reviewer, retry graph, prompt, model or subagent fields.
