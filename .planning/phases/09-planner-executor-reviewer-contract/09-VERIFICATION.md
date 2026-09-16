# Phase 9 Verification — Planner / Executor / Reviewer Contract

Status: passed locally; integration review passed after authority revalidation fixes. Local master merge pending.

Implementation under verification:

- baseline: `b73bb5905c36076bad11305a775498fbdf7f62f7` — `feat(flow): add auditable workflow runs`
- review fix: `a627df9` — revalidate fresh Runtime authority before every executor step
- review fix: `50fe9ae` — recheck matching writer before every executor step
- final reviewed source: `50fe9ae`

## Requirement mapping

### FLOW-01 — structured auditable Planner / Executor / Reviewer

Passed.

The Phase 9 workflow uses the existing persistent Gateway-owned `run_` lifecycle instead of adding another owner/persistence model.

- Planner materializes one explicit immutable workflow plan before Run installation.
- Planned executor steps receive stable `step_01...` identities and remain visible in `run_status`.
- Executor runs steps sequentially through the existing Runtime command authority and records structured step state/result evidence.
- Reviewer invokes an existing Environment verifier through `app.Service.RunVerifier` and retains the structured verifier result.
- Workflow audit data is owner-local and available through the ordinary Run status/list/cancel surface.

### Review rejection is distinct from orchestration/runtime failure

Passed.

The tests prove three materially different terminal outcomes:

1. Executor succeeds and verifier passes:
   - Run lifecycle: `succeeded`
   - workflow outcome: `accepted`
   - review state: `accepted`
2. Executor succeeds and verifier executes normally but reports a failed verification:
   - Run lifecycle: `succeeded`
   - workflow outcome: `rejected`
   - review state: `rejected`
   - no orchestration error kind
3. Executor or reviewer infrastructure fails:
   - Run lifecycle: `failed`
   - executor failure uses `executor_step_failed`
   - reviewer invocation/configuration failure uses `reviewer_error`

This prevents a normal negative review decision from being collapsed into an execution failure.

## Authority and safety

Passed.

- Workflow start requires the active Environment writer.
- Every executor command is preflighted through the existing Runtime executable allowlist and cwd containment before the workflow Run is installed.
- Failed preflight does not create a partially installed `run_` observation.
- Integration review additionally requires a fresh matching-writer check and fresh app Runtime resolution immediately before every executor step, so mid-workflow writer takeover, allowlist revocation, or managed-worktree tamper cannot be bypassed by a start-time snapshot.
- Execution otherwise reuses the owner-derived Run context, bounded output, timeouts, writer heartbeat and OS process-tree cancellation already established by Phase 8/runtime.
- Reviewer reuses existing verifier definitions and writer-gated verifier execution rather than introducing hidden shell execution.
- `run_cancel` remains the cancellation boundary for workflow Runs.

## Scope audit

Passed.

Phase 9 does not implement:

- LLM Planner/Executor/Reviewer agents or subagents;
- GSD `.planning` execution/state advancement;
- parallel workflow steps or worktree orchestration;
- automatic Git merge/rebase/push;
- persisted workflow observation/state separate from the persistent Gateway owner.

Those remain Phase 10/11 or later concerns.

## Automated evidence

Focused deterministic workflow tests cover:

- `TestWorkflowRunAcceptedPlanExecutorReviewerAudit`
- `TestWorkflowRunReviewRejectionIsNotOrchestrationFailure`
- `TestWorkflowRunExecutorAndReviewerFailuresAreDistinct`
- `TestWorkflowPlannerAuthorityFailureDoesNotInstallRun`
- `TestWorkflowRunCancellationReusesAgentRunBoundary`
- `TestWorkflowRunBoundsExecutorOutput`

All passed.

Concurrency gate:

- focused Gateway race detector covering workflow/Run code: passed (`12.293s` recorded during implementation validation).

Real Agent-facing transport gate:

- `TestWorkflowLifecycleThroughRealStreamableHTTP`: passed.
- fresh repeated run with `-count=5`: passed in `0.914s`.
- Acceptance proves accepted and rejected workflow audit through real Streamable HTTP and a later client, while workflow observation remains absent from persisted state.

Regression gates:

- full Gateway suite: passed during implementation (`34.299s`) and again fresh after implementation commit (`33.774s`).
- `go test ./...`: passed after implementation commit; all test-bearing packages passed and no-test packages compiled.
- `go vet ./...`: passed.
- `git diff --check`: passed.

## Negative acceptance

Passed.

- forbidden executor command/cwd policy failure is local and installs no Run;
- ordinary review rejection is not `failed`;
- executor failure and reviewer error remain distinguishable;
- cancellation uses existing writer-gated Run semantics;
- no verifier/GSD/parallel capability becomes a prerequisite for unrelated command Run or ordinary Environment development.

## Integration review follow-up

The independent review found and fixed two blocking dynamic-authority gaps that were not covered by the initial local-verification snapshot:

- `TestWorkflowRevalidatesRuntimeAuthorityBeforeEachStep` proves allowlist revocation between steps is honored; the fresh Runtime resolution also re-enters managed-worktree validation for each step.
- `TestWorkflowRevalidatesWriterAuthorityBeforeEachStep` proves forced writer takeover between steps prevents the next executor command from starting.

After both fixes, all workflow tests ×5 passed in `6.258s`, the focused race gate passed in `11.332s`, fresh Gateway passed in `37.170s`, full `go test ./...` passed (Gateway `43.275s`), vet/diff-check passed, and committed-source workflow tests ×3 passed in `3.383s`.

See `09-INTEGRATION-REVIEW.md` for the findings and final review decision.

## Conclusion

FLOW-01 is locally verified and integration-reviewed on final source `50fe9ae`. The implementation provides one deterministic, inspectable workflow vertical slice over the validated Run/Runtime/Verifier foundations without introducing Phase 10 or Phase 11 behavior.

Phase 9 is ready for local master integration. Do not push or start Phase 10 solely from this verification result.
