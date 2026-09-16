# Post-1.0 Phase Map

Date: 2026-09-11
Base: `c898586` (`v1.0.1` green release hotfix)

This document sets the high-level post-1.0 phase sequence. It intentionally does not define execution-level `PLAN.md` files for every phase. Each phase gets a detailed plan only when work on that phase starts.

## Post-1.0 planning rules

1. User-facing management UX is the first priority after the green 1.0 line.
2. Desktop remains a management surface over the shared Core; it must not introduce Desktop-only state, authorization or persistence.
3. CLI remains a management/runtime surface over the same Core; CLI and Desktop can be separated logically before any repository split.
4. ADM supplies capabilities, lifecycle, authorization and diagnostics. It must not become a task planner/orchestrator.
5. Long-running work should prefer async start/status/cancel patterns over blocking request/response calls.
6. Large workspace navigation must be bounded and explainable; ADM should help Agents map human language to likely project roots without scanning arbitrary host paths.
7. Temporary resources must remain explicit, inspectable and conservative to clean up.
8. Distribution polish remains conditional unless daily use proves it is blocking.

## Phase sequence

| Phase | Name | Priority | Status | Purpose |
|---|---|---:|---|---|
| 18 | Desktop Management UX Reorganization | P1 | Complete | Make ADM easier for the human operator to manage through a menu-based Desktop shell and validated section workflows. |
| 19 | Workspace Discovery + Project Navigation | P1 | Complete | Let ADM summarize large workspace directories and suggest likely project roots such as `projects/p2`. |
| 20 | Agent Context Bundle + Capability Injection | P1 | Complete | Give Agents a compact explicit Environment context bundle: root, tree digest, enabled MCP/Skill summary, verifier/run guidance and capability reasons, without hidden session selection. |
| 21 | Async Verifier + Long Operation Observability | P1 | Complete | Move heavy verifier/test workflows toward owner-local async observable lifecycle instead of relying on long blocking calls. |
| 22 | Temporary Task Environments + Safe Cleanup Workflow | P1/P2 | COMPLETE | Make task-scoped temporary Environments first-class and safely cleanable/promotable. |
| 23 | CLI Agent UX + MCP/Skill Provisioning | P2 | COMPLETE | Improve CLI flows for MCP/Skill import, enablement, refresh, diagnostics and Agent-friendly setup. |
| 24 | Desktop/CLI Surface Boundary Split | P2/P3 | COMPLETE | Separate Desktop and CLI surfaces logically while preserving one shared Core/state/Admin-MCP model; 24-01 through 24-03 are closed. |
| 25 | Distribution Polish If Needed | P3 conditional | Standby | Installer, updater, signing, notifications and deeper packaging polish only after concrete dogfood need. |
| 26 | Evidence-first Investigation Expansion | Conditional | Standby | Add deeper code/debug/data evidence helpers only when generic Runtime/search is insufficient. |

## Phase 18 — Desktop Management UX Reorganization

**Goal:** Reorganize Desktop from a dense management panel into a navigable application.

**Scope:** left/top menu shell, dashboard, section routing, clearer current ADM/Gateway status, and first pass at separating Workspace, Environment, Runs, MCP, Skill, Memory, Exec Allowlist, Gateway, Diagnostics and Settings views.

**Non-goals:** new Core model, new persistence, new Desktop-only authorization, installer/updater/signing, major backend expansion.

**Completion direction:** the same management capabilities become easier to find and operate without reducing existing functionality.

**Completion:** Phase 18 is complete. `18-CLOSEOUT.md`, `18-04-SUMMARY.md` and `18-VALIDATION.md` record final browser/full Go/vet/Wails/native evidence, including the D10 artifact-path note.

**Requirements:** ADM-DESKTOP-001, ADM-MGMT-001, ADM-CORE-015/017/020, preserving optional-capability, writer, catalog/Memory and Runtime boundaries. **New prerequisites:** none.

**Delivery sequence:** 18-01 establishes the compact connection/context header, grouped menu, ten real routes, truthful dashboard and shared scope/load-state contract. 18-02 refines Workspace/Environment and existing Runtime views. 18-03 refines MCP/Skill, explicit Global/Environment Memory, existing system controls and diagnostics. 18-04 is mandatory integrated acceptance across final routes/context/focus/scaling and the exact Wails artifact; only evidence-backed regressions are fixed there. Distinguish unloaded from zero, isolate auxiliary read failures, preserve dialogs and discard old-scope responses throughout.

## Phase 19 — Workspace Discovery + Project Navigation

**Goal:** Make large workspace roots understandable when a human says things like “look at p2”.

**Scope:** bounded workspace scan, project candidate summaries, git/module/package markers, suggested Environment root, tree digest and Desktop/CLI/Agent-facing views.

**Non-goals:** arbitrary full-disk indexing, semantic search engine, automatic code editing, hidden background scans.

**Completion direction:** ADM can summarize `projects/p1`, `projects/p2`, `projects/p3` style directories and help select/create the right Environment quickly.

## Phase 20 — Agent Context Bundle + Capability Injection

**Goal:** Reduce Agent setup friction by providing one compact context object for an explicitly identified Environment.

**Scope:** stable Environment/Workspace identity, Environment root, bounded tree digest, enabled MCPs and passive observed tool-name summary, enabled Skills and support summary, verifier availability, run/verifier usage guidance and unavailable capability reasons. Static MCP server instructions advertise how to obtain the bundle; dynamic data still requires explicit `environment_id`.

**Non-goals:** hidden current-Environment session state, prompt/task orchestration, choosing the task plan, auto-calling/probing MCP tools, silently injecting Memory values or full Skill instructions, interpreting Skills into ADM workflows.

**Completion direction:** an Agent can start work with one concise explicit ADM context instead of manually probing many surfaces, while preserving existing authority and privacy boundaries.

**Completion:** Phase 20 is complete. `.planning/phases/20-agent-context-bundle/20-CLOSEOUT.md`, `20-01-SUMMARY.md` and `20-02-SUMMARY.md` record bounded Core composition, passive Gateway-owner enrichment, the shared Agent/Admin context tool, static initialize guidance, full Go/vet evidence and the fixed-head artifact. Phase 21 is also complete as recorded below.

## Phase 21 — Async Verifier + Long Operation Observability

**Goal:** Make heavy verification flows observable and resilient across client/tool timeouts.

**Scope:** distinct owner-local `vfrun_` start/list/status/cancel lifecycle for configured verifier definitions, bounded live output, terminal `verifier.Result` retention, writer heartbeat/cancellation, owner/Environment cleanup, no restart persistence, and clearer blocking-request interruption diagnostics. Existing Phase-20 context guidance may point Agents toward async verification for long work without starting anything.

**Non-goals:** replacing/aliasing generic `run_`, persisted verifier history/resume, CI orchestration, automatic retry/replay, arbitrary duration cutoffs for synchronous verifier calls, Desktop/CLI feature expansion or task/GSD semantics.

**Completion direction:** long tests/builds/verifiers can be monitored across client disconnects without losing owner-local output/results to an outer tool timeout, while short synchronous verifier calls and all existing authority boundaries remain intact.

**Completion:** Phases 21 and 22 are complete. Phase 21 evidence remains in `.planning/phases/21-async-verifier-observability/`; Phase 22 evidence is recorded in `.planning/phases/22-temporary-task-environments/22-01-SUMMARY.md`, `22-02-SUMMARY.md`, `22-03-SUMMARY.md`, and `22-CLOSEOUT.md`, with final implementation head `56c79e3`.

## Phase 22 — Temporary Task Environments + Safe Cleanup Workflow

**Goal:** Let Agents create task-scoped work areas without polluting durable project state.

**Status:** COMPLETE — Core lifecycle, shared Agent/Admin workflow, Desktop visibility/actions and fixed-head integrated acceptance are delivered through `56c79e3`.

**Scope:** atomic temporary Environment creation with explicit lifecycle owner + positive TTL; optional session/run provenance; existing Workspace-contained root or optional ADM-managed worktree mode; Environment-targeted cleanup preview/execute; matching-owner promote-to-durable; active writer/MCP/process/`run_`/`vfrun_` blockers; existing managed-worktree dirty/unpublished/tamper safety; bounded Desktop visibility/actions through Admin MCP.

**Non-goals:** ADM task records/orchestration, automatic GC, unsafe deletion of ordinary host/project directories, force cleanup, automatic merge/rebase/push, branch deletion, new CLI lifecycle UX, or deleting ambiguous non-ADM roots.

**Completion direction:** temporary work can be safely created, inspected, promoted or cleaned up with conservative current evidence while ordinary non-Git Environment development stays valid and unrelated temporary resources are never swept by a targeted cleanup.

**Detailed planning:** `.planning/phases/22-temporary-task-environments/22-CONTEXT.md`, `22-01-PLAN.md`, `22-02-PLAN.md`, `22-03-PLAN.md`. All three plans are complete. Core feature commit is `0f6efa7`, Gateway feature commit is `d51a426`, and final Desktop/fixed-head implementation is `56c79e3`. See `22-01-SUMMARY.md`, `22-02-SUMMARY.md`, `22-03-SUMMARY.md`, and `22-CLOSEOUT.md`. `ADM-CORE-022` is the product contract.

## Phase 23 — CLI Agent UX + MCP/Skill Provisioning

**Goal:** Make CLI setup and automation smoother for humans and Agents.

**Status:** COMPLETE — MCP/Skill provisioning, Environment context/temporary lifecycle CLI and fixed-head integrated acceptance are delivered through `f85c566`.

**Scope:** keep normal CLI management on Admin MCP while adding MCP import-from-file/stdin, distinct global probe / Environment status / passive owner-runtime inspect / explicit refresh flows, Skill source update with multiple support roots plus global/Environment availability diagnostics, the existing Phase-20 Environment context bundle, and the existing Phase-22 temporary Environment lifecycle.

**Non-goals:** a second Admin API, direct normal-mode writable state access, hidden current Workspace/Environment, cwd-based authorization, a universal CLI formatter/query layer, new MCP/Skill/Core semantics, force temporary cleanup, task/GSD orchestration, Desktop feature work, or the Phase-24 package split.

**Completion direction:** common MCP/Skill/Environment setup tasks are easy from CLI, use explicit stable IDs, preserve the shared authority/safety contracts, and produce machine-readable existing-model output without introducing a second surface model.

**Detailed planning:** `.planning/phases/23-cli-agent-ux/23-CONTEXT.md`, `23-01-PLAN.md` and `23-02-PLAN.md`. Both plans are complete: 23-01 at `b733948`, final implementation head `f85c566`. See `23-01-SUMMARY.md`, `23-02-SUMMARY.md`, and `23-CLOSEOUT.md`. `ADM-CLI-001` is the Phase-23 product contract.

## Phase 24 — Desktop/CLI Surface Boundary Split

**Goal:** Keep Desktop and CLI independently optimizable without forking product semantics.

**Status:** COMPLETE — 24-01 shared target/management boundary, 24-02 CLI package extraction, and 24-03 Desktop adapter separation/integrated closeout are all delivered. See `.planning/phases/24-surface-boundary-split/24-CLOSEOUT.md`.

**Scope:** package/module cleanup, shared management adapter boundaries, shared ADM target/endpoint/local-lifecycle semantics, release/build naming clarity, surface-specific tests, CLI executable/package separation and Desktop connection/management/runtime separation inside the existing repository.

**Locked authority:** one Core/application model, one persisted desired state and one normal Admin MCP management plane. CLI/Desktop may keep surface-local narrow test interfaces but production uses the shared `internal/adminmcp.Client`; neither surface may silently fall back to writable local state.

**Plans:** `24-01` shared ADM target + management boundary; `24-02` CLI package extraction; `24-03` Desktop adapter separation + integrated closeout. See `.planning/phases/24-surface-boundary-split/`.

**Non-goals:** immediate multi-repo/module split, second state file/cache/API, duplicate business logic, remote auth/TLS, UI redesign, new product semantics, Phase-25 distribution polish or Phase-26 investigation helpers.

**Completion direction:** Desktop and CLI can evolve separately while still sharing one Core/Admin-MCP contract and release names `adm` / `adm-desktop`.

## Phase 25 — Distribution Polish If Needed

**Goal:** Add distribution features only when daily use proves they matter.

**Scope candidates:** installer, updater, signing, notifications, deeper autostart polish, release UX.

**Non-goals:** speculative packaging work before a real blocker.

**Completion direction:** open only with concrete post-1.0 dogfood evidence.

## Phase 26 — Evidence-first Investigation Expansion

**Goal:** Add deeper investigation helpers only when real development tasks show generic file/search/runtime tools are insufficient.

**Scope candidates:** symbol/reference/write tracing, impact evidence, data lineage, debug-SQL-to-code mapping, test metric explanation, symbol-scoped history.

**Non-goals:** mandatory code graph engine, speculative provider lock-in, ADM task planning.

**Completion direction:** each slice must return evidence, confidence and uncertainty, with fallback behavior when optional providers are unavailable.

## Review notes — 2026-09-11

The sequence is retained after source/contract review. Phase 19's bounded tree digest informs 20. Async verifier (21) has no hard dependency on 19/20 and may be explicitly reprioritized if the existing pjadm timeout blocker recurs. Phase 22 reuses the completed Phase 15 lifecycle/cleanup primitives; task-scoped UX must not create ADM task orchestration. Local frontend extraction required by 18 does not wait for the wider Desktop/CLI boundary cleanup in 24. Phases 25/26 remain conditional.

## Immediate next phase

Phases 18-24 are complete. Phase 25 Distribution Polish and Phase 26 Evidence-first Investigation Expansion both remain conditional/Standby; neither opens automatically. Continue bounded dogfood fixes only when concrete daily-use evidence justifies them.


## Dogfood checkpoint — 2026-09-12

After Phase 18 closeout and before Phase 19 execution, Desktop received a bounded dogfood fix: visual status/card polish plus exec_denials observations for Runtime allowlist rejections. The denial log records executable/count/source metadata only, not command args or output. Phase 19 remains planned and unstarted.

## Follow-up management UX backlog — 2026-09-16

### Bulk Environment MCP/Skill assignment

Environment MCP/Skill management needs a bulk operation flow for day-to-day administration. The intended interaction is resource-first: select one or more global MCPs and/or Skills, then select one or more target Environments, then apply one explicit action (`enable` or `disable`) across the selected resource × Environment set.

The UX should make mixed current state visible before applying changes and report per-Environment/per-resource results after the operation so partial failures are not hidden. Repeating the same enable/disable action should be safe and effectively idempotent. Desktop must continue to use the shared Core/Admin-MCP authority and persisted Environment selections rather than introducing Desktop-only assignment state.

**Desktop implementation — 2026-09-16:** MCP and Skill rows support stable-ID multi-selection, a shared dialog selects multiple target Environments, current selected-state counts are shown per Environment, and apply builds an idempotent resource × Environment plan that skips already-desired pairs and reports changed/unchanged/failed results per pair. The implementation deliberately reuses the existing per-resource Admin-MCP mutations instead of adding Desktop-only assignment persistence or a second management API.

### In-app environment variable refresh

ADM needs an explicit in-app environment-variable refresh action so changes to host/user environment variables can be picked up without requiring the operator to restart Desktop and CLI processes. The refresh must update the environment snapshot used for subsequent MCP secret/reference resolution and other operations that depend on host environment values, while preserving the rule that literal secret values are not persisted into ADM state.

The UX should report when the environment snapshot was last refreshed and make refresh failures visible. Exact platform semantics must stay explicit: refresh should read the current OS/user environment source supported by the platform rather than pretending a running process can automatically inherit mutations from an already-changed parent shell. Desktop and CLI should share the same Core/runtime refresh capability instead of implementing separate refresh state.

**Completion — 2026-09-16:** implemented through the shared host-environment manager, Admin MCP, CLI and Desktop. Refresh updates the snapshot used by subsequent Runtime/MCP resolution, reports source/status/timestamps/count without exposing values, and drops owner MCP sessions so future activations use the refreshed snapshot.

### Writer root-overlap exclusion

Writer authority must be exclusive across overlapping Environment roots, not only identical root strings. Two active Writer leases conflict when either Environment root is the same as, an ancestor of, or a descendant of the other root. Sibling roots remain independent and may hold Writers concurrently; for example `D:\projects\work\wm_group` and `D:\projects\work\wm_group1` do not conflict, while `D:\projects\work` and `D:\projects\work\wm_group` do conflict in both acquisition directions.

The comparison must use canonical path semantics already used by Environment path validation, including Windows case-insensitive comparison and path-segment boundaries; simple string-prefix checks are not sufficient. Reacquiring the same Environment with the same owner remains a lease renewal. A different Environment with an overlapping root must be rejected even when the owner string is the same, so one owner cannot accidentally hold two overlapping Environment authorities.

**Completion — 2026-09-16:** implemented and regression-covered. Writer acquisition now treats equal/ancestor/descendant Environment roots as one exclusive physical authority while sibling roots remain independent; expired leases stop blocking and same-owner renewal is limited to the same Environment.

### Environment Workspace rebind

Existing Environments can be attached to a broad Workspace even when a more specific registered Workspace contains their root. Add an explicit rebind workflow that only permits moving an Environment to a Workspace containing that Environment root. Desktop should support previewing/suggesting the most specific containing Workspace and preserve Environment explicit MCP/Skill selections while inherited Workspace selections are recalculated from the new Workspace. Managed-worktree logical source Workspace semantics must not be replaced merely by the physical ADM Worktrees storage Workspace.

**Completion — 2026-09-16:** delivered through Core, Admin MCP, CLI and Desktop. Candidate Workspaces are ordered from the most specific containing root to the broadest; migration remains explicit rather than automatic, system Workspaces are excluded, and managed-worktree logical source Workspace binding is protected.

**Batch follow-up — 2026-09-16:** ADM can now list only actionable Workspace-binding drift (`recommended != current`) across ordinary Environments. Admin MCP and CLI expose the read-only recommendation list, while Desktop provides an explicit preview/apply workflow. Batch apply still executes the existing per-Environment rebind mutation so every item is revalidated against current Workspace containment and managed-worktree/system-Workspace rules; partial failures remain visible instead of being hidden.

**Drift-prevention follow-up — 2026-09-16:** Desktop now checks the existing Workspace options immediately after a successful Environment create. When the explicitly selected Workspace is broader than the most-specific registered containing Workspace, Desktop opens the same rebind preview with the recommendation selected. Creation itself is never retargeted and no rebind mutation runs until the operator explicitly confirms, so the shared Core authority and explicit-choice semantics remain unchanged.

**Workspace-registration follow-up — 2026-09-16:** after Desktop successfully registers a new Workspace, it reuses the read-only recommendation list and filters it to Environments for which that newly added Workspace is now the most-specific recommendation. When matches exist, Desktop opens the batch preview immediately; no Environment is automatically rebound and the existing per-Environment mutation is still required for explicit apply.

### Unified product version source

Desktop previously hard-coded the Wails window title (`adm-desktop — 1.0 RC`) independently of the released product version. Replace surface-local version literals with one shared product version source used by Desktop title, Gateway/Admin MCP implementation metadata and CLI version output, with release/tag/build injection where appropriate. Future releases must not require manually finding and editing multiple version strings.

**Completion — 2026-09-16:** one linker-overridable `internal/version` source now drives Desktop title, Gateway/Admin MCP metadata and `adm version` / `adm --version`. Local source builds identify as `dev`; release/tag builds inject the requested tag through the shared CLI/Desktop build scripts and CI.
