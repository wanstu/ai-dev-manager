# Phase 7 Plan 07-01 Summary — Optional Git Worktree Isolation

**Status:** complete and locally verified; integration review pending
**Implementation commit:** `4b11375` — `feat(isolation): add managed git worktrees`
**Planning commit:** `5c1334f` — `docs(07): plan optional worktree isolation`

## Delivered

- Added separate persisted `ManagedWorktree` metadata rather than adding Git/worktree fields to the intrinsic Environment model.
- Added `internal/isolation.Service` for create/list/validate/destroy of ADM-managed Git worktrees.
- Managed destinations are generated under the ADM state directory (`worktrees/<workspace-id>/<wt-id>`); callers cannot supply an arbitrary filesystem destination or managed branch name.
- Creation requires the registered Workspace root itself to be the Git top-level, resolves a concrete base commit, generates `wt_` identity plus `adm/wt_...` branch, and leaves the source checkout branch/HEAD/files unchanged.
- Managed Environment roots outside the Workspace are permitted only through the internal managed creation seam. Ordinary `environment_create` remains Workspace-contained.
- `app.Service.Runtime` revalidates managed root, Git top-level, common-dir and branch identity before routed Runtime access. Missing/replaced/tampered managed roots therefore fail before file/runtime mutation.
- Gateway now exposes `environment_worktree_create`, `environment_worktree_list`, and `environment_worktree_destroy` without exposing arbitrary path/branch Git authority.
- Generic `environment_remove` refuses managed worktree Environments so it cannot bypass worktree safety policy.
- Destroy requires the matching Environment writer. Clean destroy removes only the worktree root and retains the generated branch. Dirty or locally advanced/unpublished work is refused by default; `force=true` is required for reviewed unsafe removal and still retains the branch.
- Product contract now records Git worktree as the first implemented optional isolation mechanism while keeping non-Git Environment semantics unchanged.

## Verification highlights

Real temporary Git repositories/worktrees are used by the isolation tests. They prove two distinct managed roots/branches, cross-root file isolation, unchanged source checkout, managed-root containment, tamper detection, dirty/unpublished destroy refusal, force behavior and branch retention.

The Agent-facing lifecycle is also exercised over a real MCP Streamable HTTP endpoint (`httptest` server + `StreamableClientTransport`), not only an in-memory transport. The existing non-Git Gateway development acceptance remains green.

Final local gate passed:

- named real-Git isolation acceptance: 7.820s
- app Runtime/tamper acceptance: 1.179s
- real Streamable HTTP worktree Gateway acceptance: 1.505s
- non-Git Gateway regression: 0.123s
- focused isolation/environment/app/gateway regression: pass
- `go test ./...`: pass
- `go vet ./...`: pass
- `git diff --check`: pass

Exact regression record: `evidence/regression.json`.

## Scope boundary

No Agent Run lifecycle, parallel execution scheduler, automatic integration/merge, branch deletion, remote push, Desktop expansion, installer/package work, or generic arbitrary Git command API was introduced.

Phase 7 stops at local verification for integration review. Phase 8 is not started automatically and nothing is pushed.
