---
gsd_state_version: 1.0
milestone: V2
current_phase: 24
current_phase_name: Desktop/CLI Surface Boundary Split
status: phase-24-complete
stopped_at: Phase 24 remains COMPLETE; bounded post-v1.2 dogfood management follow-ups are integrated on dev through Workspace hierarchy drift prevention; Phase 25/26 remain Standby/conditional
last_updated: "2026-09-16"
last_activity: 2026-09-16
last_activity_desc: Completed bounded post-v1.2 dogfood fixes on dev for overlapping Writer authority, Workspace rebind/recommendations, unified version reporting, and explicit Workspace hierarchy drift prevention
state_head: 905a879
current_plan: phase-24-complete
progress:
  total_phases: 26
  completed_phases: 24
  total_plans: 47
  completed_plans: 47
  percent: 92
---

# Project State

## Project Reference

See `.planning/PROJECT.md`, `.planning/ROADMAP.md`, and `.planning/rebaseline/2026-09-08-core-boundary.md`.

**Core value:** Give external Agents one reliable, inspectable and safe local development control plane for MCP, Skill and project Runtime capabilities.

**Explicit boundary:** ADM supplies capabilities, lifecycle, authorization and diagnostics. Agent/GSD supplies task planning/orchestration.

## Current Position

Phase: 24 — Desktop/CLI Surface Boundary Split
Status: COMPLETE. Phase 24 is closed through `9a4fea0` after shared target semantics, CLI package extraction, Desktop adapter separation and fixed-head integrated acceptance. Phase 25 remains Standby/conditional and is not opened automatically.
Baseline: clean `2b14a57` (`docs: add comprehensive v1.1 user guides`).
Current context: `.planning/phases/24-surface-boundary-split/24-CONTEXT.md`
Current executable plan: none. Phase 24 is complete; see `.planning/phases/24-surface-boundary-split/24-CLOSEOUT.md`.
Next candidate: Phase 25 Distribution Polish If Needed and Phase 26 Evidence-first Investigation Expansion remain Standby/conditional; continue only bounded evidence-backed dogfood fixes unless one phase is explicitly opened.
Active development branch: `dev`; `master` is the stable/release integration branch and is not advanced unless explicitly requested.
Phase 11 importer implementation: `ea0d85af99f4591a431ba22dd8f1df036c76cac6`
Phase 12 source-aware Skill implementation: `4074d3e8e5349ee717bf63ad027391d14579cecc`
Phase 12 Skill availability implementation: `8f46f6e019afc8722708e3a4478de06faa4b241a`
Phase 13 static capability report implementation: `b0eb0c74e6f43844b3a754feaa2d34c93bb82da6`
Phase 13 Gateway-owner capability report implementation: `46c86f3`
Phase 14 endpoint evidence resolver implementation: `0e40394`
Phase 14/15/16/17 priority planning: `9c4c539`
Phase 16 GitHub CI/RC baseline: `6d51e6d`
Phase 16 RC baseline closeout: `5a96fe4`
Phase 16 Desktop management adapter APIs: `374fa12`
Phase 16 Desktop MCP/Skill visual management UI: `74be256`
Phase 16 Agent/Admin MCP surface split: `2c6ab1c`
Phase 16 Desktop ADM connection profiles: `cd191ff`
Phase 16 Desktop Admin MCP convergence: `0dfe01d`
Phase 16 CLI Admin MCP convergence: `ecf0516`
Phase 16 Windows default Gateway port fix: `65396ef`
Phase 16 local Gateway bind preflight: `28bf1ac`
Phase 16 Desktop Runtime visibility: `19808f7`
Phase 16 RC1 dogfood blocker fixes: `8dbe817`
Phase 16 MCP/Skill Desktop UI dogfood polish: `a3478f2`
Phase 16 generic mcpServers import fix: `c52d584`
Phase 16 Windows path canonicalization: `11c49ee`
Phase 16 Desktop tray/autostart polish: `53300d8`
Phase 16 saved connections/modal editors/icons: `41a161a`
Phase 16 packaging/docs/tray sizing polish: `abafcc0`
Phase 16 Windows canonical path CI fix: `73c4481`
Phase 16 tray event loop fix: `5c8574e`
Phase 16 release automation: `559aa3d`
Phase 15 lifecycle cleanup implementation: `48b0858`
Phase 15 closeout validation: `04d1bba`
Phase 17 local distribution standby validation: `9fc4c4b`
Run observability mitigation: `6b793d3`
Phase 17 run-observability artifact refresh: `da5d2ea`
ADM v1.0.0 pre-release validation head: `da5d2ea`
ADM v1.0.0 closeout: `b57efb9`
ADM v1.0.1 Windows shutdown race hotfix: `b73b749`
ADM v1.0.1 green release record: `c898586`
Post-1.0 phase map base: `c898586`
Post-1.0 phase map commit: `0173259`
Phase 18 scope review checkpoint: `d95b088`
Phase 18 initial design/18-01 plan: `93a74d9`
Phase 18 detailed 18-02 plan: `b6da7fc`
Phase 18 detailed 18-03 plan: `f52b872`
Phase 18 closeout: all 18-01 through 18-04 work is complete and accepted. Final native evidence is recorded in `18-CLOSEOUT.md`, `18-04-SUMMARY.md` and `evidence/18-04-native-final-ui-acceptance.json`.
Phase 19 Workspace Discovery + Project Navigation is complete through 19-02 integrated acceptance; native GUI click-through is pending under B09 availability semantics.
Phase 18 design: `.planning/phases/18-desktop-management-ux/18-UI-REFACTOR.md`
Phase 18 context / acceptance / durable planning log: `18-CONTEXT.md` / `18-VALIDATION.md` / `18-PLANNING-LOG.md` in the same phase directory
Implementation status: Phases 18-24 are complete. Phase 24 closed at `9a4fea0` after shared ADM target/management boundary cleanup (`a96558c`), internal CLI surface extraction (`810d6bc`) and Desktop adapter separation/integrated acceptance. No Phase-24 product feature semantics were introduced.

`state_head` records the inspected head before this planning update, not a self-referencing final commit hash; use Git log for the current head.

Plan-count convention: count NN-NN-PLAN.md in active phases, excluding phase-00 and dogfood follow-ups. 44 plans were complete through Phase 23; Phase 24 added three execution nodes. All 47 plans through Phase 24 are complete. Completed phases are 24/26 (92%); Phases 25-26 remain conditional/Standby.

The prior `feat/gsd-phase-executor` branch is abandoned and must not merge. Its planning/provenance/state-advance implementation is not ADM product scope.

## Retained Core History

### Phases 1-4 鈥?Development foundations

- real `SKILL.md` discovery/read from explicit roots with support-root containment and Environment gating;
- structured optional verifier runtime;
- real external MCP Streamable HTTP tool discovery/call with Environment gating, four-state health and activation-boundary secret resolution;
- external Agent dogfood through ADM-only local development capabilities.

### Phases 5-8 鈥?Persistent local Runtime

- persistent Gateway owner and external MCP session restart reconciliation;
- long-running dev process lifecycle, bounded logs and owned-port facts;
- optional managed Git worktree isolation;
- generic single-command asynchronous `run_` start/list/status/cancel lifecycle.

These capabilities remain ADM Core.

## Superseded Work

### Phase 9 鈥?Planner / Executor / Reviewer

Phase 9 was technically implemented, reviewed and integrated to local master, but the product rebaseline determined that its workflow orchestration semantics belong to the Agent/GSD/orchestrator layer.

The Git/evidence history remains for auditability. The Agent-facing workflow surface and workflow-specific domain code are cleanup scope under BOUNDARY-01. Future ADM capabilities must not depend on FLOW-01.

### Abandoned Phase 10 GSD executor branch

`feat/gsd-phase-executor` is not to be merged. `gsd_phase_inspect`, `gsd_phase_start`, `gsd_phase_advance`, `.planning` provenance interpretation and STATE advancement are explicitly outside ADM Core.

## Active Requirements

### Phase 10 鈥?Boundary cleanup (complete)

- BOUNDARY-01 鉁? Planner/Executor/Reviewer workflow surface/domain is removed from ADM Core while generic single-command Runs remain.
- BOUNDARY-02 鉁? no GSD `.planning` interpretation/state-advance API was merged or introduced.
- BOUNDARY-03 鉁? files/exec, verifier, process, MCP, Skill, managed worktree, ordinary Environment and generic Run regression gates passed.

### Phase 11 鈥?MCP runtime completion (complete)

- 11-01 鉁? typed desired MCP configuration, activation-only references and real Streamable HTTP/stdio owner-bound transport runtime.
- 11-02 鉁? owner-local health/recovery observation, configurable probe/reconnect policy, inventory/inspect/refresh, stable-ID update invalidation and safe no-replay semantics; full test/vet/race/diff gates passed at `4c4f4dc`.
- 11-03 鉁? external JSON/JSONC preview/apply import adapters, batch atomicity, conflict policy, credential-reference conversion, Gateway/management/CLI surfaces and runtime-owner definition-fingerprint/generation reconciliation; full `go test -count=1 ./...` plus vet/race/diff gates passed at `ea0d85a`.

### Phase 12 鈥?Skill runtime completion (complete)

- 12-01 鉁? persisted Skill sources, source/artifact stable identity, atomic per-source refresh, safe failed-refresh preservation, unresolved selection preservation and Gateway/management/CLI source surfaces; full test/vet/race/diff gates passed at `4074d3e`.
- 12-02 鉁? Environment-specific Skill availability, bounded artifact/support inventory, structured Skill read diagnostics and broken-Skill isolation; full test/vet/race/diff gates passed at `8f46f6e`.

### Phase 13 鈥?Environment capability diagnostics (complete)

- 13-01 鉁? shared `CapabilityReport`/`CapabilityFact`/`CapabilityEvidence` model and resilient side-effect-free application-level Environment report with static file/exec/verifier/Git/isolation/MCP/Skill/process/run facts; full test/vet/race/diff gates passed at `b0eb0c7`.
- 13-02 鉁? Gateway-owner observation enrichment and canonical Agent-facing `environment_capability_report`, plus CLI/management wrappers; full test/vet/race/diff gates passed at `46c86f3`.

### Phase 14 鈥?Evidence-first Investigation Toolkit (slice complete / later helpers deferred)

- 14-01 鉁? endpoint evidence resolution for URL/path plus optional HTTP method; returns bounded static route evidence, confidence and uncertainties; full test/vet/diff gates passed at `0e40394`.
- 14-02 鉁? optional code intelligence provider + GitNexus integration boundary uses existing Environment MCP/runtime authorization, passive Gateway-owner observations, explicit provenance/freshness/uncertainty and static fallback; focused app/Gateway plus full test/vet/diff gates passed before the later lifecycle closeout.
- Additional evidence-first slices remain deferred by default and do not block Phase 15.

### Phase 15 鈥?Temporary Resource Lifecycle (complete)

- 15-01 鉁? explicit durable/temporary retention metadata, cleanup inspect/dry-run/execute, mark/promote, Skill/MCP safe cleanup and retention lifecycle surfaces landed at `fc8303d`.
- 15-02 鉁? temporary Environment state-only cleanup, Gateway-owner managed worktree cleanup through existing destroy safety, and bounded Environment capability inspection to avoid unbounded disabled catalog facts landed at `48b0858`; rebuilt Gateway live dogfood confirmed `skill.catalog` summary with `suppressed_disabled_fact_count` instead of per-disabled-Skill facts.

### Phase 16 鈥?Desktop Core Parity + 1.0 RC Readiness (complete)

- 16-01 鉁? Desktop parity matrix, RC gate and GitHub Actions CI build baseline completed at `6d51e6d`; closeout recorded at `5a96fe4`.
- 16-02 鉁? Desktop MCP/Skill visual management UI completed through adapter API commit `374fa12` and UI commit `74be256`; full repository test/vet/build/diff gates passed. Human Wails click-through continues as RC1 dogfood.
- 16-03A 鉁? Agent/Admin MCP surface split completed at `2c6ab1c`. `/mcp` now excludes Admin-only management tools; `/admin/mcp` exposes the privileged management superset over the same Core/runtime owner.
- 16-03B 鉁? Desktop connection profile + configurable health/liveness completed at `cd191ff`; Base URL derives health/Agent/Admin MCP endpoints and local bootstrap is loopback-only.
- 16-03C 鉁? Production Desktop normal management moved to Admin MCP at `0dfe01d`; disconnected/stopped ADM clears the management backend and never silently falls back to writable local state.
- 16-03D 鉁? Normal CLI `workspace`/`environment`/`exec`/`mcp`/`skill`/`memory` management converged on the shared Admin MCP client at `ecf0516`; `--adm-url` / `ADM_V2_URL` select the management target and connection failure never falls back to writable local state. `gateway`/`doctor`/`state` remain explicit local bootstrap/offline/recovery paths.
- 16-03E 鈴革笍: Authenticated/TLS-safe non-loopback remote Admin MCP is intentionally deferred post-RC.
- RC1 鈿狅笍: dogfood exposed that the published Desktop artifact was incorrectly produced by raw `go build` and therefore failed Wails build-tag validation at launch; it also exposed that Gateway lifecycle did not honor `--adm-url` / `ADM_V2_URL`. RC1 is superseded by RC2.
- RC2 鈿狅笍: Wails build/CI, Gateway target convergence and screenshot-driven MCP/Skill UI polish were fixed, but dogfood later exposed generic top-level `mcpServers` auto import as unnecessarily ambiguous; RC2 is superseded by RC3.
- RC3 鉁? canonical `generic-mcpservers` auto import landed at `c52d584`; literal imported env/header values remain reference-only, Desktop preview shows generated reference requirements, Admin MCP acceptance passed, exact Wails Desktop launch smoke passed, and full `go test -count=1 ./...` / vet / diff gates passed.
- 16-04 鉁?local code/gates: Windows short/long path canonicalization landed at `11c49ee`; Desktop UI/tray/autostart polish landed at `53300d8`; full local tests/vet/diff/Wails build passed. Tray menu and real login acceptance remain manual.
- 16-05 鉁?local code/gates: saved ADM connection profiles, modal child editors and ADM application/tray/window icon pipeline landed at `41a161a`; full tests/vet/diff/Wails build plus hidden/single-instance smoke passed. Visual/modal and tray/autostart click-through remain manual.
- 16-06 DONE local code/gates: ime-lock-v2 tray lifecycle alignment, fitted tray icon sizing, simplified `adm` / `adm-desktop` user-facing names, unified `dist/` packaging, tag-aware GitHub artifacts and README/docs split landed at `abafcc0`; full tests/vet/diff/default Wails build and RC packaging smoke passed.
- 16-07 DONE closeout: Windows CI short/long path test compatibility landed at `73c4481`; tray left/right click dispatch was fixed by keeping the Win32 tray message loop on one OS thread at `5c8574e`; tag-triggered GitHub Release automation landed at `559aa3d`; user accepted the tray behavior and closed Phase 16.

### Phase 17 鈥?Distribution Only If Needed (complete for stable 1.0 line)

- `v1.0.0` was tagged and pushed, but its remote release workflow failed on a Windows shutdown connection-reset race in nested Gateway acceptance.
- `v1.0.1` fixed the shutdown race at `b73b749`, recorded the hotfix at `c898586`, and completed the GitHub Actions Release workflow successfully.
- Installer/updater/signing/notifications remain conditional post-1.0 work. Tray/autostart were pulled into Phase 16 by dogfood requirements and are implemented locally.

### Post-1.0 phase map

See `.planning/post-1.0/PHASE-MAP.md` for the high-level phase sequence. Detailed execution plans are intentionally created only when a phase starts.

- Phase 18 鈥?Desktop Management UX Reorganization: 18-01 is accepted through exact-artifact native Wails/WebView2 evidence. 18-02 is the next executable plan, followed by detailed 18-03 and mandatory integrated 18-04 acceptance/closeout.
- Phase 19 — Workspace Discovery + Project Navigation: complete. Bounded metadata-only discovery/digest is exposed through Core, Agent/Admin MCP, CLI and explicit Desktop navigation/root handoff; native GUI click-through remains pending under B09 availability semantics.
- Phase 20 — Agent Context Bundle + Capability Injection: complete. Explicit bounded Environment context, passive Gateway-owner MCP/tool-name enrichment, shared Agent/Admin tool and static stable-ID guidance are delivered with no hidden current-Environment state.
- Phase 21 — Async Verifier + Long Operation Observability: complete. Owner-local `vfrun_` lifecycle, shared Agent/Admin tools, blocking-request interruption diagnostics, passive context guidance and fixed-head integrated acceptance are delivered.
- Phase 22 鈥?Temporary Task Environments + Safe Cleanup Workflow: COMPLETE through Core lifecycle, shared Agent/Admin workflow, Desktop Admin-MCP visibility/actions and fixed-head integrated acceptance.
- Phase 23 鈥?CLI Agent UX + MCP/Skill Provisioning: COMPLETE through MCP/Skill provisioning, Environment context + temporary lifecycle CLI, and fixed-head integrated acceptance.
- Phase 24 — Desktop/CLI Surface Boundary Split: COMPLETE at `9a4fea0`; shared target semantics, CLI package extraction, Desktop separation and integrated acceptance are closed.
- Phase 25 鈥?Distribution Polish If Needed: standby. Installer/updater/signing/notifications only after concrete dogfood need.
- Phase 26 鈥?Evidence-first Investigation Expansion: standby. Add deeper helpers only when generic Runtime/search is insufficient.

### Next Core priorities

1. Phase 24 is complete at `9a4fea0`; do not perform further Phase-24 product work unless a regression is found.
2. Keep Desktop and CLI as presentation surfaces over one Core/Admin MCP authority: no second state, protocol, product validation or writable local fallback.
3. Preserve Phase 15/22 cleanup safety unchanged: active writers/processes/runs/`vfrun_`, dirty or unpublished managed worktrees, unknown ownership and insufficient evidence continue to block cleanup.
4. Phase 24 is refactor-only from the product perspective; any user-visible MCP/Skill/Environment/Memory/Runtime behavior difference is a regression unless documented as an existing blocker.
5. Do not open Phase 25 automatically after Phase 24; distribution polish remains conditional.

## Product Decisions

- Phase 18 Skill management keeps Skills and Sources as local Desktop subviews over already loaded data: switching/filtering those views does not read Skill artifacts, refresh sources, execute Skills or imply Agent prompt injection.
- MCP and Skill are core product capabilities and must stay visible in the main roadmap.
- Phase 20 keeps stable explicit Environment routing: there is no hidden per-session current Environment. The delivered Agent context path is one explicit bounded `environment_context_bundle(environment_id)` plus static non-project-specific MCP server instructions. Existing Gateway-owner observations may supply MCP tool-name summaries, but bundle generation does not probe/connect. Memory values and full Skill instructions remain behind explicit read operations rather than being silently injected by default.
- Phase 21 planning introduces a distinct owner-local `vfrun_` verifier lifecycle rather than aliasing generic `run_`. Start/cancel remain writer-gated, list/status are read-only, execution reuses the existing verifier/Runtime authority and `verifier.Classify`, live output is bounded, observations are not persisted or resumed, and blocking verifier calls are not rejected by an arbitrary duration threshold. Caller interruption should produce a clear async-verifier diagnostic without silently continuing/retrying work.
- Phase 22 planning reuses Phase-15 retention rather than introducing a task model: temporary Environments are normal `env_` resources created atomically with owner + explicit TTL; optional run/session attachment is provenance only. Existing-root cleanup is state-only, managed-worktree cleanup must reuse non-force destroy safety, targeted cleanup never sweeps unrelated resources, active `vfrun_` joins process/run/MCP/writer cleanup blockers, and matching-owner promotion changes retention only. Git remains optional outside managed-worktree mode.
- A foundation vertical slice is not the same as completion. Phase 3 does not mean MCP is finished; Phase 1 does not mean Skill is finished.
- `run_` is retained only as generic asynchronous Runtime lifecycle; it must not grow task semantics.
- Worktree remains an optional isolation primitive. ADM does not orchestrate parallel Agents or choose integration policy.
- Verifier returns structured evidence; the external Agent/orchestrator decides what that evidence means for its task/phase.
- No LLM/GSD/OpenCode subagent quota is used unless the user explicitly reverses the existing instruction.
- No automatic push at review boundaries.
- During active development, continue on `dev`; keep `master` stable for explicit integration/release. Make a commit at each clear node and do not merge/push/tag/release unless explicitly requested.
- Phase 11 transport scope is Streamable HTTP + stdio; legacy HTTP+SSE is not added. Stdio MCP executable launch must still obey ADM's executable allowlist.
- Phase 11 health-check interval/probe timeout are configurable per MCP. Automatic reconnect defaults to off; when enabled it retries at one fixed configured interval with no exponential/adaptive backoff. Health/recovery observation is owner-local, and recovery never replays failed tool calls.
- Phase 11 supports preview + atomic single/batch JSON/JSONC import adapters for OpenCode, WorkBuddy/CodeBuddy, Codex plugin MCP JSON, Claude Code and supported MCPHub shapes. Name conflicts default to error. Import writes only global MCP definitions, never existing Environment selections, and converts literal credential-bearing values into secret/environment-reference requirements instead of persisting them.
- Phase 11 does not persist interactive OAuth tokens until an approved secure credential lifecycle exists; supported HTTP auth is none or explicit secret-backed headers.
- Phase 12 uses persisted Skill sources, source/artifact-based stable identity and explicit atomic refresh; ADM does not interpret Skill instructions.
- Phase 12 availability is Environment-specific; a broken Skill is local to that Skill and does not poison unrelated Skill, MCP, file, verifier or Runtime operations.
- Phase 13 capability inspection is side-effect-free by default and aggregates per-capability facts instead of probing/executing optional tools.
- Phase 13 Gateway-owner enrichment reads existing owner-local observations only; it does not reconnect, Ping, refresh inventory, call MCP tools, run verifiers, start processes, or acquire writer leases.
- Phase 14 investigation helpers must return concrete evidence, confidence and uncertainties. 14-01 endpoint investigation is static literal/dynamic-candidate search only and does not execute code or call endpoints.
- GitNexus is an optional evidence provider through existing Environment MCP/runtime boundaries; ADM does not embed a mandatory code graph engine, and passive provider inspection must remain side-effect-free with static fallback.
- Phase 15 temporary resource lifecycle/retention is complete; CLI/UI-created resources remain durable by default, and temporary cleanup must remain explicit, inspectable and conservative.
- Phase 16/17 completed the stable 1.0 line. GitHub Actions Release automation is retained release infrastructure, not speculative polish.
- Desktop must remain a management surface over Core, with no Desktop-only state or authorization model.
- Normal management uses the separate Admin MCP management plane. Agent/Admin MCP privilege separation is implemented; production Desktop and normal CLI management have converged on the shared Admin MCP client. Direct writable state-file access is not a peer normal mode; retain it only as explicit offline/bootstrap/recovery behavior, preferably read-only until cross-process locking and service-stopped safety are designed.
- Desktop connection configuration should support explicit scheme/host/domain/port health checks. Non-loopback remote Admin MCP must remain disabled until authentication/TLS/Host-boundary semantics are defined.
- Phase 18 first UI priority is Desktop management UX reorganization: menu shell, dashboard, routing and clearer management sections before deeper page-specific redesign.
- 18-01 uses ten real routes plus an Environment diagnostics shortcut, preserves the connected Admin MCP path and existing dialogs, distinguishes unloaded/zero/stale/error, isolates auxiliary read failures and rejects old-scope results.
- 18-02 refines Workspace/Environment and existing Runtime views; 18-03 refines MCP/Skill, explicit Memory scopes, existing system controls and diagnostics; 18-04 is mandatory integrated acceptance with only evidence-backed fixes.
- Future Phase 22 UI/workflow reuses the completed Phase 15 lifecycle/cleanup primitives. Local frontend extraction in 18 does not require the broader Phase 24 package split.
- Important Phase 18 task/check/commit progress must be written to planning before further work when quota/context is low; chat is not the continuation record.
- Phase 25 distribution polish remains conditional unless daily use proves it is necessary.

## Deferred

- automatic Memory context composition;
- automatic MCP/Skill/Memory Agent context injection or prompt composition before the approved Agent Context Bundle phase;
- additional evidence-first investigation slices after 14-02;
- broad Desktop feature expansion outside the approved Phase 18 UX reorganization;
- installer/updater/signing/notifications unless Phase 25 is opened by dogfood;
- migration/compatibility burden.


## 2026-09-12 Global Memory route UX checkpoint

After `903004f`, fixed the Memory management page behavior: successful ADM snapshot refresh now restores the Global Memory panel to `尚未加载 Global Memory` instead of leaving a stale disconnected message, and entering the Memory route lazily loads Global Memory values for the human management view. This does not implement Agent automatic Memory context injection; storage/admin management remains separate from future Agent Context Bundle work. Evidence: `node --check` for `app.js` and `browser-smoke.cjs` PASS, helper regression 20/20 PASS, Chromium smoke 89 checks x 3 PASS, `go test -count=1 ./cmd/ai-dev-manager-desktop` PASS.

## 2026-09-12 Skill/MCP management UX checkpoint

After `746ed8c`, implemented user-requested Skill/MCP management refinements: Skill Sources can be edited without implicit refresh; Skill and MCP search supports literal text or `/pattern/flags`; both pages add a current-Environment-unselected filter; and filtered bulk toggles apply only to the currently visible result set for new-Environment defaults and current-Environment enablement. Backend support adds Admin-only `skill_source_update` and Desktop adapter plumbing while keeping refresh explicit. Evidence: JS syntax checks PASS, helper regression 20/20 PASS, Chromium smoke 91 checks x 3 PASS (Chrome selected because local Edge returned empty dump-dom output), focused Go async run `run_208c048021def316` PASS, and `git diff --check` PASS.

## 2026-09-12 Checkpoint

User-priority correction implemented after `073fab1`: Skill bulk availability now uses global catalog structural checks (`skill_availability_list`) rather than current Environment enabled state; Desktop connection profiles now support an explicit active-profile `start_service_on_desktop_launch` option that calls local loopback `StartLocalADM` during Desktop startup and then refreshes Admin MCP data. Verified before commit: `node --check` for app/connections/skill-bulk/browser-smoke, helper regression 20/20 PASS, focused Go async run `run_d17bf3d3621b5684` PASS, Chromium smoke 82 checks at three sizes/scales PASS, and `git diff --check` PASS.

## Session Continuity

Current instruction: Phase 24 remains COMPLETE at `9a4fea0`; bounded post-v1.2 dogfood work continues on `dev`. Latest delivered hierarchy management nodes include `1a0695b` Workspace-binding recommendations, `25eacb3` post-create specific-Workspace suggestion and `905a879` post-Workspace-registration drift surfacing. Preserve normal CLI/Desktop Admin-MCP-only management/no writable local fallback, explicit stable Environment IDs, MCP probe/status/inspect/refresh side-effect distinctions, Skill source update separate from refresh, Phase-20 passive context boundaries, Phase-22 matching-owner/non-force cleanup safety, optional Git, and the no-task/GSD-orchestration boundary. Keep Phase 19 B09 and Phase 22 native GUI evidence pending unless real native GUI-control evidence exists. Phase 25/26 remain Standby/conditional and must be opened only by an explicit planning decision backed by concrete dogfood need. Keep `master` stable; no merge/push/tag/release or subagents unless explicitly requested.

Historical Phase 18 acceptance and dogfood evidence remains in its phase directory and the dated checkpoints below. Do not resume old 18-02 instructions from historical summaries.

## 2026-09-12 Phase 18 closeout

Phase 18 is complete. Final evidence: helper regression 20/20 PASS, production browser smoke 131 checks x 3 PASS, focused Go PASS, full Go `run_60ca8af474ac99d3` PASS, vet `run_d1f5e27c8f11f6dc` PASS, Wails build `run_401fb4c5bf51b1d3` PASS, and visible native Wails/WebView2 evidence for `cmd/ai-dev-manager-desktop/build/bin/adm-desktop-phase18-final-windows-amd64.exe` PID 13540. Dist final-name overwrite was blocked by the active Gateway child file lock, so the closeout records an artifact-path note rather than pretending the dist path was replaced. Next candidate: Phase 19 Workspace Discovery + Project Navigation; not started.

## 2026-09-12 Post-Phase-18 dogfood checkpoint — UI status polish and exec denial observations

User feedback after Phase 18 closeout showed that the ADM connection / system pages still had uneven spacing, weak hierarchy and plain-text status presentation. A post-18 dogfood fix was implemented before starting Phase 19: shared status badges now have clearer color/icon treatment, the ADM connection endpoint/profile/lifecycle groups have more consistent card spacing, and the Exec allowlist page separates allowed executables from blocked executable observations.

New execution-policy behavior: Runtime allowlist rejections now persist lightweight exec_denials observations in ADM state. The record includes executable, count, first/last blocked timestamps, last Environment ID, last surface and reason. It intentionally does not persist args, stdout/stderr or command payloads. Recording is wired through direct exec, verifier execution, async run start, process start and stdio MCP probe/proxy preparation. Management/Admin MCP/Desktop expose list/clear/clear-all operations; allowing a blocked executable uses the existing allow operation and clears the matching observation. The Desktop page sorts observations by denial count so a human can decide whether to allow or ignore a command.

Evidence before commit: JS syntax PASS; helper regression 20/20 PASS (run_b28e6510913d1292); production browser smoke 133 checks x 3 PASS (run_972c5ad61b3b9231); focused Go PASS (run_60e3bcb87c38274e); full Go PASS (run_dd2461626056a5e5); vet PASS (run_61031fb90544aeb5); Wails unique artifact build PASS (run_c90e1ee10fa06433), dist/adm-desktop-phase19-exec-denials-windows-amd64.exe, 17,399,808 bytes, SHA-256 C4EE388D62F8F1283A4F730522E992B5E3E5DE0B8489F522DA9344F4D0F648B7. Phase 18 remains complete; Phase 19 remains the next planned phase and is not started by this dogfood fix.


## 2026-09-12 Screenshot-driven Desktop spacing follow-up

Baseline e180cb9 confirmed on clean master. User screenshots reprioritized a bounded Settings/lifecycle/MCP layout fix before Phase 19 planning. Added padded card bodies and neutral explanatory notes; corrected the lifecycle CSS specificity conflict; MCP action rows now respond to actual content width. No Core or authority changes. Browser run_f8c4f7866b9fadda PASS (149 checks x 3), Wails run_4cbae173cf05abdd PASS. Artifact: dist/adm-desktop-spacing-fix-windows-amd64.exe; SHA-256 227D1D1E9AFF39B85F297EDFB1D7CDDDEB9C4772CCFF3BBD96782F08E17E4E1B. Native visual acceptance pending. Detailed scope/evidence: .planning/dogfood/2026-09-12-desktop-spacing.md. Phase 19 has no executable plan yet and remains unstarted. Skill global structural availability correction remains authoritative over older Environment-specific management wording above. No push/tag/release.


## 2026-09-12 Global MCP management probe correction

After be591d4 the user clarified that MCP availability/probing must be global and independent of Environment enablement. Implemented Admin-only mcp_probe, shared catalog resolution/transport, allowlisted stdio in a temporary working directory, management/client/Desktop plumbing and CLI mcp probe. Desktop global observations survive Environment changes and reject late results after definition/connection changes. Agent environment_mcp_* authorization remains unchanged. This correction supersedes older management-probe coupling above; existing Environment runtime diagnostics remain valid.

Evidence: browser run_332e94fe1d8355d0 PASS (153 checks x 3); full Go run_0d807dad79377756 PASS; vet run_89539e70556df480 PASS; Wails run_f3ee50c6a27dc6db PASS; CLI build run_09c0269551f02759 PASS. Detailed plan and evidence: .planning/dogfood/2026-09-12-global-mcp-probe-PLAN.md and -SUMMARY.md. Artifacts: dist/adm-desktop-global-mcp-probe-windows-amd64.exe and dist/adm-global-mcp-probe-windows-amd64.exe. The active Gateway has not been restarted; update it as well as Desktop before live use of the new mcp_probe API. Native window acceptance pending. Phase 19 remains unstarted. No push/tag/release.


## 2026-09-12 MCP probe badge consistency correction

After 5ca528f, user screenshot exposed configured badges beside unresolved_secret_reference diagnostics. Fixed frontend precedence: configuration shows 引用未解析/unavailable, global probe shows 受阻/error; toast agrees, issue filtering includes it, and successful reprobe clears stale error presentation. Browser run_9b55aa7e0e101775 PASS (158 checks x 3), Desktop Go run_5ba5a925268b56e8 PASS, Wails run_eec60c6b96e4d64f PASS. Artifact: dist/adm-desktop-mcp-badge-fix-windows-amd64.exe; SHA-256 57D4EF6862F0BF1556D21C833D08601BC94059FDA93509CE3A6BC0B76F6EEBC0. Details: .planning/dogfood/2026-09-12-mcp-probe-badges.md. Native visual acceptance pending. No backend or environment-reference values changed. Phase 19 remains unstarted. No push/tag/release.


## 2026-09-12 Phase 19 detailed planning checkpoint

Prepared 19-CONTEXT.md, 19-01-PLAN.md (bounded Core discovery/digest) and 19-02-PLAN.md (Agent/Admin/CLI/Desktop and explicit Environment handoff). Scans stay within registered roots, have hard budgets and explicit partial evidence, and read metadata only; no new prerequisites, indexing daemon, hidden scan or automatic creation. No implementation tests/builds run in this documentation-only checkpoint. Plan confirmation precedes 19-01 feature work per the user's initial handoff. Baseline 5c30979; local planning commit identified by Git log. No push/tag/release.

## 2026-09-12 Phase 19 closeout

Phase 19 is complete through final implementation commit `8a5c99a`. 19-01 delivered bounded metadata-only Workspace discovery and Environment tree digest; 19-02 exposed the same Core through Agent/Admin MCP, normal CLI and explicit Desktop discovery/root handoff with stale-response guards and explicit Environment directory summary. Final evidence: full Go `run_4df936e2226f72ec` PASS; vet `run_f84bb4ef0561b86d` PASS; production browser smoke `run_105573529a6a9764` PASS with 184 checks at each of 1120x760, 820x560 and 125% scaling; CLI build `run_c8a7c2806498667f` PASS; Wails v2.15.0 production build `run_88c7536254ea3e9d` PASS. CLI SHA-256 `84f2ef07f0425a6e8ff3bdcf31c0bda42732adb86e2ba4834b0b8a8ba00cd17d`; Wails SHA-256 `2564136b5d2a3bb375a24b96e30731d3eb5744e5b4e9324a09ba3f1e2253d210`. Native Wails/WebView2 click-through is recorded pending under B09 because no native GUI-control tool was available; the active Gateway was not replaced or terminated for screenshots. See `19-02-SUMMARY.md` and `19-CLOSEOUT.md`. Phase 20 was not started. No push/tag/release.

## 2026-09-12 Phase 20 detailed planning checkpoint

From clean `89be773`, opened Phase 20 Agent Context Bundle + Capability Injection after explicit user direction to continue. Prepared `20-CONTEXT.md`, `20-01-PLAN.md` and `20-02-PLAN.md`. The plan preserves stable explicit Environment IDs: no hidden current-Environment session state. Dynamic context is an explicit bounded `environment_context_bundle` call; MCP initialize instructions are static usage guidance only. Bundle generation reuses Phase 19 tree digest and Phase 13 capability facts, may include passive Gateway-owner MCP tool-name observations, and performs no probes/execution/mutation. Memory values and full Skill instructions are not silently injected by default; existing explicit reads remain authoritative. No feature code/tests/builds were run in this docs-only checkpoint. No push/tag/release or subagents.

## 2026-09-12 Phase 20-01 Core completion

Implemented the shared bounded Environment context bundle from planning commit `b0e675d`. The Core bundle composes stable Environment/Workspace identity, Phase-19 tree digest, passive capability summaries, safe MCP/Skill/verifier summaries and factual guidance without acquiring/renewing a writer, executing Git/verifiers/processes/Runs, connecting to MCPs, or injecting Memory/Skill contents. A real HTTP MCP fixture observed zero requests. Final evidence: focused context/discovery PASS; context repeat stress x3 PASS; full repository `run_5f025df7d6c4d3f0` PASS with Gateway 93.606s; vet `run_6fbbc979ec9b272c` PASS; diff checks PASS before commit. An earlier full run hit a Desktop loopback timing flake that passed repeated standalone checks and the final full rerun. See `.planning/phases/20-agent-context-bundle/20-01-SUMMARY.md`. 20-02 is complete as recorded below. No push/tag/release or subagents.

## 2026-09-12 Phase 20 closeout

Phase 20 is complete through implementation head `20ae96d`. 20-02 added passive Gateway-owner context enrichment, bounded observed MCP tool names, shared Agent/Admin `environment_context_bundle` and static MCP initialize guidance. Fake MCP context calls produced zero upstream requests both before and after a controlled owner observation; 160 long observed tool names were capped at 128 with 32 explicit omissions; Memory/Skill sentinels and verifier side effects remained absent; existing writer expiry/last-seen and owner resource counts were unchanged. Final evidence: focused run `run_456774d2120b3a8f` PASS; full Gateway `run_0be1e9a1f9f95c94` PASS at 109.990s; full repository `run_34634311551124f4` PASS with Gateway 108.134s; vet `run_ec2cf832e35190a6` PASS. Fixed-head artifact `ai-dev-manager-phase20-02-20ae96d.exe`, 16,817,152 bytes, SHA-256 `79F1377506E59EEBA8EF643999822191937AF13C1A70556DC579064564110F97`. See `20-02-SUMMARY.md` and `20-CLOSEOUT.md`. Phase 21 is planned and not started. No push/tag/release or subagents.

## 2026-09-12 Phase 21 detailed planning checkpoint

From clean `5e3f67a`, opened Phase 21 Async Verifier + Long Operation Observability after explicit user direction to continue. Added `ADM-CORE-021` to the product contract and prepared `21-CONTEXT.md`, `21-01-PLAN.md` and `21-02-PLAN.md`. The design keeps async verification as a distinct owner-local `vfrun_` resource: it reuses verifier definition/Runtime authority, writer heartbeat, configured timeout and `verifier.Classify`; exposes bounded live output; is observable across client disconnects while one Gateway owner lives; cancels on matching-writer request, Environment drop or owner shutdown; and is never persisted/resumed across restart. Existing generic `run_` and synchronous verifier remain intact. Blocking verifier diagnostics improve only for real caller/request interruption; no arbitrary long-duration rejection, automatic background conversion, retry or CI/task orchestration is introduced. Planning commit: `2aceec3`. No push/tag/release or subagents.

## 2026-09-12 Phase 21-01 owner lifecycle completion

Implemented the owner-local async verifier lifecycle from planning commit `2aceec3`. Shared `PrepareVerifierExecution` now centralizes verifier definition/enabled/writer/Runtime/allowlist/cwd/timeout authority for synchronous and owner-local execution. Gateway owner now owns stable `vfrun_` resources with bounded live output, classified terminal results, writer heartbeat, matching-writer cancel, Environment-drop and owner-close cleanup, and no persistence/restart resurrection. Generic `run_` semantics remain unchanged after extracting the shared owner output buffer. Final evidence: focused combined `run_b9a69b74dd3c3d07` PASS; full repository `run_b59177d17a9620a5` PASS with Gateway 112.957s; vet `run_8666a6eb51de25d9` PASS; `TestVerifierRun*` repeat x3 PASS; diff check PASS. See `.planning/phases/21-async-verifier-observability/21-01-SUMMARY.md`. 21-02 is complete as recorded below. No push/tag/release or subagents.

## 2026-09-13 Phase 21 closeout

Phase 21 is complete through implementation head `3ee045c`. 21-02 exposed shared Agent/Admin async verifier start/list/status/cancel tools, added stable `blocking_request_interrupted` handling for genuine outer blocking-call interruption, and updated passive Environment context guidance for long verification. Real Streamable HTTP acceptance proved client disconnect/later-client observation, bounded live output, pass/non-zero/timeout classification, matching-writer cancellation, no ghost resources including managed-worktree validation failure, fresh-owner no-resurrection and no persisted `vfrun_` state. Final fixed-head evidence: focused `run_00198299d68611ab` PASS; full repository `run_2f67a98f9388baf6` PASS with Gateway 122.556s; vet `run_137dfd200fb22288` PASS; CLI/Gateway build `run_83f6d82aa3907bea` PASS. Artifact `dist/ai-dev-manager-phase21-02-3ee045c.exe`, 16,926,720 bytes, SHA-256 `A5BADEB350B6F7901F8C42D90037596E5EEA040DF4A2F07B1817EFB0CBE43F4D`. See `21-02-SUMMARY.md` and `21-CLOSEOUT.md`. Phase 22 follows as the newly opened planning node below. No push/tag/release or subagents.

## 2026-09-13 Phase 22 detailed planning checkpoint

From clean Phase-21 closeout `b6c96cd`, opened Phase 22 Temporary Task Environments + Safe Cleanup Workflow after explicit user direction to continue. Added `ADM-CORE-022` and prepared `22-CONTEXT.md`, `22-01-PLAN.md`, `22-02-PLAN.md` and `22-03-PLAN.md`. The design reuses Phase-15 retention/cleanup instead of introducing a task model: temporary Environments remain normal `env_` resources, are created atomically with explicit owner + positive TTL, may carry session/run provenance only, and support existing-root or optional managed-worktree mode. Cleanup is targeted to one Environment, preview-first and non-force; ordinary cleanup is state-only, managed cleanup reuses dirty/unpublished destroy safety and retains the branch. Active Phase-21 `vfrun_` is added to the runtime cleanup blockers. Desktop scope is limited to retention visibility and explicit Admin-MCP promote/cleanup actions; CLI lifecycle UX remains Phase 23. No feature code/tests/builds were run in this docs-only planning checkpoint. Next executable plan is 22-01. No push/tag/release or subagents.

## 2026-09-13 Phase 22-01 Core lifecycle completion

Implemented the Core temporary Environment lifecycle from planning commit `84cc9c7`: atomic owner+TTL creation for ordinary existing roots and optional managed worktrees, explicit session/run provenance without task semantics, durable-collision refusal, retention-aware managed-worktree persistence with rollback on post-Git persistence failure, matching-owner retention-only promotion, one-Environment targeted preview/execute cleanup, and complete owner-local cleanup blockers including active Phase-21 `vfrun_`. Ordinary cleanup preserves Workspace/project files; managed cleanup reuses non-force dirty/unpublished/tamper safety and retains the generated branch. Final evidence: focused `run_0e7fb13de9ea93e3` PASS; temporary lifecycle repeat x3 PASS; full repository `run_061053596b8e414a` PASS with Gateway 124.718s; vet `run_1e123fe4c00bcbf7` PASS; diff check PASS. See `.planning/phases/22-temporary-task-environments/22-01-SUMMARY.md`. 22-02 is ready and not started. No push/tag/release or subagents.

## 2026-09-13 Phase 22-02 Gateway workflow completion

Implemented the shared Agent/Admin temporary Environment workflow at `d51a426`: both surfaces expose `environment_temporary_create/status/promote/cleanup`, while generic durable `environment_create` remains Admin-only and generic retention management remains the Admin recovery surface. Real Streamable HTTP acceptance proved plain non-Git create/use/restart persistence, targeted preview/execute, matching-owner promotion/cleanup, managed-worktree optionality and retained branch, dirty/unpublished/tamper safety, active writer/process/`run_`/real `vfrun_` blockers, provenance-only `run_id`, privacy boundaries, and no force path. Final evidence: focused `run_93a79538fd11dc6e` PASS; full repository `run_5356c300b0312025` PASS with Gateway 170.068s; vet `run_bb24c986567e97a6` PASS; diff check PASS. See `.planning/phases/22-temporary-task-environments/22-02-SUMMARY.md`. 22-03 is ready and not started. No push/tag/release or subagents.

## 2026-09-13 Phase 22 closeout

Phase 22 is complete through final implementation head `56c79e3`. 22-03 added Desktop retention visibility and explicit Admin-MCP status/promote/preview/confirm-cleanup actions with no force path, no local writable fallback and connection-generation stale-result guards. Fixed-head evidence: focused `run_97667f3e145341a7` PASS; full repository `run_873a9a8b33dac10e` PASS with Gateway 203.904s; vet `run_05c2a8553a06917c` PASS; production browser `run_a41c0cce2b26efe3` PASS with 194 checks at each of three viewport/scaling entries; Desktop helper `run_a9afd9071cae231a` PASS 20/20; CLI build `run_45d7bd0b2d0f3a95` PASS; Wails v2.15.0 build `run_0a9fa491ea9ab808` PASS. CLI artifact SHA-256 `E53168E6D9614409DC6862C6AB7AAC98D15BD922F88D1CB3E212641FE9013F4A`; Wails artifact SHA-256 `FA2644F46FF7FE9901AED958488C8B122D931253A4B05E526538C01A7225C55A`. Native Wails/WebView2 click-through remains PENDING by availability because no native GUI-control path exists in this session. See `22-03-SUMMARY.md` and `22-CLOSEOUT.md`. Phase 23 remains planned/not started. No push/tag/release or subagents.

## 2026-09-13 Phase 23 detailed planning checkpoint

From clean Phase-22 closeout `bb1fc82`, opened Phase 23 CLI Agent UX + MCP/Skill Provisioning after explicit user direction to continue. Added `ADM-CLI-001` and prepared `23-CONTEXT.md`, `23-01-PLAN.md` and `23-02-PLAN.md`. Source calibration found that normal CLI management already uses Admin MCP and emits JSON, so Phase 23 does not add a second protocol or universal formatter. 23-01 focuses on the real remaining provisioning gaps: file/stdin MCP import to avoid Windows native-shell inline-JSON quoting, passive MCP inspect vs explicit refresh, Skill source update/multiple support roots, and global/Environment Skill availability. 23-02 exposes the existing Phase-20 Environment context bundle and Phase-22 temporary Environment lifecycle through the same Admin MCP client, with explicit stable IDs, no force cleanup, no hidden current Environment and no writable-state fallback. No feature code/tests/builds were run in this docs-only planning checkpoint. Next executable plan is 23-01 only. No push/tag/release or subagents.

## 2026-09-13 Phase 23-01 MCP/Skill CLI completion

Implemented Phase 23-01 at `b733948`: MCP import preview/apply now accept exactly one inline/file/stdin content source through a shared local CLI resolver; Admin-MCP-backed `mcp inspect` and explicit `mcp refresh` expose the existing passive-vs-active owner-runtime distinction; Skill source add/update support repeated support roots without implicit refresh; global structural and Environment-specific Skill availability are available as JSON CLI commands. Real disposable Gateway/Admin-MCP acceptance verifies credential-reference privacy, zero/multiple import-source non-mutation, passive inspect zero upstream traffic, refresh with zero business-tool calls and no desired-state mutation, source update/explicit refresh separation, availability scoping, broken-capability locality and normal no-fallback Admin MCP behavior. Final evidence: focused `run_06b50106d7daaae3` PASS; Phase-23 CLI repeat x3 `run_fa71e6ac63dd1dac` PASS; full repository `run_59eda237328f72de` PASS with Gateway 184.948s; vet `run_7a57d5e80b84d6c6` PASS; diff check PASS. See `23-01-SUMMARY.md`. 23-02 followed as the final execution node. No push/tag/release or subagents.

## 2026-09-13 Phase 23 closeout

Phase 23 is complete through final implementation head `f85c566`. 23-02 exposed the existing Phase-20 bounded Environment context bundle and Phase-22 temporary Environment lifecycle through the normal Admin-MCP-backed CLI, corrected capability-report help, preserved explicit stable IDs/no-fallback/no-force boundaries, and added real HTTP CLI acceptance for passive context, owner/TTL lifecycle, blockers, targeted cleanup and ordinary-root file preservation. Fixed-head evidence: focused `run_e6dd66630f674d97` PASS; Environment CLI repeat x3 `run_73ace1e85bf573f4` PASS; full repository `run_b7593afcb1f20f56` PASS with Gateway 205.344s; vet `run_88f9c56ff957c7a4` PASS; CLI build `run_56be6a0b03f2bee9` PASS. Artifact `dist/ai-dev-manager-phase23-final-f85c566.exe`, 17,175,552 bytes, SHA-256 `6E0E54F0EBDC3A7DEF20316C24B15E188692548E3D2AEA92DD54ADB561CF690A`. See `23-02-SUMMARY.md` and `23-CLOSEOUT.md`. Phase 24 subsequently completed at `9a4fea0`; see `24-CLOSEOUT.md`. No push/tag/release or subagents.
