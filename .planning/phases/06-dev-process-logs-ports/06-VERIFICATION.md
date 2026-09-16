# Phase 6 Verification — Dev Process / Logs / Ports

**Status:** passed locally; integrated to local master
**Date:** 2026-09-08 (latest regression record; independent dogfood ran 2026-09-07)
**Requirements:** PROC-01, PROC-02, PROC-03 plus LIFE-01/LIFE-03 and ADM-DEV-001..004 constraints named by 06-01.

## Result

Phase 6 adds the first real long-running development-process vertical slice to the existing persistent Gateway owner. An allowlisted Environment-scoped command can be started as a Gateway-owned `proc_` resource, outlive the Agent request/client that launched it, and later be listed, inspected, read for bounded stdout/stderr tails, observed for owned listening TCP ports, and stopped by stable ADM identity. No second daemon, raw-PID control tool, host-wide process API, process persistence, worktree lifecycle, or Agent Run abstraction was introduced.

`process_start` reuses the same Runtime executable allowlist, Environment-relative cwd containment and OS process-tree cancellation setup as short `exec`. Start and stop are writer-gated. `process_start` calls `Environment.RequireWriter()` before preparing/starting the child; `RequireWriter` immediately calls `renewWriter`, updates `LastSeenAt`, and sets `ExpiresAt = now + TTL`. `TestProcessStartRenewsWriterLeaseBeforeChildLifetime` passed and verifies that start advances both timestamps. The child therefore starts with a freshly renewed lease rather than waiting for the first periodic heartbeat; the owner continues heartbeating while the process remains running. Read-only process list/status/logs are available to later clients connected to the same Gateway.

## Acceptance

| ID | Result | Evidence |
|---|---|---|
| P1 stable long-running `proc_` identity | pass | `TestRuntimeOwnerDevProcessLifecycleAcrossAgentSessions`; independent dogfood started `proc_d5c103ee5ee32e51` and the launching probe exited while PID 42712 remained running |
| P2 later client sees same process | pass | second independent probe returned the same `proc_d5c103ee5ee32e51`, state `running`, PID 42712 |
| P3 bounded queryable logs | pass | `TestTailLogBufferKeepsBoundedTail`; independent second probe read stdout `phase6-dogfood-stdout port=37503` and stderr `phase6-dogfood-stderr ready`; logs remain queryable after process exit while owner lives |
| P4 listening port is an owned-process fact | pass on current Windows dogfood target | Runtime test observes only the supplied owned PID; independent status reported 37503 and direct HTTP returned `phase6-dogfood-ok` |
| P5 authority and negative cases | pass | forbidden executable and escaped cwd tests reject start; wrong writer cannot stop; `TestProcessStartRenewsWriterLeaseBeforeChildLifetime` passed, with source review confirming immediate `RequireWriter` renewal before child start; Runtime `PrepareCommand` is the shared allowlist/cwd/cancellation seam |
| P6 deterministic stop/owner cleanup | pass | explicit `process_stop` released port 37503; second process `proc_eea861f94fb328c9` on 37529 was not explicitly stopped and was cleaned by owner-bound graceful Gateway shutdown; Environment drop focused test stops and forgets owned records |
| P7 no observed-state persistence/resurrection | pass | private `state.json` contains neither `proc_` IDs, `owned_dev_processes`, nor `listening_ports`; restart changed owner and `process_list` returned `[]` |
| P8 optional-capability / scope boundary | pass | full regression remains green; no Git/worktree requirement was added; Agent API accepts ADM process IDs rather than arbitrary PIDs; Phase 7/8 APIs are absent |

## Independent external dogfood

Current source built three task-owned binaries under ignored `.tmp/phase6-dogfood`: the real ADM CLI/Gateway, a small loopback HTTP dev server, and a separate MCP probe. The run used a private `ADM_V2_HOME`, a non-Git project root, and detached Gateway `127.0.0.1:36728`; shared Gateway 41137 was not touched.

The CLI created Workspace `ws_9ddd578ae0cba1d3`, Environment `env_32b565c8340e0c36`, allowlisted the dev-server executable, acquired writer `phase6-dogfood-writer`, and detached the current-source Gateway. The first standalone probe started `proc_d5c103ee5ee32e51` then exited. A second standalone probe observed the same running process, bounded logs and listening port 37503; direct HTTP traffic to that port returned `phase6-dogfood-ok`. Explicit `process_stop` changed the process to exited and the port closed.

A second process, `proc_eea861f94fb328c9` on port 37529, was intentionally left running. `gateway stop` gracefully stopped Gateway owner `owner_22220_18d30be9a67cbd0c_1`; both the Gateway endpoint and dev-server port closed. Restart produced owner `owner_5396_18d30c103bbe92d4_1`, and `process_list` returned an empty array. Persisted state contained none of the process identities or process/port observation fields.

## Platform boundary

Listening-port observation is concretely implemented and dogfooded on the current Windows target. The non-Windows implementation intentionally returns no port facts rather than falling back to a generic host-wide scanner. This verification therefore proves PROC-03 on the active Windows dogfood target; it does not claim cross-platform port observation parity.

The persistent cross-client acceptance target is the long-lived detached HTTP Gateway. The stdio Gateway still owns and cleans its processes, but if its host process exits, those resources are cleaned rather than being promoted into a separate persistent daemon.

## Regression

The pre-integration gate below was executed during review on source committed as `75e919dee81bcce2567dcb6db39e2fffbea6fbfd`. The separate 2026-09-07 independent dogfood remains valid evidence and was not repeated.

- deterministic Phase 6 plan structure — pass, `valid=true`, 3 tasks, zero errors/warnings; checked 2026-09-08, plan unchanged during review
- `go test ./internal/gateway -count=1` — pass in 17.872s
- `go test ./internal/gateway -run ^TestDevProcessLifecycleAcrossRealGatewayRestart$ -count=5` — pass in 5.745s; listening ports asserted from decoded fields
- tail/cross-client/writer-lease focused tests — pass in 0.993s, including `TestProcessStartRenewsWriterLeaseBeforeChildLifetime`
- full Go regression — all 16 packages covered by the separate Gateway command and two bounded package groups: 11 test-bearing packages passed, 5 no-test packages compiled
- `go vet ./...` — pass
- `git diff --check` and fix staged diff check — pass

The exact commands/output are in `evidence/regression.json`. Earlier connector-interrupted aggregate attempts remain historical incomplete executions, not product failures or completed aggregate passes.

## Integration review

Review found and fixed an exact-capacity log truncation flag error and replaced whole-response port substring matching with structured identity/state/port assertions. The log bug failed two targeted cases before the fix; all eight tail cases now pass. The port helper preserves non-Windows lifecycle checks without requiring unavailable Windows port facts; no native non-Windows execution is claimed.

Integration review passed on the current Windows target. The user then explicitly authorized integration, and local `master` fast-forwarded `0ad5488 -> 6dd87d6` on 2026-09-08. Post-integration Gateway tests passed in 15.004s; all 16 packages passed tests or compiled, with vet/diff-check passing. Evidence: `06-INTEGRATION-REVIEW.md` and `evidence/integration.json`.

## Scope review

No Git worktree Environment lifecycle, Agent Run identity/cancel, planner/executor/reviewer orchestration, Desktop feature expansion, installer/package work, migration layer, arbitrary OS process manager, kill-by-PID Agent tool, or LLM subagent was introduced.

No `phase complete 06` or `phase uat-passed 06` transition command was run. Phase 6 is integrated to local master after explicit authorization and post-integration validation. Phase 7 is not started automatically.
