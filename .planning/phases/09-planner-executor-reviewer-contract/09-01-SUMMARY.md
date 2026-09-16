# Phase 9 Plan 09-01 Summary

## Delivered

FLOW-01 is implemented as a deterministic workflow kind on the existing persistent Gateway-owned Agent Run lifecycle.

- Added `run_workflow_start`; workflow observation/cancel continues through ordinary `run_list`, `run_status`, and `run_cancel` using stable `run_` identity.
- Planner materializes an explicit immutable goal/ordered step/verifier plan before Run installation, assigns stable `step_01...` IDs, and preflights every executor command through existing Runtime allowlist/cwd authority.
- Executor runs planned commands sequentially on the owner-derived Run context and records per-step timing, state, bounded stdout/stderr, exit code, and structured error evidence.
- Reviewer calls existing `app.Service.RunVerifier` definitions and retains structured verifier results.
- Normal verifier failure is a review rejection, not an orchestration failure: Run lifecycle reaches `succeeded`, review/outcome are `rejected`, and no orchestration error kind is set.
- Executor non-zero and reviewer invocation errors are separately classified as `executor_step_failed` and `reviewer_error` with visible audit state.
- Workflow cancellation reuses Phase 8 writer-gated Run cancellation and owner process-tree cleanup.
- Workflow observations remain owner-local and are absent from `state.json`.

## Scope deliberately not implemented

- no LLM Planner/Executor/Reviewer subagents
- no GSD phase execution or planning-state advancement
- no parallel workflow steps / parallel worktrees
- no automatic Git merge/rebase/push
- no second Run/workflow persistence model or hidden shell authority

## Implementation commits

- `b73bb5905c36076bad11305a775498fbdf7f62f7` — `feat(flow): add auditable workflow runs`
- `a627df9` — integration-review fix: fresh Runtime/managed-root/allowlist revalidation before every executor step
- `50fe9ae` — integration-review fix: matching writer recheck before every executor step

Final reviewed source: `50fe9ae`.

## Verification snapshot

- focused workflow acceptance: passed
- workflow race detector: passed
- real Streamable HTTP accepted/rejected workflow ×5: passed
- fresh full Gateway suite: passed
- `go test ./...`: passed
- `go vet ./...`: passed
- `git diff --check`: passed

See `09-VERIFICATION.md`, `09-UAT.md`, and `evidence/regression.json`.
