# Phase 7 Verification — Optional Git Worktree Isolation

**Status:** passed locally; integration review pending
**Date:** 2026-09-08
**Implementation commit:** `4b113756518e065acd47153ee5764d82af97b887`
**Requirements:** ISO-01, ISO-02, ISO-03 plus ADM-CORE-002/003/009/010 and ADM-DEV-001..003 constraints named by 07-01.

## Result

Phase 7 adds Git worktree as an optional isolation capability around the existing Environment model. It does not redefine Workspace or Environment around Git: ordinary Environment creation remains Workspace-contained and non-Git development remains green. Managed worktree state is persisted separately and only the isolation service can create an Environment rooted outside the source Workspace.

A managed worktree receives stable ADM `wt_` identity, an ADM-generated `adm/wt_...` branch and an ADM-owned destination under the state-directory `worktrees` root. The caller chooses the Workspace, Environment display name and optional base ref, but cannot choose an arbitrary managed destination or branch name. Creation verifies that the registered Workspace root is the Git top-level and resolves the requested base ref to a concrete commit.

Before any routed Runtime is constructed, `app.Service.Runtime` asks the isolation service to validate an Environment. Ordinary in-Workspace Environments pass without Git. A managed Environment must still match its persisted Environment/worktree relation, remain under the ADM-owned root, exist as the same filesystem directory, report itself as the Git top-level, share the recorded Git common directory, and remain on the recorded managed branch. Missing/tampered identity therefore fails before routed mutation; the same Runtime seam is also used by Gateway-owned dev-process startup.

Destroy is writer-exclusive. Default destroy checks both worktree dirtiness and locally advanced HEAD publication evidence. Dirty or locally advanced work whose HEAD is absent from every remote-tracking ref is refused unless `force=true`. Whether clean or forced, destroy removes the worktree directory/metadata but retains the generated branch, so committed work is not silently deleted with the worktree root. Generic `environment_remove` refuses managed worktree Environments and cannot bypass this policy.

## Acceptance

| ID | Result | Evidence |
|---|---|---|
| P1 non-Git behavior remains valid | pass | `TestNonGitEnvironmentRemainsValidWhenWorktreeIsolationUnavailableForWorkspace`; `TestGatewayDevelopsPlainDirectoryWithoutGit` over Gateway remains green |
| P2 two managed Environments are isolated | pass | `TestCreateTwoManagedWorktreesKeepsSourceCheckoutUnchanged` creates two real Git worktrees with distinct `wt_` IDs, Environment IDs, roots and branches; a file written to lane A is absent from lane B and source checkout |
| P3 source checkout unchanged | pass | same test records source branch, HEAD and tracked content before create and verifies all remain unchanged afterward |
| P4 ADM-owned root + revalidation | pass | managed roots are generated beneath `<state-dir>/worktrees/<workspace-id>/<wt-id>`; `TestManagedWorktreeTamperBlocksRoutedMutationBeforeFileChange` changes branch identity and proves routed `Write` fails before creating the target file |
| P5 unmanaged out-of-Workspace root is rejected | pass | `TestOutOfWorkspaceEnvironmentWithoutManagedRecordIsRejectedByRuntime` tampers persisted Environment root outside Workspace and proves mutation is rejected without managed metadata |
| P6 safe dirty destroy | pass | `TestDestroyRefusesDirtyByDefaultAndForceRetainsBranch` refuses default destroy, accepts explicit force and verifies root removal plus retained branch |
| P7 safe unpublished-commit destroy | pass | `TestDestroyRefusesUnpublishedCommitAndRetainsCommittedWork` creates a real local commit, refuses default destroy, force-removes root, and verifies the retained branch still resolves to that commit |
| P8 Agent Gateway lifecycle + optional failure | pass | `TestGatewayManagedWorktreeLifecycleIsOptionalAndSafe` uses real MCP Streamable HTTP to create/list/destroy a real managed worktree; generic remove is refused, retained branch/source checkout are verified, non-Git worktree create fails locally, and ordinary Environment create still succeeds |

## Deterministic regression

Fresh named acceptance (`-count=1`):

- real Git isolation lifecycle: `go test ./internal/isolation ... -count=1` — pass in 7.820s
- Runtime revalidation/tamper: `go test ./internal/app ... -count=1` — pass in 1.179s
- real Streamable HTTP managed-worktree lifecycle: `go test ./internal/gateway -run ^TestGatewayManagedWorktreeLifecycleIsOptionalAndSafe$ -count=1` — pass in 1.505s
- non-Git Gateway regression: `go test ./internal/gateway -run ^TestGatewayDevelopsPlainDirectoryWithoutGit$ -count=1` — pass in 0.123s

Broader gate:

- `go test ./internal/isolation ./internal/environment ./internal/app ./internal/gateway` — pass; latest package timings 8.948s / 0.162s / 4.238s / 36.438s
- `go test ./...` — all test-bearing packages passed; no-test packages compiled successfully
- `go vet ./...` — pass
- `git diff --check` — pass; only line-ending conversion warnings, no whitespace error

Exact command record: `evidence/regression.json`.

## Authority and failure boundaries

- Git remains operation-local. A plain non-Git Workspace can still create/use an ordinary Environment and all normal file capabilities.
- Managed creation does not accept destination or managed branch parameters.
- Managed roots outside a Workspace need persisted managed metadata; arbitrary out-of-Workspace Environment roots are rejected at Runtime routing.
- Managed destroy requires the matching writer owner.
- Generic Environment removal cannot delete a managed root or bypass dirty/unpublished checks.
- Force is explicit and only relaxes worktree-directory safety; it does not delete the managed branch.
- No automatic merge, branch delete, push, remote synchronization, Agent Run or parallel orchestration behavior exists in this phase.

## Scope review

Phase 7 implements isolation only. It does not implement Phase 8 Agent Run identity/status/cancel or Phase 11 parallel Agent scheduling. Desktop/package work remains frozen. No OpenCode/GSD/other LLM subagent was invoked for implementation or verification.

Phase 7 is locally verified on `feat/optional-git-worktree-isolation` and stops for integration review. Local `master` is unchanged by Phase 7 and no push was performed.
