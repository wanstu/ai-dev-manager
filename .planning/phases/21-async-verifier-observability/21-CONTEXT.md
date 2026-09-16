# Phase 21 Context — Async Verifier + Long Operation Observability

Date: 2026-09-12
Planning baseline: `5e3f67a` (`docs: close Phase 20 Agent context acceptance`)
Status: COMPLETE. 21-01 and 21-02 are implemented and accepted; see `21-02-SUMMARY.md` and `21-CLOSEOUT.md`.

## Goal

Make heavy Environment verifier/test/build operations observable and resilient across short MCP/client request lifetimes without turning ADM into CI/task orchestration.

The primary delivery is an owner-local asynchronous verifier-run lifecycle with stable identity, bounded live output, later-client status/cancel, terminal `verifier.Result` retention, and clear blocking-path diagnostics. Existing synchronous verifier execution remains available for short/direct uses.

## Contract mapping

Phase 21 refines these existing contracts and adds one explicit product requirement before feature implementation:

- ADM-GOAL-001 — Agents can run tests/build/lint when available and inspect results.
- ADM-GOAL-002 / ADM-NONGOAL-001 — ADM owns safe execution/lifecycle/diagnostics, not task/CI orchestration.
- ADM-CORE-003 / ADM-CORE-004 / ADM-CORE-008 — verifier remains optional and operation-local.
- ADM-CORE-005 — verifier start/cancel honor the single-writer lease; read-only observation does not require a writer.
- ADM-CORE-007 — verifier execution reuses the executable allowlist and Environment Runtime authority.
- ADM-GW-001 / ADM-GW-003 — stable Environment identity and local failures.
- ARUN-01 — design precedent only: owner-local async lifecycle, no persistence/resurrection. Async verifier is a distinct verifier resource, not a generic `run_` alias.
- New `ADM-CORE-021` — Gateway-owned asynchronous verifier-run lifecycle and blocking-path observability.

No new prerequisite is introduced.

## Source calibration

Current behavior at `5e3f67a`:

1. `app.Service.RunVerifier` is synchronous. It resolves one configured verifier, requires an enabled definition and matching writer, reuses `Service.Runtime`, heartbeats the writer, applies the verifier-defined timeout, calls Runtime execution, then classifies through `verifier.Classify`.
2. `verifier.Result` already carries verifier ID/kind, passed/failed status, exit code, duration, timeout bit, summary and bounded stdout/stderr.
3. The persistent Gateway owner already owns two lifecycle patterns:
   - dev processes (`proc_`) for long-running servers/logs;
   - generic single-command Runs (`run_`) with running/succeeded/failed/canceled, bounded live stdout/stderr, writer heartbeat, cancellation, owner shutdown cleanup, Environment-drop cleanup, and no restart persistence.
4. `runtime.Runtime.PrepareCommand` is the correct seam for long-lived owner resources because it reuses executable allowlist, cwd containment, managed-root validation and OS process-tree cancellation without duplicating command policy.
5. Existing Agent/Admin tool `environment_verifier_run` is blocking. `environment_verifier_list` lists definitions, so async run-list naming must not collide with it.
6. Phase 20 context guidance already describes verifier execution and can be refined to recommend async lifecycle for long/heavy verification without auto-starting anything.

## Locked design decisions

### 1. Async verifier is a distinct owner-local resource

Use a stable verifier-run identity with prefix `vfrun_` (exact helper implementation may use the existing identity package). It is not persisted to `state.json`, not represented as a generic `run_`, and is not a new verifier definition.

Lifecycle states are:

- `running`
- `succeeded`
- `failed`
- `canceled`

Terminal verifier semantics remain in the retained `verifier.Result`:

- passed verifier => lifecycle `succeeded` + `result.status=passed`;
- non-zero verifier / verifier timeout => lifecycle `failed` + classified `verifier.Result`;
- explicit cancel, writer-heartbeat failure, Environment drop or owner shutdown => lifecycle `canceled` (with structured error kind/message when applicable).

No automatic retry or replay occurs.

### 2. One verifier authority path, not two execution policies

Before async execution is added, refactor the application verifier path only as needed to expose a narrow reusable preparation/authority seam. Both synchronous and owner-local async execution must resolve the same definition snapshot and reuse:

- Environment existence and managed-root validation;
- verifier enabled state;
- matching writer requirement for start;
- executable allowlist;
- Environment-contained cwd;
- configured verifier timeout;
- `verifier.Classify` terminal semantics;
- exec-denial observation rules.

Do not duplicate verifier validation in Gateway-specific ad-hoc code.

### 3. Bounded live output

While a verifier run is `running`, status exposes bounded stdout/stderr snapshots plus explicit truncation flags. Terminal status retains the final classified `verifier.Result` and the same bounded output evidence.

Reuse/extract the existing owner-local bounded output buffer pattern used by generic Runs rather than maintaining two subtly different buffering policies. `max_output_bytes` remains caller-bounded with the existing default behavior; Phase 21 does not add unbounded streaming or persistent logs.

### 4. Client disconnect does not own the verifier

Once successfully started, a verifier run is owned by the persistent Gateway owner. Closing the launching MCP client does not cancel it. A later client connected to the same owner can list/status/cancel it by stable IDs.

Gateway-owner shutdown and `DropEnvironment` cancel active verifier runs and wait boundedly for cleanup. A restarted owner starts with no prior verifier-run identities; no observation/result is inferred or restored from persisted state.

### 5. Writer semantics

- async start requires the current matching writer;
- active verifier execution heartbeats that writer;
- list/status are read-only and require only a valid Environment;
- cancel requires the current matching writer and the writer owner that launched that verifier run;
- writer heartbeat failure cancels the run and is visible as `writer_heartbeat_failed`;
- terminal result observation does not renew the writer.

Phase 21 does not change the global single-writer lease model.

### 6. Gateway surface

Preferred Agent/Admin tool names:

- `environment_verifier_run_start`
- `environment_verifier_run_list`
- `environment_verifier_run_status`
- `environment_verifier_run_cancel`

Existing `environment_verifier_list` continues to list definitions. Existing `environment_verifier_run` remains the synchronous compatibility path.

Agent and Admin receive the same read/execute verifier lifecycle tools; no new Admin-only mutation is introduced.

### 7. Sync-path observability without arbitrary thresholds

Do not invent a duration threshold that rejects a valid blocking verifier just because its configured timeout is large.

Instead:

- keep successful synchronous behavior unchanged;
- make the synchronous tool description explicitly recommend the async lifecycle for long/heavy verification;
- distinguish caller/request cancellation from the verifier's own configured timeout where the call can still return a diagnostic;
- return a stable blocking-interruption diagnostic that points to `environment_verifier_run_start` rather than surfacing an opaque process-killed/context error;
- never auto-convert a sync call into an async run and never auto-retry it.

### 8. Phase 20 context guidance follows the new capability

`environment_context_bundle` may update its factual verifier guidance to prefer async start/status/cancel for long verification while retaining the existing verifier definition availability facts. Context generation remains passive and does not create verifier runs.

## Explicit non-goals

1. No CI pipeline/job graph, test matrix, retry policy, dependency graph, Planner/Executor/Reviewer flow or GSD phase semantics.
2. No replacement/removal of generic `run_`; generic Runs remain task-semantic-neutral single commands.
3. No automatic retry/replay after failure, timeout or client disconnect.
4. No persisted verifier-run observations/results and no restart resume/reconciliation.
5. No raw PID/process control; only ADM-owned verifier-run identities are exposed.
6. No new Git/worktree/verifier prerequisite for ordinary Environment/file development.
7. No Desktop feature-page expansion in Phase 21. Existing synchronous Desktop verifier behavior may remain; future surface work can consume the shared Agent/Admin lifecycle without creating Desktop-only state.
8. No new normal CLI verifier lifecycle in Phase 21; Phase 23 owns broader CLI Agent UX unless an acceptance blocker requires a thin existing-Admin-MCP wrapper.
9. No push/tag/release and no subagents.

## Delivery sequence

### 21-01 — Owner-local async verifier lifecycle

Create the reusable verifier preparation seam plus `vfrun_` owner lifecycle, bounded live output, writer heartbeat, cancellation and cleanup. Prove lifecycle/authority/no-persistence behavior in focused application/Gateway-owner tests. Do not expose MCP tools yet.

### 21-02 — Agent/Admin surface, sync diagnostics and integrated acceptance

Expose the four async verifier tools, improve the blocking verifier diagnostic/description, update Phase-20 context guidance, prove real client-disconnect/later-client observation/cancel and owner restart behavior, then run full regression/build acceptance and close Phase 21.

Do not start Phase 22 automatically after closeout.
