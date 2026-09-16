# Phase 9 Integration Review — Planner / Executor / Reviewer Contract

Status: passed after two review fixes. Local master merge pending.

Reviewed branch: `feat/planner-executor-reviewer-contract`

Base: local `master@4520baf`

Reviewed commit chain:

- `9eb5c42` — `docs(09): plan planner executor reviewer contract`
- `b73bb59` — `feat(flow): add auditable workflow runs`
- `6a43f93` — `docs(09): record local verification`
- `a627df9` — `fix(flow): revalidate runtime authority per step`
- `50fe9ae` — `fix(flow): recheck writer before each step`

Final reviewed source: `50fe9ae`.

## Review scope

The review checked FLOW-01 against the integrated Phase 5-8 ownership/runtime contracts:

- one persistent Gateway owner and one `run_` lifecycle;
- immutable/visible plan and ordered step evidence;
- Runtime/allowlist/cwd/managed-worktree authority;
- single-writer authority through the whole workflow lifetime;
- verifier-backed review and rejection semantics;
- cancel/heartbeat concurrency;
- no Phase 10 GSD state mutation or Phase 11 parallel orchestration.

## Finding IR-09-01 — Runtime authority was only snapshotted at workflow start

Severity: blocking before integration.

Initial implementation `b73bb59` materialized and preflighted the plan with one Runtime and then reused that same Runtime for later asynchronous executor steps.

That created two contract violations if authority changed while a prior step was running:

1. removing an executable from the global allowlist could fail to affect a later step because the old Runtime retained the previous allowlist snapshot;
2. a managed-worktree identity change between steps would not be revalidated before the later routed mutation.

Fix: `a627df9` resolves a fresh app Runtime immediately before every executor step. This refreshes the allowlist snapshot and re-enters the Phase 7 managed-worktree validation boundary before each step executes.

Regression: `TestWorkflowRevalidatesRuntimeAuthorityBeforeEachStep` holds step 1, removes the executable from the allowlist, releases step 1, and proves step 2 fails locally before command execution. The targeted test passed repeatedly.

## Finding IR-09-02 — Writer authority was only guaranteed at workflow start/heartbeat cadence

Severity: blocking before integration.

The initial workflow checked the writer at start and maintained a heartbeat, but a forced writer release/takeover between heartbeat ticks could otherwise allow the next executor step to begin under stale authority.

Fix: `50fe9ae` calls `RequireWriter` immediately before every executor step, before Runtime resolution or command execution.

Regression: `TestWorkflowRevalidatesWriterAuthorityBeforeEachStep` holds step 1, force-releases the original writer, acquires a replacement writer, releases step 1, and proves step 2 fails as `executor_error` with review `not_run`.

## Semantic review

Passed.

- verifier pass -> Run `succeeded`, workflow/review `accepted`;
- verifier normal failure -> Run `succeeded`, workflow/review `rejected`, no orchestration error;
- executor non-zero -> Run `failed`, `executor_step_failed`, review `not_run`;
- executor authority/runtime error -> Run `failed`, `executor_error`, review `not_run`;
- reviewer invocation/configuration error -> Run `failed`, `reviewer_error`;
- cancel remains writer-gated and uses the Phase 8 Run owner/context cleanup path.

The review found no second workflow persistence model, hidden executor, LLM subagent path, GSD state advancement, parallel lane orchestration, or automatic Git integration.

## Final gates after review fixes

All passed:

- all `^TestWorkflow` tests ×5: `6.258s`;
- focused workflow race detector: `11.332s`;
- fresh full Gateway suite: `37.170s`;
- final `go test ./...`: passed; Gateway package `43.275s` in that run;
- `go vet ./...`: passed;
- `git diff --check`: passed;
- committed-source `^TestWorkflow` ×3 after both fix commits: `3.383s`;
- `git diff --check master...HEAD`: passed.

An earlier broad race invocation hit the tool execution time limit without test-failure output; the review replaced it with the focused workflow race gate above, which passed.

## Decision

Phase 9 integration review passes on `50fe9ae` with both blocking findings resolved and regression-covered.

The branch is ready for local master integration. Do not push, merge automatically, or start Phase 10 solely from this review artifact; local master integration remains a separate project transition.
