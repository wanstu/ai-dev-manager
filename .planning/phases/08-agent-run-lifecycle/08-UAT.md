# Phase 8 UAT — Agent Run Lifecycle

**Result:** passed by deterministic automated and real Streamable HTTP evidence on 2026-09-08.

No human-only acceptance item remains for the Phase 8 slice.

## Covered behavior

1. **Asynchronous stable identity** — `run_start` returns a stable `run_` while a real allowlisted helper remains running.
2. **Client independence** — the launching MCP client disconnects; a later client connected to the same Gateway owner lists and inspects the same Run.
3. **Cancellation authority** — wrong writer is rejected; the matching writer cancels the Run and its real helper process tree/port is cleaned up.
4. **Terminal semantics** — exit 0 becomes `succeeded`; non-zero exit becomes `failed` with exit code/result; timeout is a distinct failed error kind; explicit owner cancellation becomes `canceled`.
5. **Bounded output** — a helper emitting 4096 bytes is retained at exactly the configured 64-byte output bound.
6. **Runtime authority reuse** — forbidden executable and escaped cwd fail before a Run is installed; Run start routes through the existing app Runtime rather than a new shell policy.
7. **Owner cleanup** — owner Environment drop and owner/Gateway close cancel active Runs and wait boundedly for process-tree cleanup.
8. **Restart negative acceptance** — a second Run is left active when the real HTTP Gateway is gracefully stopped; its port closes, the restarted Gateway has a different owner, `run_list` is empty, and the old `run_` ID is invalid.
9. **Persistence boundary** — `state.json` contains no Run identity/result/owner-count observation.
10. **Regression/locality** — ordinary Environment `read` remains available after restart; full Gateway and repository regression, vet, diff-check and focused race detection pass.

## Scope boundary

This UAT validates only the single-command persistent-owner Run lifecycle required by ARUN-01. It does not validate or imply Planner/Executor/Reviewer workflow, GSD execution, parallel Runs/worktrees, Run resume after restart, automatic integration, or Desktop Run management.

## Transition

Phase 8 is locally verified on `a74b318`. This artifact records local evidence only. Integration review is the next gate; do not merge local `master`, push, or start Phase 9 automatically.
