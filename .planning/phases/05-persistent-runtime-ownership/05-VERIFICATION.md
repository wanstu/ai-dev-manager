# Phase 5 Verification — Persistent Runtime Ownership

**Status:** passed locally
**Date:** 2026-09-07
**Requirements:** LIFE-01, LIFE-02, LIFE-03 plus operation-local/core Gateway constraints named by 05-01.

## Result

Phase 5 establishes the existing long-lived HTTP/stdio Gateway process as the single in-process owner for live external MCP sessions. Persisted Environment/catalog configuration remains desired state; owner identity, SDK sessions and observed health remain process-instance state only. Restart creates a fresh owner and rebuilds from persisted desired selections. Current-version HTTP stop is owner-bound and graceful so owned resources are closed before process exit; older/no-owner Gateway responses retain the existing safe kill fallback.

## Acceptance

| ID | Result | Evidence |
|---|---|---|
| L1 one stable owner per Gateway | pass | `runtime_owner_test.go` independent-client test; `/healthz` lifecycle test; independent dogfood first owner `owner_33248_18d3019897036610_1` observed by two client probes |
| L2 Gateway-owned MCP session reuse | pass | deterministic owner seam proves exactly one connect for repeated status/tools/call and two independent Agent sessions; real Streamable HTTP dogfood calls `phase5_ping` through the owned path before and after restart |
| L3 desired and observed state separated | pass | independent private `ADM_V2_HOME` state retained Environment/MCP selection while containing no owner ID / `owner_id` / `owned_mcp_sessions`; `evidence/independent-dogfood.json` |
| L4 restart reconciles and stale healthy is impossible | pass | restart changed owner to `owner_44588_18d30198b6014014_1`; post-restart call passed; dead upstream returned `error_kind=connection_refused` with `owned_mcp_sessions=0`; cross-process acceptance passed 5 consecutive runs |
| L5 deterministic cleanup | pass | fake-session tests cover disable and idempotent owner close; Gateway disable/remove hooks evict owned sessions; owner-bound `/shutdown` tests reject wrong owner and close the owned session; independent CLI stop exited 0 and released the endpoint |
| L6 MCP failure remains local / no secret leakage regression | pass | existing Phase 3 health/error tests remain green; owner reuses canonical app activation resolution and never persists resolved endpoint/header secrets |
| L7 unrelated development remains optional-capability based | pass | full `go test ./...` remains green including non-Git/core Gateway tests; no new Environment/Git/MCP prerequisite was added |
| L8 no Phase 6 leakage | pass | added-line diff search returned `NO_PHASE6_API_ADDITIONS`; no dev-process/list/log/port API exists in this phase |

## Real independent dogfood

Current source built `.tmp/phase5-dogfood/ai-dev-manager-v2.exe`, a real local Streamable HTTP fixture and an independent MCP client probe. A private `ADM_V2_HOME` and loopback ports `43761` (ADM) / `43762` (fixture) were used; shared Gateway `41137` was not touched.

The CLI itself created Workspace `ws_9a3361532b8cbad3`, Environment `env_7cc45f1dc68a0de6`, MCP `mcp_6b9d6245bf79a365`, enabled the selection, detached the Gateway, stopped it, restarted it, and stopped it again. Two independent probes on the first Gateway both passed `healthy` + `phase5-pong`; the post-restart probe passed with a different owner/PID. With the upstream fixture stopped, status became `connection_refused` and the owner reported zero owned MCP sessions.

The upstream fixture is intentionally Streamable HTTP stateless, matching the already validated Phase 3 transport. With the current MCP Go SDK this mode does not expose a stable upstream session identifier or separate `initialize` RPC suitable for black-box connect counting, so exact one-connect reuse is asserted at the deterministic owner seam while the independent process test proves that real requests route through the same observable Gateway owner and survive restart reconciliation.

## Regression

- `go test ./... -count=1` — pass; `internal/gateway` 14.921s.
- `go vet ./...` — pass.
- `git diff --check` — pass (line-ending warnings only where applicable).
- `go test ./internal/gateway -run TestRuntimeOwnerRealHTTPRestartReconciliation -count=5` — pass in 2.177s.
- Focused current-owner graceful stop and legacy fallback tests — pass.
- Initial owner-identity red test is retained in `evidence/red-green.json`; the fixed test is green.

## Scope review

No generic process manager, dev-server lifecycle, logs, ports, worktree isolation, Agent Run, orchestration, Desktop feature, installer, migration layer, or LLM subagent was introduced. Phase 6 remains not started.

No `phase complete 05` or `phase uat-passed 05` transition command was run; local verification stops for integration/review.
