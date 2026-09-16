# Phase 8 Verification — Agent Run Lifecycle

**Status:** passed locally; integration review pending
**Date:** 2026-09-08
**Implementation:** `a74b318f883e65a2ff8eb62fc043271bb66c2277`
**Requirement:** ARUN-01, constrained by existing LIFE-01/LIFE-03, writer and Runtime authority contracts.

## Result

Phase 8 implements a stable owner-local Agent Run lifecycle without introducing Phase 9 orchestration. A `run_` resource is a single asynchronous Environment-scoped command owned by the persistent Gateway. The launching MCP request/client may exit while the Run remains observable to later clients connected to that same owner.

Start reuses the existing authority seams: `Environment.RequireWriter`, `app.Service.Runtime`, and `runtime.Runtime.Exec`. As a result, allowlisted executable policy, cwd containment, managed-worktree validation, timeout, bounded command output and OS process-tree cancellation are not duplicated in a Run-specific shell path.

Run states are `running`, `succeeded`, `failed`, and `canceled`. Terminal status retains bounded stdout/stderr and an exit code when the command reached an ordinary process exit. Timeout is a failed Run with `error_kind=timeout`. Explicit cancel and owner-derived context cancellation produce `canceled`.

Run observations exist only in the live Gateway owner. `state.json` contains no Run ID/result/owner-count field. Graceful owner close cancels and waits for running Runs. A new owner after restart has an empty Run set and rejects prior Run IDs.

## Acceptance matrix

| ID | Result | Evidence |
|---|---|---|
| P1 stable `run_` identity / asynchronous start | pass | `TestAgentRunLifecycleAcrossAgentSessions`; real HTTP restart test starts a loopback helper and receives `run_` before helper termination |
| P2 later-client visibility | pass | launching session closes, a later in-memory and real HTTP MCP client successfully `run_list` / `run_status` the same running Run |
| P3 lifecycle states | pass | `TestAgentRunTerminalStatesAndAuthority` proves exit 0 → `succeeded`, exit 7 → `failed`, timeout → `failed/timeout`; explicit cancel → `canceled` |
| P4 writer and Runtime authority | pass | wrong writer cancellation rejected; forbidden executable and escaped cwd rejected before Run installation; `StartAgentRun` resolves the existing Runtime before asynchronous execution |
| P5 bounded result | pass | helper emits 4096 bytes with `max_output_bytes=64`; retained stdout length is exactly 64 |
| P6 owner/drop cleanup | pass | `TestAgentRunDropEnvironmentCancelsAndForgets` and `TestAgentRunOwnerCloseCancelsActiveRun` cancel real helper process trees and release their ports |
| P7 no persistence / restart resurrection | pass | real HTTP subprocess acceptance checks `state.json` for Run observation leakage, gracefully stops a Gateway with an active Run, restarts with a distinct owner, gets empty `run_list`, and rejects old ID |
| P8 optional/local failure and regression | pass | restarted Gateway still reads ordinary project file; full Gateway suite passes; all repository packages pass/compile; no Git/worktree prerequisite added by Run lifecycle |

## Real Streamable HTTP restart acceptance

`TestAgentRunLifecycleAcrossRealGatewayRestart` launches the actual test-hosted HTTP Gateway in a separate OS process using the same `RunHTTP` path as existing lifecycle acceptance. It starts a real allowlisted helper through `run_start`, closes the launching client, connects another client, observes the same running Run, cancels it and verifies the helper port closes.

A second Run is deliberately left running. Graceful Gateway stop closes that Run and releases its port. A restarted Gateway reports a different owner ID, returns no prior Runs, rejects the old `run_` identity, and still serves an ordinary Environment `read` operation. The acceptance was repeated three times successfully.

## Concurrency gate

`go test -race ./internal/gateway -run ^TestAgentRun -count=1` passed in 9.746s after the final acceptance test was present. This exercises the owner Run map, Run state locks, writer heartbeat, cancellation, owner close/drop and real restart path under the race detector.

## Regression

Final evidence:

- `go test ./internal/gateway -run TestAgentRun -count=1` — pass in 4.072s.
- `go test -race ./internal/gateway -run ^TestAgentRun -count=1` — pass in 9.746s.
- `go test ./internal/gateway -run ^TestAgentRunLifecycleAcrossRealGatewayRestart$ -count=3` — pass in 2.966s.
- `go test ./internal/gateway -count=1` — pass in 43.320s.
- `go test ./...` — pass after all packages were covered and caches warmed.
- `go vet ./...` — pass.
- `git diff --check` — pass (line-ending warnings only, no whitespace errors).

One earlier aggregate `go test ./...` invocation was cut off by the project tool timeout and returned no test-failure output. It is recorded as an incomplete execution only. The Gateway and all remaining packages subsequently passed in bounded groups, followed by a successful aggregate `go test ./...`; it is not treated as a product failure or as acceptance evidence.

Exact command/result metadata is in `evidence/regression.json`.

## Scope review

No Planner/Executor/Reviewer workflow, GSD execution engine, run persistence/resume, parallel lane scheduling, automatic worktree allocation, automatic Git merge/rebase/push, Desktop Run UI, LLM subagent, or second daemon was introduced.

Phase 8 is locally verified and stops for integration review. Local `master` is unchanged by Phase 8 and no push was performed. Phase 9 has not started.
