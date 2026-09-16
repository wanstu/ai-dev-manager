# Phase 8 Integration Review — Agent Run Lifecycle

**Result:** passed; local master merge pending
**Date:** 2026-09-08
**Reviewed implementation:** `a74b318f883e65a2ff8eb62fc043271bb66c2277`
**Reviewed verification state:** `55dbb54`

## Review conclusion

No blocking correctness, authority, lifecycle, persistence, or scope finding remains in the Phase 8 branch.

The implementation adds one owner-local `runs` resource map to the already-established persistent Gateway owner. Run execution is not a new shell path: start first requires the active Environment writer, resolves `app.Service.Runtime`, validates the command through the Runtime authority, then executes through `Runtime.Exec` under an owner-derived cancelable context. The same Runtime therefore remains responsible for executable allowlist, cwd containment, managed-worktree root validation, bounded output, timeout and OS process-tree cancellation.

The first Run payload is exactly one asynchronous command. The branch contains no Planner/Executor/Reviewer workflow model, GSD executor, parallel lane scheduler, automatic worktree assignment, automatic Git integration, Desktop Run surface, or persisted Run state.

## Lifecycle review

- `run_start` returns stable `run_` identity and installs only after writer/Runtime/command validation succeeds.
- owner memory retains running and terminal Run observation; no Run field is added to persisted model/state.
- `run_list` / `run_status` are scoped by Environment ID and stable Run ID.
- `run_cancel` rechecks the active writer and the Run's writer owner before cancellation.
- writer heartbeat is active only while the Run is running; heartbeat failure requests cancellation.
- exit 0, non-zero exit, timeout, explicit cancellation and owner-derived cancellation have distinct tested outcomes.
- owner Environment drop cancels/forgets affected Runs; owner close cancels and boundedly waits for all active Runs.
- restart creates a fresh owner with an empty Run map; old IDs are invalid rather than inferred/resumed.

## Review gates

- `git diff --check master...HEAD` — pass.
- focused cancel/drop/owner-close acceptance ×5 — pass in 5.716s.
- real Streamable HTTP Gateway shutdown/restart acceptance ×5 — pass in 5.372s.
- final implementation verification already passed focused race detection (`go test -race ...`), full Gateway regression, aggregate `go test ./...`, `go vet ./...`, and working diff-check; see `08-VERIFICATION.md` and `evidence/regression.json`.

## Source/scope audit

`master...HEAD` modifies only Phase 8 planning/evidence, `docs/PRODUCT_CONTRACT.md`, Gateway owner/server wiring, the new Run lifecycle implementation, and Run acceptance tests. There is no Phase 9-11 implementation file or new persisted Run model.

The review also checked a subtle lifecycle boundary: ordinary `environment_remove` remains protected by the existing writer lease and is not changed into a force-removal primitive merely to terminate Runs. The Run contract therefore states owner Environment-drop cleanup (including after successful removal), while active writer semantics remain unchanged.

## Merge boundary

Phase 8 is ready for local fast-forward integration after explicit authorization. This review does not merge `master`, does not push, and does not start Phase 9.
