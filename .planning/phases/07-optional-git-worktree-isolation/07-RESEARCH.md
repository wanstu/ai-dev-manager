# Phase 7: Optional Git Worktree Isolation — Research

## Current architecture findings
- `model.Environment` contains only development-context fields; Git is not intrinsic.
- `environment.Service.Create` currently accepts only roots contained by the registered Workspace. Phase 7 therefore needs one internal managed-root creation path without widening the public arbitrary-root rule.
- `app.Service.Runtime` is the common constructor used by file/search/exec/verifier/Git operations and by Gateway-owned dev-process startup. It is the narrow revalidation seam for managed roots.
- writer leases are already keyed by physical root and therefore naturally isolate two distinct managed worktree roots.
- existing optional Git Runtime operations call the host `git` executable directly and do not require the general exec allowlist.
- generic `environment_remove` is metadata-only today; it must refuse managed-worktree Environments so managed filesystem state cannot be orphaned accidentally.

## Local Git target
- Current Windows target: Git 2.55.0.windows.3.
- `git worktree list --porcelain` exposes absolute worktree path, HEAD and branch refs for all existing linked worktrees.
- The existing project already uses linked worktrees successfully without switching the primary checkout, matching the Phase 7 isolation model.

## Proposed managed record
Persist a separate `ManagedWorktree` record with stable `wt_` identity, Environment ID, Workspace ID, root, generated branch, creation base commit, Git common directory and creation time. The record is desired/managed isolation metadata, not transient observation.

## Owned-root policy
Derive the only allowed managed root from the active state file directory: `<state-dir>/worktrees/<workspace-id>/<managed-worktree-id>`. Callers choose a display name and optional base ref, but never a filesystem destination or branch name.

## Creation semantics
1. Verify Git is available and Workspace path is the repository top-level for this operation.
2. Resolve the requested base ref (default `HEAD`) to a commit.
3. Resolve the source repository common Git directory.
4. Generate `wt_...`, branch `adm/<wt-id>`, and owned destination.
5. Run `git worktree add -b <branch> <owned-root> <base-commit>`.
6. Create an Environment through an internal managed-root path, then persist the managed record. Roll back a failed unpublished creation before exposing the Environment.

## Revalidation semantics
A normal Environment inside its Workspace is unchanged. An Environment outside its Workspace is usable only when a matching managed record exists. Before Runtime construction, managed environments must prove:
- persisted Environment and managed record agree on Environment ID, Workspace ID and root;
- root remains contained by the ADM-owned worktree root;
- root exists and resolves to the persisted path;
- `git rev-parse --show-toplevel` resolves to that root;
- `git rev-parse --git-common-dir` resolves to the persisted source common directory;
- current branch equals the persisted managed branch.

This catches missing roots, redirected `.git` metadata and branch substitution before routed file/exec/verifier/process mutation.

## Destroy semantics
Destroy requires the matching Environment writer. Safety inspection records dirty status, current HEAD and whether locally advanced HEAD is contained by a remote-tracking ref. Without `force`, reject dirty state or a HEAD that moved from the creation base and is not present in any remote-tracking ref. With `force`, Git may remove dirty files, but ADM never deletes the branch. After successful worktree removal, remove only the Environment and managed record.

## Non-goals
No arbitrary destination, arbitrary branch management, automatic merge/rebase/push, detached-run orchestration, repository cloning, multi-agent scheduling, Desktop UI, or compatibility migration.
