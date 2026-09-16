# Phase 7 Integration Review — Optional Git Worktree Isolation

**Status:** passed for local integration
**Date:** 2026-09-08
**Reviewed head:** `8022856`
**Target:** local `master@9ae56ad`

## Scope reviewed

Phase 7 adds optional managed Git worktree isolation around the existing Workspace/Environment model. Git remains optional for ordinary Environment development. Managed worktree metadata is persisted separately; caller-facing create does not accept an arbitrary destination or branch, Runtime revalidates managed root/Git identity before routed access, and destroy is writer-exclusive with dirty/unpublished refusal by default and explicit force override while retaining the managed branch.

The Phase 7 diff from `master` contains only Phase 7 planning/evidence, product contract changes, managed-worktree model/service/runtime/Gateway code, and tests. No Agent Run, planner/executor/reviewer, parallel orchestration, Desktop expansion, merge/rebase/push automation, or migration layer is introduced.

## Review findings

No integration blocker remains.

During local verification, destroy metadata cleanup was hardened so successful Git worktree removal does not depend on a second writer lease check after the destructive Git operation. Gateway acceptance was also raised from in-memory transport to real Streamable HTTP. Both changes were revalidated before the implementation commit.

The final feature branch is a direct descendant of `master@9ae56ad`, so integration can be a conflict-free fast-forward. `git diff --check master..HEAD` passed.

## Review gate evidence

- `go test ./internal/isolation -count=3` — pass in 25.950s.
- `go test ./internal/gateway -run TestGatewayManagedWorktreeLifecycleIsOptionalAndSafe -count=3` — pass in 5.280s using the real Streamable HTTP Gateway acceptance path.
- Earlier current-head regression recorded in `07-VERIFICATION.md` and `evidence/regression.json`:
  - `go test ./...` — pass across all packages.
  - `go vet ./...` — pass.
  - `git diff --check` — pass.
  - named non-cached acceptance proves two managed roots from one Git Workspace, unchanged source checkout, cross-root mutation isolation, managed branch tamper rejection before mutation, dirty/unpublished safe-destroy refusal, force removal with retained branch, generic Environment remove refusal, and ordinary non-Git Environment/Gateway behavior remaining usable.

## Requirement decision

- **ISO-01:** passed — Git worktree remains optional; non-Git Environment behavior remains valid.
- **ISO-02:** passed — managed roots are generated under the ADM-owned state root and revalidated before Runtime access.
- **ISO-03:** passed — destroy requires matching writer, refuses dirty/unpublished work by default, requires explicit `force=true` for unsafe removal, and retains the branch.

## Integration decision

Phase 7 is approved for fast-forward integration into local `master`. After the fast-forward, run post-integration `go test ./...`, `go vet ./...`, and `git diff --check`, then record the resulting master head. Do not push. Phase 8 may start only after that post-integration gate passes and should use a separate isolated feature worktree.
