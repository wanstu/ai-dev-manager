# Phase 22 Context — Temporary Task Environments + Safe Cleanup Workflow

Date: 2026-09-13
Planning baseline: `b6c96cd` (`docs: close Phase 21 verifier observability acceptance`)
Status: COMPLETE. 22-01 Core lifecycle, 22-02 shared Agent/Admin workflow, and 22-03 Desktop visibility + integrated fixed-head closeout are delivered and validated.

## Goal

Make temporary Environment lifecycle a first-class Agent capability by composing the retention/cleanup primitives delivered in Phase 15 with the runtime-observation boundaries delivered through Phases 5-8 and 21.

An external Agent may explicitly create a short-lived development Environment, inspect its retention/cleanup state, promote it to durable, or clean it up when current evidence says cleanup is safe. ADM remains the lifecycle/authority layer only; it does not define tasks, schedule work, decide completion, merge Git, or advance GSD state.

## Contract mapping

Phase 22 implements/refines:

- ADM-GOAL-001/002 — usable Agent development capabilities without ADM task orchestration.
- ADM-CORE-001/002/003/004 — Workspace/Environment roots and operation-local optional capabilities.
- ADM-CORE-005 — active writer blocks cleanup.
- ADM-CORE-010 — managed worktree is optional isolation with existing destroy safety.
- ADM-CORE-013 — Environment-private Memory follows Environment lifecycle and is never promoted implicitly.
- ADM-CORE-016/017 — Environment identity/management semantics remain metadata-safe and informative.
- ADM-CORE-022 — explicit owner+TTL temporary Environment lifecycle, targeted safe cleanup and promotion.
- ADM-GW-001/003 — stable IDs and local failures.
- ADM-MGMT-001 / ADM-DESKTOP-001 — Desktop visibility uses the same Admin MCP/Core state.
- ADM-NONGOAL-001 — no task/GSD orchestration.

New prerequisite: none. Git is required only when the caller explicitly requests managed-worktree mode.

## Source calibration at `b6c96cd`

Phase 15 already provides most low-level safety primitives:

1. `model.ResourceRetention` stores persistence class, creator surface, owner/session identity, timestamps, expiry, policy and attached Environment.
2. `Environment.CreateWithRetention` can create an ordinary Environment with retention metadata, but no Agent-facing temporary create workflow uses it.
3. Managed worktree creation always creates a durable Environment today; the isolation path has no retention-aware create variant.
4. `resource_retention_inspect` and `resource_retention_cleanup` can preview/execute generic retention cleanup, but cleanup is installation-wide rather than targeted to one requested task Environment.
5. Ordinary temporary Environment cleanup is state-only and preserves the underlying Workspace/project directory.
6. Managed-worktree retention cleanup already reuses `Isolation.Safety` and `Isolation.Destroy(force=false)`, so dirty/unpublished work blocks deletion and the branch is retained.
7. Gateway owner cleanup currently checks MCP sessions/in-flight work, dev processes and generic `run_`; Phase 21 added `vfrun_`, which is not yet included in retention blockers.
8. `PromoteResourceRetention` already makes a temporary resource durable, but it is a generic Admin retention mutation and does not enforce temporary-Environment lifecycle ownership.
9. Environment summaries already contain retention metadata through the shared model, but the Desktop currently gives temporary lifecycle no explicit Environment UX.

These facts mean Phase 22 should compose/refine existing primitives rather than create a new task model or cleanup engine.

## Locked design decisions

### 1. Temporary Environment is still a normal Environment

No `task` entity, workflow record, planner state or parent/child execution graph is introduced. A temporary Environment uses the existing stable `env_` identity and all normal file/Runtime/MCP/Skill/Memory rules.

`temporary` is retention/lifecycle intent only.

### 2. Creation is atomic with retention intent

The constrained temporary-create path must never create a durable Environment first and then mark it temporary in a second mutation.

Required lifecycle inputs:

- `workspace_id`
- human-readable `name`
- stable `owner_id`
- positive `ttl_seconds`

Optional provenance:

- `session_id`
- `run_id`

`run_id` is provenance only. It may identify an external/Gateway run but does not make a Run a prerequisite, does not create execution ownership, and is not used as proof that cleanup is safe.

Creator surface is set by the calling surface, not trusted from arbitrary user input.

### 3. Two creation modes, one Environment contract

Mode `existing_root`:

- default mode;
- root defaults to the registered Workspace root;
- optional root must already exist and remain inside that Workspace;
- creation writes ADM metadata only;
- cleanup never deletes the root or files.

Mode `managed_worktree`:

- optional Git isolation only;
- root is ADM-chosen under the existing owned worktree root;
- optional `base_ref` follows existing managed-worktree validation;
- the Environment must be persisted as temporary in the same managed Environment persistence operation;
- creation failure rolls back worktree/branch exactly as the existing isolation path does.

No copy/clone/container mode is added.

### 4. No accidental durable reuse

The temporary-create workflow must not return an already-existing durable Environment as a successful temporary creation due to the current ordinary `CreateWithRetention` duplicate shortcut.

Temporary creation either creates a new `env_` with the requested temporary retention or fails clearly on a conflicting existing Environment identity/root/name combination.

### 5. Owner-scoped lifecycle mutations

Temporary status/preview may be read-only by stable Environment ID.

Promotion and cleanup execution require the supplied lifecycle `owner_id` to match the Environment retention owner. This is analogous to explicit writer-owner routing: it is not a new authentication system, but it prevents a normal Agent lifecycle call from silently operating on another temporary owner's Environment.

Admin generic retention tools remain the explicit recovery/management override path.

### 6. Targeted cleanup, not a global sweep

Phase 22 adds an Environment-scoped cleanup path. Preview/execute evaluates only the requested Environment and must not opportunistically remove other expired temporary MCPs, Skills or Environments.

Implementation should refactor/reuse the existing retention report/execution policy with selectors rather than duplicate blocker logic.

Preview is always non-mutating. Execute performs a fresh safety recheck.

### 7. Runtime blockers include every current owned execution family

Environment cleanup remains blocked by:

- active writer;
- owner-local MCP session or in-flight MCP operation;
- active `proc_` development process;
- active generic `run_`;
- active async verifier `vfrun_`;
- managed-worktree dirty/unpublished or identity/safety failure;
- Environment not ready, owner/expiry mismatch, or insufficient evidence.

Terminal process/run/verifier observations do not block merely because their records remain queryable.

### 8. Expiry is intent, not permission

`ttl_seconds` must be positive and produces an explicit persisted `expires_at`. Expiry alone never deletes anything and never overrides runtime/Git safety.

Phase 22 adds no background GC, timer, startup cleanup or automatic retry. Cleanup remains an explicit call.

### 9. Promotion preserves development context

Promotion changes retention from temporary to durable only. It preserves Environment ID, Workspace/root, managed worktree metadata/branch, selections, private Memory and normal runtime semantics. It does not merge/push Git or move files.

After promotion, temporary cleanup reports the resource as durable/not eligible.

### 10. Desktop visibility is thin management UX

Phase 22 includes bounded Desktop visibility because it is explicitly in the post-1.0 scope:

- show durable/temporary status and expiry in Environment management;
- show owner/session/run provenance without treating it as orchestration;
- allow explicit promote and cleanup preview/execute through Admin MCP;
- show blockers before destructive action and require explicit confirmation;
- no Desktop-only state, cleanup policy or direct `state.json` access.

CLI lifecycle UX remains Phase 23 scope.

## Preferred shared Gateway tools

Names may be calibrated against implementation conventions, but preferred names are:

- `environment_temporary_create`
- `environment_temporary_status`
- `environment_temporary_promote`
- `environment_temporary_cleanup`

These are shared Agent/Admin capability tools. Existing generic `environment_create` remains Admin-only and durable by default; existing generic `resource_retention_*` tools remain Admin management surfaces.

`environment_temporary_cleanup` uses explicit `execute=false|true`; false is preview. The temporary workflow exposes no `force` flag.

## Explicit non-goals

1. No ADM task record, task queue, Planner/Executor/Reviewer workflow, GSD phase state or parent/child Agent orchestration.
2. No automatic cleanup/GC/background scheduler.
3. No automatic merge/rebase/push or branch deletion.
4. No force-delete option in the temporary cleanup workflow.
5. No deletion of ordinary Workspace/project directories or unmanaged host files.
6. No copy/clone/container isolation mode.
7. No automatic creation of temporary MCP/Skill resources and no cascade cleanup of unrelated retention resources.
8. No new CLI lifecycle UX in Phase 22; Phase 23 owns that work.
9. No second persistence model or Desktop-only lifecycle state.
10. No push/tag/release/subagents and no automatic Phase 23 start.

## Delivery sequence

### 22-01 — Core temporary Environment lifecycle

Add/extend the shared model and application/isolation seams for atomic temporary creation, run provenance, owner-scoped promotion, targeted retention preview/execute and `vfrun_` cleanup blocking. Prove ordinary non-Git and managed-worktree safety at Core/Gateway-owner level. No new MCP tools or Desktop UI yet.

### 22-02 — Agent/Admin temporary Environment workflow

Expose the four shared Gateway tools and prove real Streamable HTTP create/status/promote/preview/cleanup flows, wrong-owner rejection, restart behavior, active runtime blockers and no unrelated-resource sweep. Keep generic Admin retention surfaces intact.

### 22-03 — Desktop visibility + integrated closeout

Add bounded Environment retention visibility and explicit promote/cleanup UX over Admin MCP, then run browser/Wails plus fixed-head Core/Gateway/full regression acceptance and close Phase 22. Native GUI click-through remains evidence-by-availability; do not claim PASS without a real native-control path.

Do not start Phase 23 automatically after closeout.
