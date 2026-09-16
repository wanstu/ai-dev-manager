# Phase 7: Optional Git Worktree Isolation — Validation

## Deterministic checks

### P1 — Non-Git regression
Create/register an ordinary non-Git Workspace and Environment. Prove Environment creation plus read/write/search still work and managed-worktree APIs fail only for the Git-specific operation.

### P2 — Two isolated managed roots
Create a temporary Git Workspace with an initial commit. Create two managed worktree Environments. Assert distinct `wt_` IDs, Environment IDs, roots and generated branches; both roots are under the ADM-owned state directory.

### P3 — Source checkout unchanged
Capture source checkout branch, HEAD, root and a tracked file before P2. After both creates, assert the source branch/HEAD/file are unchanged and `git worktree list --porcelain` shows both additional roots.

### P4 — Routed mutation revalidation
Acquire a writer for one managed Environment and prove a normal write succeeds. Then tamper the managed branch or Git link/root and assert the next routed mutation fails before changing the target file. Also prove an out-of-Workspace Environment without a managed record is refused.

### P5 — Safe destroy default
In one managed worktree, create dirty/untracked work and assert destroy without force is rejected. Restore cleanliness, create a local commit after the creation base with no remote-tracking containment, and assert destroy without force is rejected again.

### P6 — Explicit force policy and branch retention
With the matching writer and `force=true`, destroy the unsafe managed worktree. Assert root removal and ADM Environment/managed-record removal, while the generated branch still exists in the source repository.

### P7 — Clean destroy
Destroy a clean untouched managed worktree with the matching writer and no force. Assert root and ADM records are removed, source checkout is unchanged, and the generated branch is retained.

### P8 — Gateway surface
Gateway tool tests prove managed create/list/destroy inputs and outputs route through the same app/isolation service, generic `environment_remove` refuses a managed worktree, and ordinary Environment tools remain available without Git.

## Regression gate
- focused isolation/environment/app/Gateway tests
- `go test ./...`
- `go vet ./...`
- `git diff --check`

No OpenCode/GSD/other LLM subagent is used. Phase completion requires real local Git worktree filesystem acceptance on the current Windows target, not only mocked command tests.
