# Phase 10 Verification — Orchestration Boundary Cleanup

**Status:** passed locally; integration review pending
**Date:** 2026-09-08
**Implementation:** `36202489c13111cba2621b6d5a373707b7fde3f2`
**Requirements:** BOUNDARY-01, BOUNDARY-02, BOUNDARY-03; retained ARUN-01

## Result

Phase 10 passes the boundary cleanup gate. ADM no longer exposes or implements its former Planner/Executor/Reviewer workflow orchestration surface. The retained `run_` lifecycle is again one asynchronous Environment-scoped command with no task/workflow semantics.

The cleanup reused the Phase 8 authority path rather than introducing replacement orchestration. Run start still validates the active writer and existing Runtime before installation; execution still inherits executable allowlist, Environment-relative cwd containment, managed-worktree validation, timeout, bounded output and process-tree cancellation. Run observations remain owner-local and are not persisted or resurrected after restart.

## Acceptance matrix

| ID | Result | Evidence |
|---|---|---|
| P1 workflow tool is gone | pass | `internal/` search finds `run_workflow_start` only in the negative `server_test.go` assertion. `TestGatewayDevelopsPlainDirectoryWithoutGit` proves the official MCP client does not receive a successful result when calling the retired tool. |
| P2 generic asynchronous Run remains intact | pass | focused `TestAgentRunLifecycleAcrossAgentSessions`, terminal/authority, drop, owner-close and writer-heartbeat tests passed; `TestAgentRunLifecycleAcrossRealGatewayRestart` passed. |
| P3 Run status is task-semantic-neutral | pass | `agentRunKind`, workflow fields and workflow status/domain were deleted; `runtime_workflow.go` no longer exists. |
| P4 authority is unchanged | pass | focused generic Run authority suite passed; existing Runtime path still owns allowlist/cwd/managed-worktree/timeout/output enforcement. |
| P5 Core regressions stay green | pass | combined plain non-Git, MCP lifecycle/tool call, Skill gating/read, verifier, dev process and managed worktree acceptance passed in 56.215s. |
| P6 no GSD implementation leaks into Core | pass | production search under `internal/` found no `gsd_phase_*`, no `StartWorkflowRun`, no workflow Run/domain symbol. `STATE.md`/`.planning` matches are test fixture/exclusion strings only, not provenance/state-advance code. |

## Focused verification

- `go test ./internal/gateway -run '^(TestAgentRunLifecycleAcrossAgentSessions|TestAgentRunTerminalStatesAndAuthority|TestAgentRunDropEnvironmentCancelsAndForgets|TestAgentRunOwnerCloseCancelsActiveRun|TestAgentRunStartRenewsWriterLease)$' -count=1` — pass in 2.462s.
- `go test ./internal/gateway -run '^TestAgentRunLifecycleAcrossRealGatewayRestart$' -count=1` — pass in 1.576s.
- `go test ./internal/gateway -run '^(TestGatewayDevelopsPlainDirectoryWithoutGit|TestMCPHealthLifecycleEndToEnd|TestGatewayUsesOnlyEnabledConfiguredExternalMCPAndSkillRuntime|TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy|TestDevProcessLifecycleAcrossRealGatewayRestart|TestGatewayManagedWorktreeLifecycleIsOptionalAndSafe)$' -count=1` — pass in 56.215s.
- after strengthening the retired-tool negative call assertion: `go test ./internal/gateway -run '^TestGatewayDevelopsPlainDirectoryWithoutGit$' -count=1` — pass in 0.106s.

## Final gates

- `go test ./...` — pass; final Gateway package completed in 31.278s and all repository packages passed/compiled.
- `go vet ./...` — pass.
- `git diff --check d52af4...HEAD` — pass.
- source search: `run_workflow_start` only appears in negative test coverage; `StartWorkflowRun`, `workflowRun`, `workflowStep`, `gsd_phase_*` absent from production `internal/` source.

Earlier in the prior session, `pjadm` closed the connector when invoking Go tests and therefore produced no Go pass/fail result. In this session the connector recovered and all required tests returned real exit-code-0 results. The earlier connection closure is not treated as a product failure or as test evidence.

## Scope review

No Phase 11 implementation was started. No legacy HTTP+SSE, reconnect/import work, Skill refresh work or capability-report implementation was introduced. The `feat/gsd-phase-executor` branch was not touched or merged. Master was not modified.

Phase 10 is locally verified and ready for integration review.
