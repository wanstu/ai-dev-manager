# Phase 18 — Desktop Management UX Reorganization

Date: 2026-09-11
Baseline: `0173259 docs: define post-1.0 phase map`, branch `master`
Stable release baseline: `v1.0.1` (recorded green in repository planning)
Status: 18-01/02/03/04 detailed; implementation has not started; 18-01 is the next execution plan

## User intent and working rules

Review the new post-1.0 phase map, define the UI refactor, and persist decisions and execution checkpoints promptly in `.planning/` so a quota/context interruption does not block continuation. The authorized work for this session is planning, not implementation.

Read AGENTS, STATE, PROJECT, ROADMAP and PHASE-MAP first. Check Git status before editing; acquire the Environment writer lease before writes; make local commits at clear nodes on master; do not push, tag, release or use subagents. No new product prerequisites.

## Review decision

The phase order is reasonable: daily human management is the immediate bottleneck after the stable release, and Phase 18 can improve it using existing capabilities. The user explicitly requested durable execution planning for all remaining Phase 18 slices before implementation, so 18-01/02/03/04 are detailed now; Phases 19-26 remain phase-level until opened.

| Area | Assessment / constraint |
|---|---|
| Phase 18 | Approve the direction; menu shell + dashboard + section routing is the first small slice. Do not redesign every feature page or change Core alongside it. |
| Phase 19 -> 20 | A bounded workspace/tree summary is an input to the richer Agent context bundle. Preserve registered-root containment, explicit budgets and missing/partial evidence. |
| Phase 21 | Async verifier is independently valuable; the existing pjadm timeout dogfood note is real evidence. It need not wait for a redesigned Desktop Runtime page. If synchronous verification blocks work after Phase 18, explicitly reprioritize in STATE before changing execution scope. |
| Phase 22 vs Phase 15 | Retention, conservative cleanup, promotion and managed-worktree safety already exist. Plan 22 as workflow/accessibility over those primitives, not a second lifecycle model. Task labels/attachments must not grow ADM task planning or automatic integration. |
| Phase 23 | Reuse the same Core/Admin MCP operations and context outputs; no alternate provisioning state or authority. |
| Phase 24 vs Phase 18 | Local frontend separation needed for menu routing is allowed in 18. Cross-surface package/repository reorganization remains 24; neither phase depends on a second state store. |
| Phases 25 / 26 | Keep conditional; release polish and deeper investigation helpers need concrete dogfood evidence. |

## Requirement traceability

These are refinements of existing requirements, not new Core semantics:

- ADM-DESKTOP-001: embedded Desktop shell and management bindings.
- ADM-MGMT-001 / ADM-CORE-015: one management boundary and shared persisted development context.
- ADM-CORE-001 / 002 / 014 / 016: Workspace/Environment identity, ordinary directories and metadata-safe lifecycle.
- ADM-CORE-003 / 004 / 020: operation-local optional capabilities and truthful availability.
- ADM-CORE-005 / 007: preserve existing writer and executable authorization.
- ADM-CORE-012 / 013 / 017: global catalog vs Environment selection, explicit Memory scope, no overview/private-value leakage.
- PROC-01..03 / ARUN-01: existing Runtime visibility remains reachable and owner-local.
- ADM-GOAL-002 / ADM-NONGOAL-001: no Agent/GSD orchestration in ADM.

## Locked delivery boundary

1. Desktop becomes a navigable management application with a compact persistent connection/context header, grouped navigation and one active content section.
2. The dashboard summarizes existing sanitized snapshot/status facts and links to management sections. Counts of configured definitions are not evidence of healthy Runtime.
3. Existing feature DOM/forms/handlers are reused in 18-01. Keep one instance per control ID and shared dialogs outside hidden page containers. Feature-page redesign is a later slice.
4. Route, filter, focus and current selection are transient presentation state. No new persistence, framework, router package, frontend build pipeline or business model is required.
5. Production Desktop continues to use the connected Admin MCP client. Disconnected shell/settings remain usable; management data/actions must not silently fall back to writable local state.
6. Profile changes clear old connection-bound snapshot, selection, detail, Memory, Runtime and observation state; results from an old target must not populate a new target.
7. Navigation/dashboard must not implicitly load Memory values, probe/reconnect MCPs, refresh Skill sources, acquire writer, execute verifier, start/stop Runtime resources or mutate selection.
8. No new Core APIs or backend expansion in 18-01. Missing UI support is recorded for a later bounded plan, not filled by inventing a new prerequisite.
9. Preserve the accepted connection profile, modal, tray/autostart, icon and packaging behavior from Phase 16. Existing Desktop preferences are a prior explicit exception; Phase 18 adds no preference file.
10. Full UI completion requires executable/browser interaction and real Wails smoke evidence. Go source/asset marker tests alone do not prove clickability, focus or draft preservation.

## Planning consistency notes

PROJECT's 2026-09-08 reality audit and parts of PRODUCT_CONTRACT describe an initial Desktop slice before Phase 16 management convergence. Treat those as historical implementation context: current STATE, completed 16-03C/D and production NewClientAdapter define the established connected Desktop path. The shell opening offline is distinct from management availability while disconnected.

The original STATE totals (27 completed plans) were stale. The reconciled inventory counts numbered NN-NN-PLAN.md files in phases 01-18: 29 closed historical plans plus pending 18-01/02/03/04, for 33 total. Exclude the phase-00 bootstrap and per-slice SUMMARY files; Phase 09 remains superseded historical delivery and 16-03 counts the closed local scope with remote 16-03E still deferred. Phase completion remains 17/26 (65%, rounded by the existing phase-count convention).

## Planned slices

- 18-01 (next executable plan): menu shell, read-only dashboard and routes to existing sections, with behavior/negative acceptance and Wails smoke.
- 18-02: Workspace/Environment list-detail readability and existing Runtime subviews/output, calibrated against the delivered 18-01 shell.
- 18-03: MCP/Skill scope hierarchy, explicit Global/Environment Memory management, existing system controls and Environment diagnostic presentation.
- 18-04: mandatory integrated Desktop acceptance across all ten routes, keyboard/scaling/context transitions and the exact Wails artifact; code fixes occur only when acceptance evidence demonstrates a regression.
- Delivery order is 18-01 -> 18-02 -> 18-03 -> 18-04. Detailed later plans may be narrowed by predecessor evidence, but their acceptance boundary is not silently skipped.
- Phase 18 does not implement discovery/context bundling, async verifier, new cleanup workflow, broad CLI split, distribution or investigation expansion.

## Continuation checkpoint

The initial scope checkpoint was committed at `d95b088`; the first full design/18-01 plan landed at `93a74d9`. Detailed 18-02 and 18-03 plans landed at `b6da7fc` and `f52b872`; 18-04 and the cross-plan synchronization are the final planning node for this session. `18-UI-REFACTOR.md`, `18-VALIDATION.md`, STATE/PROJECT/ROADMAP/PHASE-MAP and `18-PLANNING-LOG.md` describe the same four-plan delivery order. After the planning closeout commit, the next session checks Git/context, acquires a fresh writer lease and begins 18-01 Task 1. No feature code, new tests, build or UI acceptance has been completed by this planning session.
