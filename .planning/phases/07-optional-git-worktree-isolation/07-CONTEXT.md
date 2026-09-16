# Phase 7: Optional Git Worktree Isolation — Context

## Scope
Implement ISO-01, ISO-02 and ISO-03 as an optional managed Git-worktree isolation capability around the existing Workspace/Environment model.

## Locked decisions
- Git/worktree remains operation-local. Ordinary Workspace registration, non-Git Environment creation, files, Memory, Skills, MCP and verifier behavior remain valid without Git or worktrees.
- A managed worktree is separate persisted isolation metadata keyed to an Environment; Git fields do not become intrinsic Environment fields.
- Managed worktree roots live only under an ADM-owned directory derived from the active ADM state root, never under arbitrary caller-supplied paths.
- A managed worktree Environment may therefore have a root outside the source Workspace directory only when a matching ADM managed-worktree record exists and revalidates successfully.
- Creation uses the registered Git Workspace checkout as source, resolves a base commit, creates an ADM-generated branch and a distinct managed root, and does not switch or rewrite the source checkout.
- Routed Runtime access revalidates managed identity before use: Environment/Workspace relation, owned-root containment, existing worktree root, Git top-level, Git common directory and managed branch must still match persisted metadata.
- Generic `environment_remove` must not orphan a managed worktree. Managed worktrees are destroyed through an explicit isolation operation.
- Destroy is writer-exclusive. By default it refuses dirty work and locally advanced commits not present in any remote-tracking ref. `force=true` is the explicit override for those unsafe conditions.
- Destroy removes the managed worktree root and ADM Environment/isolation metadata but never deletes the managed Git branch; retained branch identity is returned.
- Phase 8 Agent Run lifecycle, Phase 11 parallel orchestration, automatic integration/merge/push, Desktop expansion and packaging remain out of scope.

## Acceptance boundary
1. Existing non-Git Environment behavior and file mutation remain green with no Git/worktree requirement.
2. One Git Workspace can create two managed worktree Environments with distinct ADM-owned roots and branches.
3. Source checkout branch/root/files remain unchanged by managed worktree creation.
4. Dirty or locally advanced managed worktree destroy is rejected unless the caller holds the Environment writer and explicitly sets `force=true`; the branch is retained either way.
5. Deleting/tampering a managed root or redirecting its Git metadata/branch causes routed Runtime mutation to fail before touching files.
6. Clean managed worktree destroy removes only the managed root plus ADM records and leaves the source checkout and retained branch intact.
