# Phase 9 Research — Planner / Executor / Reviewer Contract

## Existing seams to reuse

### Persistent Run ownership

`internal/gateway/runtime_run.go` already owns `run_` resources in `runtimeOwner.runs`. It provides stable identity, later-client list/status/cancel, writer heartbeat, owner/Environment cleanup, bounded cancellation and no persistence/restart resurrection. Phase 9 should extend this lifecycle rather than introduce a workflow map or second daemon.

### Runtime authority

`app.Service.Runtime` revalidates Environment/worktree identity and builds the existing allowlisted Runtime. `runtime.Runtime.PrepareCommand` validates executable allowlist and cwd containment; `Runtime.Exec` applies bounded output, timeout and OS process-tree cancellation. Workflow steps must reuse these paths.

### Structured verifier semantics

`app.Service.RunVerifier` resolves an Environment verifier, enforces enabled/writer/Runtime policy, heartbeats the writer, runs with the verifier deadline, and returns `verifier.Result` with `status`, exit code, timeout and bounded output. A verifier process exit/non-pass is returned as a structured `Result{Status:"failed"}`; configuration/policy/execution infrastructure problems return Go errors. This gives the exact boundary needed to distinguish review rejection from orchestration failure.

### Gateway surface

`internal/gateway/server.go` already exposes `run_start`, `run_list`, `run_status` and `run_cancel`. A single `run_workflow_start` tool can create workflow-kind Runs while all observation/cancel operations stay unchanged.

## Design alternatives considered

### Separate Workflow resource

Rejected. A new `workflow_` identity/map would duplicate persistent-owner lifecycle, cancel, cleanup and restart semantics and would make "plan/steps/review remain visible in Run status" indirect.

### Caller supplies an opaque shell script

Rejected. It would bypass structured step audit and would encourage a second hidden command policy.

### AI Planner/Executor/Reviewer in Phase 9

Rejected. The project explicitly forbids consuming separate LLM/provider quota, and Phase 9 only requires the orchestration contract. Later GSD/agent execution can plug into the structured contract after it is locally proven.

### Treat verifier failure as Run failure

Rejected. FLOW-01 explicitly requires review failure to be distinguishable from orchestration failure. A verifier that runs correctly and reports failure is a valid review result. Therefore Run lifecycle may complete successfully while workflow outcome is `rejected`.

## Proposed model

- Add `kind` to Run status: `command` or `workflow`.
- Command Runs preserve Phase 8 fields/behavior.
- Workflow status contains:
  - immutable `plan`: goal, ordered normalized steps, verifier IDs;
  - `steps[]`: stable step ID/name, lifecycle, timing, exit/output/error evidence;
  - `review`: pending/running/accepted/rejected/error/not_run plus verifier results;
  - `outcome`: accepted/rejected when orchestration completes.
- Planner assigns deterministic per-Run step IDs such as `step_01`, validates non-empty goal, 1..32 steps and all Runtime command authority before installing the Run. Verifier references are retained for reviewer resolution so reviewer errors remain auditable at the review stage.
- Executor runs one step at a time. Non-zero exit is an executor failure and stops the workflow before review.
- Reviewer runs configured verifier IDs sequentially and records every normal verifier result. Any structured failed result makes the final review rejected; invocation error makes review error and Run failed.

## Concurrency / cleanup notes

- Mutable workflow observations should be guarded by the existing `ownedAgentRun.mu`; no second lock is needed.
- The same owner-derived Run context is used by executor commands. `run_cancel` therefore cancels an in-flight step with the existing Runtime process-tree policy.
- `RunVerifier` receives the Run context so owner/cancel shutdown can also stop an in-flight reviewer command.
- The workflow-level heartbeat remains the existing `heartbeatAgentRun`; nested verifier heartbeat calls are same-owner renewals and do not widen writer authority.

## Verification focus

The most important negative distinction is three-way:

1. normal verifier result failed → orchestration completed, review rejected;
2. verifier invocation/policy error → orchestration failed, reviewer error;
3. executor command failure → orchestration failed before review.

Real Streamable HTTP acceptance should prove at least accepted and rejected workflows through stable `run_` status from a later client.
