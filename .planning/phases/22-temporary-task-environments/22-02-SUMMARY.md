# 22-02 Summary — Agent/Admin temporary Environment workflow

Date: 2026-09-13
Implementation baseline: `0f6efa7` (`feat(environment): add targeted temporary lifecycle`) from planning commit `84cc9c7`.
Implementation commit: `d51a426` (`feat(gateway): expose temporary environment lifecycle`).
Status: complete and validated; 22-03 ready, not started.

## Delivered

- Added four shared Agent/Admin MCP tools over the accepted 22-01 Core lifecycle:
  - `environment_temporary_create`
  - `environment_temporary_status`
  - `environment_temporary_promote`
  - `environment_temporary_cleanup`
- Kept generic durable `environment_create` Admin-only and kept generic `resource_retention_*` as Admin management/recovery surfaces.
- `environment_temporary_create` accepts Workspace/name/lifecycle owner/positive TTL plus optional session/run provenance and the locked `existing_root` / `managed_worktree` modes. The trusted creator surface comes from the actual Agent/Admin Gateway surface and is not caller-controlled.
- Tool descriptions explicitly state that owner/session/run IDs are lifecycle provenance only, not ADM task orchestration. A `run_id` does not need to exist and temporary creation does not start, parent, cancel or otherwise manage a Run.
- `environment_temporary_status` and cleanup preview are read-only. Real HTTP acceptance proves they do not acquire or renew an existing writer lease.
- Promotion requires the matching lifecycle owner and preserves stable Environment ID/root/private Memory while changing retention only.
- Cleanup preview/execute targets exactly one Environment; execute requires matching lifecycle owner plus fresh runtime/Git safety. No force input exists on either Agent or Admin surface.
- Real HTTP cleanup exercises existing-root state-only cleanup, managed-worktree non-force cleanup with retained branch, dirty/unpublished/tamper refusal, and active writer/process/generic Run/async verifier Run blockers.
- No Desktop UI, CLI lifecycle UX, automatic GC/scheduler, task model/orchestration, merge/rebase/push, branch deletion or second persistence model was added.

## Acceptance A01–A10

| Gate | Result | HTTP evidence |
|---|---|---|
| A01 | PASS | Agent and Admin both list all four temporary lifecycle tools; Agent still does not list generic durable `environment_create`, while Admin does. Cleanup schema contains no force input. |
| A02 | PASS | Agent creates a plain non-Git temporary Environment, acquires/releases writer and performs ordinary write/read. Temporary retention survives a real Gateway owner/server restart while a real pre-restart owner-local `run_` ID is absent from `state.json` and from the fresh owner list. |
| A03 | PASS | Two TTL-expired temporary Environments are created through HTTP. Preview of one is non-mutating and mentions only that Environment; execute removes only the selected Environment and leaves the other eligible Environment plus ordinary project directories untouched. |
| A04 | PASS | Wrong lifecycle owner cannot promote or execute cleanup. Matching owner promotes to durable without changing stable Environment/root/private Memory; later temporary cleanup refuses the durable Environment. |
| A05 | PASS | Managed-worktree temporary create/preview/execute succeeds on a Git Workspace without changing source branch/HEAD; cleanup removes the ADM-owned worktree and retains its branch. Managed mode fails locally on a non-Git Workspace while `existing_root` still succeeds. |
| A06 | PASS | Dirty, unpublished and branch-tampered managed worktrees expose their existing safety blockers and reject cleanup execute; temporary cleanup exposes no force parameter. |
| A07 | PASS | Real HTTP lifecycle proves active writer, real dev process, real generic `run_`, and a real Phase-21 `vfrun_` each block cleanup. Matching release/stop/cancel removes the corresponding active blocker, and cleanup succeeds only after all active resources are terminal/released. |
| A08 | PASS | Plain temporary create/status succeeds in a non-Git Workspace with no verifier/process/Run/MCP/Skill prerequisite. Optional Git failure is operation-local to managed-worktree mode. |
| A09 | PASS | Arbitrary nonexistent `run_nonexistent_provenance` persists/returns only as retention provenance; no Run is created or required. |
| A10 | PASS | Status exposes lifecycle owner/session/run metadata but never the Environment-private Memory sentinel. Promotion preserves private Memory in Environment scope and does not create a Global Memory copy. |

## Validation

- Real Streamable HTTP temporary lifecycle suite: `go test -count=1 ./internal/gateway -run "^TestTemporaryEnvironment.*HTTP"` PASS after the final read-only writer assertion (`internal/gateway` 16.740s).
- HTTP timing/stability repeat before the final assertion-only addition: `go test -count=3 ./internal/gateway -run "^TestTemporaryEnvironment.*HTTP"` PASS (`internal/gateway` 47.314s).
- Final focused cross-phase regression on the frozen tree: `run_93a79538fd11dc6e` PASS.
  - `internal/app` 1.529s
  - `internal/gateway` 33.194s
  - filter: `Temporary|ResourceRetention|VerifierRun|AgentRun|DevProcess`
- Final full repository on the frozen tree: `run_5356c300b0312025` PASS; Gateway 170.068s.
- Final `go vet ./...`: `run_bb24c986567e97a6` PASS.
- `git diff --check` PASS; only Windows LF→CRLF working-copy warnings were emitted.
- One earlier pre-final full run, `run_3af5fcad46e2d758`, was canceled by its outer 180000ms Agent Run timeout after all emitted packages had passed. It is not acceptance evidence; the unchanged/final tree was rerun with a 300000ms wrapper and passed as `run_5356c300b0312025`.

## Next

22-03 is ready for bounded Desktop retention visibility and explicit Admin-MCP promote/cleanup UX plus integrated fixed-head closeout. Desktop work was not started in this node.

No push, tag, release or subagents were used.
