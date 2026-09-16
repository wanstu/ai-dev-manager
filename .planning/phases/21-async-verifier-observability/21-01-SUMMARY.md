# 21-01 — Owner-local async verifier lifecycle

Date: 2026-09-12
Planning commit: `2aceec3` (`docs: plan Phase 21 async verifier observability`) on source baseline `5e3f67a`.
Status: complete
Planned commit: `feat(verifier): add owner-local async verifier lifecycle`

## Delivered

- Added shared `app.PreparedVerifierExecution` / `PrepareVerifierExecution` so synchronous verifier execution and owner-local async verifier execution resolve the same verifier definition, enabled state, writer authority, managed-root Runtime, executable allowlist, cwd containment and configured timeout.
- Existing synchronous `Service.RunVerifier` now consumes that shared preparation path while preserving its pass/fail/configured-timeout behavior.
- Extracted the existing generic Agent Run bounded output buffer into an owner-local `ownerOutputBuffer`; generic `run_` behavior remains unchanged.
- Added a distinct Gateway-owner-local verifier-run resource with stable `vfrun_` identity and lifecycle `running|succeeded|failed|canceled`.
- Async verifier runs retain Environment/verifier identity, bounded live stdout/stderr plus truncation flags, terminal classified `verifier.Result`, timestamps and structured cancellation/runtime diagnostics.
- Start validates authority before installing a resource. Matching-writer cancel is enforced; list/status are read-only Environment-scoped owner methods.
- Verifier-defined timeout uses `verifier.Classify`, so timeout remains a failed verifier result with `timed_out=true` rather than caller cancellation.
- Active verifier runs heartbeat the writer. Heartbeat failure cancels the owned command tree with `writer_heartbeat_failed` evidence.
- `runtimeOwner.DropEnvironment` and `runtimeOwner.Close` now cancel/forget verifier runs with bounded cleanup. A fresh owner starts empty and no verifier-run observation/result is persisted.
- No Agent/Admin MCP tools, CLI/Desktop lifecycle surface, retry policy, CI/job semantics, or task orchestration were added in 21-01.

## Acceptance evidence

| Gate | Evidence |
|---|---|
| V01 shared synchronous authority | Existing `internal/app` verifier tests pass through the new `PrepareVerifierExecution` seam, including pass/fail/timeout/disabled/missing/writer/allowlist/cwd behavior. |
| V02 success lifecycle | `TestVerifierRunTerminalStatesAndLiveOutput` verifies `vfrun_` starts running then reaches `succeeded` with `result.status=passed` and retained stdout. |
| V03 failed verifier | Same test verifies non-zero exit reaches lifecycle `failed` with classified failed result, exit code and stderr. |
| V04 configured timeout | `TestVerifierRunConfiguredTimeoutUsesVerifierClassification` verifies lifecycle `failed`, `result.timed_out=true`, `verifier timed out` summary and timeout evidence. |
| V05 cancellation authority | `TestVerifierRunCancelAndWriterHeartbeatFailure` rejects a wrong writer without changing the running resource and reaches `canceled` for the matching writer. |
| V06 bounded live output | `TestVerifierRunTerminalStatesAndLiveOutput` observes live stdout/stderr capped at 64 bytes with both truncation flags while the verifier is still running. |
| V07 writer heartbeat | `TestVerifierRunCancelAndWriterHeartbeatFailure` uses a short owner test heartbeat interval, releases the writer, and verifies automatic cancellation with `writer_heartbeat_failed`. |
| V08 owner/Environment cleanup and no persistence | `TestVerifierRunOwnerAndEnvironmentCleanupAndNoPersistence` verifies Environment drop forgets/cancels, owner close cleans active work, persisted state contains no `vfrun_`/`verifier_runs`, and a fresh owner rejects old IDs. |
| Generic Run regression | Existing `TestAgentRun*` tests pass after extracting the shared owner output buffer. |

## Validation

- `go test -count=1 ./internal/app -run Verifier` — PASS (1.929s).
- `go test -count=1 ./internal/gateway -run "VerifierRun|AgentRun"` — PASS (7.424s).
- `go test -count=3 ./internal/gateway -run "^TestVerifierRun"` — PASS (10.929s).
- Focused combined owner/app gate — PASS, `run_b9a69b74dd3c3d07`; `internal/app` 2.242s, `internal/gateway` 94.128s.
- Full repository — PASS, `run_b59177d17a9620a5`; Gateway 112.957s.
- Vet — PASS, `run_8666a6eb51de25d9`.
- `git diff --check` — PASS before closeout edits (line-ending warnings only).

## Boundary / next step

21-01 intentionally exposes no new MCP tool. Existing `environment_verifier_run` remains the only Agent/Admin verifier execution surface until 21-02. Generic `run_` remains separate and task-semantic-neutral. Verifier-run observations are owner-local only and are neither persisted nor resumed. 21-02 is next: expose the four Agent/Admin async verifier tools, improve genuine blocking-request interruption diagnostics, update Phase-20 verifier guidance, and run integrated disconnect/restart acceptance. Do not start Phase 22 automatically. No push/tag/release or subagents were used.
