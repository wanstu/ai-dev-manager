# Phase 9 Context — Planner / Executor / Reviewer Contract

## Goal

Implement FLOW-01 as one deterministic, auditable workflow carried by the existing persistent-owner Agent Run lifecycle.

Phase 9 is not an AI-agent framework. It establishes structured contracts that later Phase 10 GSD execution can consume without inventing another lifecycle or hidden execution path.

## Locked decisions

- A workflow is another `run_` kind owned by the existing persistent Gateway owner. `run_list`, `run_status`, `run_cancel`, owner shutdown, Environment drop, writer heartbeat and restart non-resurrection remain the lifecycle boundary.
- Add one Agent-facing start operation: `run_workflow_start`.
- Planner contract is deterministic plan materialization from the explicit request. It validates/normalizes goal, ordered steps and verifier references, assigns stable step IDs, and freezes the plan before the Run is installed. It does not invoke an LLM or invent steps.
- Executor runs planned steps sequentially through the existing Environment Runtime. Each step is one allowlisted command with Environment-relative cwd and bounded output/timeout. No arbitrary shell, subprocess API or alternate authority is introduced.
- Reviewer consumes existing Environment verifier definitions through `app.Service.RunVerifier`. A verifier returning structured `status=failed` is a review rejection, not an orchestration/runtime error.
- Run lifecycle state keeps its Phase 8 meaning. If planning/execution/reviewer infrastructure completes, the Run reaches `succeeded`; workflow `outcome` distinguishes `accepted` vs `rejected`. Runtime/orchestration failures instead produce Run `failed` plus structured `error_kind`. Cancellation remains `canceled`.
- Executor command non-zero is an execution failure (`executor_step_failed`) and stops before review. Reviewer invocation/policy errors are orchestration failures (`reviewer_error`). A normal verifier failure is `review.status=rejected` and `workflow.outcome=rejected`, not `run.state=failed`.
- `run_status` must expose the immutable plan, every step status/result, review/verifier results and final workflow outcome so later clients can audit what happened.
- Workflow observations are owner-local exactly like Phase 8 Runs and are never persisted to `state.json`.

## Explicit non-goals

- no LLM Planner, Executor or Reviewer subagent
- no OpenCode/GSD provider/model calls
- no GSD `.planning` phase execution or STATE advancement (Phase 10)
- no parallel steps or parallel Runs/worktrees (Phase 11)
- no automatic Git merge/rebase/push/integration decision
- no new persistence model for Run observations
- no generic shell script DSL

## Acceptance

1. A real workflow starts through the Agent Gateway, immediately returns a stable `run_` ID, and a later client can inspect its plan/steps/review.
2. A successful step plus passing verifier yields Run `succeeded`, workflow `outcome=accepted`, visible step result and verifier result.
3. A verifier that executes normally but fails yields Run `succeeded`, workflow `outcome=rejected`, review `rejected`; it is not reported as an orchestration error.
4. A non-zero executor step yields Run `failed` with `executor_step_failed`, failed step evidence, and review not run.
5. A reviewer invocation/policy error yields Run `failed` with `reviewer_error`, distinct from normal review rejection.
6. Forbidden executable/escaped cwd is rejected before a workflow Run is installed.
7. Matching-writer cancellation and owner cleanup reuse Phase 8 behavior.
8. Existing command Runs, verifier tools, process lifecycle, non-Git file development and optional Git behavior remain green.
