# Phase 6 Plan 06-01 Summary — Gateway-owned Dev Process Lifecycle

**Status:** complete and integrated to local master
**Date:** 2026-09-08 (regression record refreshed; dogfood ran 2026-09-07)

## Delivered

- Added Runtime `PrepareCommand` so long-lived processes reuse short-`exec` executable allowlist, Environment-relative cwd containment and OS cancellation setup.
- Added Windows owned-PID TCP listening-port observation. The Agent-facing API never accepts arbitrary PIDs; non-Windows intentionally returns no port facts rather than falling back to a generic host scanner.
- Extended Phase 5 `runtimeOwner` with a second owned resource type: long-running development processes with stable `proc_` identity.
- Added bounded stdout/stderr tail capture, running/exited status, retained exit/log observation for the owner lifetime, and owned listening-port facts.
- Added writer heartbeat for running owned processes so the launching client can disconnect without weakening the single-writer root lease.
- Added Environment-scoped Gateway tools: `process_start`, `process_list`, `process_status`, `process_logs`, `process_stop`.
- Start/stop require the appropriate writer authority; list/status/logs are read-only. Stop accepts only stable ADM process identity, not raw PID.
- Added deterministic process-tree cleanup on explicit stop, Environment removal and owner/Gateway shutdown. Environment removal also forgets its owner-local process records.
- Process/log/port observation remains in owner memory only and is not persisted or resurrected after Gateway restart.
- Updated `docs/PRODUCT_CONTRACT.md` with explicit PROC-01..03 semantics while retaining arbitrary/general OS process management as a non-goal.

## Dogfood

A private current-source detached HTTP Gateway at `127.0.0.1:36728`, a non-Git project, standalone dev-server executable and independent MCP probe processes proved the complete lifecycle. The first process (`proc_d5c103ee5ee32e51`, port 37503) survived its launching probe, was inspected from a later probe, served real HTTP traffic, and was explicitly stopped. The second (`proc_eea861f94fb328c9`, port 37529) was deliberately left running and was cleaned by graceful Gateway shutdown. Restart produced a new owner and an empty process list. Private state contained no process observations.

## Pre-integration validation

- Phase 6 plan-structure: valid, 3 tasks, zero errors/warnings.
- Cross-process restart acceptance ×5: pass in 5.745s with structured port assertions.
- `go test ./internal/gateway -count=1`: pass in 17.872s.
- `TestProcessStartRenewsWriterLeaseBeforeChildLifetime`: focused pass; source review confirms immediate writer renewal before child start.
- Full Go regression: all 16 packages covered by the separate Gateway command and two bounded package groups; 11 test-bearing packages passed and 5 no-test packages compiled. Aggregate connector interruptions are recorded separately in `evidence/regression.json`, not as product failures or completed aggregate passes.
- Exact-capacity log truncation regression: reproduced red, then all eight tail cases passed; focused tail/cross-client/writer tests passed in 0.993s.
- `go vet ./...`: pass.
- `git diff --check`: pass.

## Post-integration validation

On 2026-09-08, local `master` fast-forwarded `0ad5488 -> 6dd87d6` after explicit user authorization. The main-worktree Gateway suite passed in 15.004s, all remaining packages passed tests or compiled, and vet/diff-check passed. Exact evidence: `evidence/integration.json`.

## Non-goals preserved

No Phase 7 worktree lifecycle, Phase 8 Agent Run, orchestration, Desktop feature expansion, generic OS process manager, arbitrary PID control, persisted process state, package/distribution work, or LLM subagent was added.

## Stop point

Phase 6 is integrated and verified on local `master`. No push, `phase complete 06` or `phase uat-passed 06` command was run. Phase 7 has not started and requires separate authorization.
