# Phase 18 Planning Progress

Date: 2026-09-11
Baseline: `93a74d9 docs: plan Phase 18 desktop UX refactor`
Work type: durable Phase 18 planning/continuation record. 18-01 is accepted with automated/browser/full-suite/exact-Wails/native evidence; active execution is now recalibrating the next Phase 18 work after a new user-requested Skill bulk-management requirement.

## Latest user direction

Continue Phase 18 development with two user-priority inserts before the remaining sequential page work:

1. Skill management: **bulk availability check**, **bulk delete**, and **one-click cleanup of unavailable Skills**. `disabled` is not an unavailable state; destructive cleanup must use a fresh explicit current-Environment availability result, and deletion means ADM catalog metadata only.
2. Tray lifecycle: when the user chooses **閫€鍑?* from the Windows tray, prompt whether to also stop the current local background CLI/MCP Gateway. Offer an explicit keep-background choice and Cancel. Never stop a remote/incompatible process or arbitrary PID; reuse the existing loopback/ADM-owner-safe `StopLocalADM` path.

These are user-authorized priority inserts. They do not by themselves mark all of 18-02 or 18-03 complete; the original acceptance gates still apply to the remaining page work.

## Delivery decisions

- 18-01 remains the first implementation plan.
- 18-02 details Workspace/Environment management and existing Runtime views.
- 18-03 details MCP/Skill, explicit Memory scopes, existing system controls and Environment diagnostics presentation.
- 18-04 is mandatory integrated acceptance and Phase 18 closeout. Fixes are conditional on evidence; acceptance itself is not optional.
- Implement sequentially: 18-01 -> 18-02 -> 18-03 -> 18-04. Dependencies are delivery gates, not new product prerequisites.
- Detailed plans must identify requirements, UI/interaction design, exact existing adapter operations, files, tasks, positive/negative acceptance, commit nodes and interruption checkpoints.
- Existing Admin MCP, writer/allowlist checks, explicit Memory reads, profile request serialization and stable IDs remain the authority.
- No code/backend/schema/framework/persistence changes in this planning session. No subagents, push, tag or release.

## Durable progress

| Work item | Status | Resume action |
|---|---|---|
| Git/context/source boundary review | Done | Clean master at 93a74d9; only root tracked AGENTS.md; existing adapter reviewed. |
| 18-02 detailed plan | Committed `b6da7fc`; execution waits for 18-01 | Detailed UI/operation map, 4 task nodes and B01-B08 acceptance saved in 18-02-PLAN.md. |
| 18-03 detailed plan | Committed `f52b872`; execution waits for 18-02 | UI/operation map, 5 task nodes and C01-C08 acceptance saved in 18-03-PLAN.md. |
| 18-04 detailed plan | Complete | Mandatory D01-D10 integrated journeys, evidence validity, fix rules and closeout criteria saved in 18-04-PLAN.md; final plan traces the union of Phase 18 requirements. |
| Cross-plan consistency | Committed at `3b26a92` | CONTEXT, UI design, VALIDATION, STATE, PROJECT, ROADMAP and PHASE-MAP synchronized. Four plans, 21 unique requirement IDs and 35 local acceptance cases checked; phases 01-18 plan count is 33 excluding phase-00. |
| 18-01 Task 1/2 implementation | Committed `6aa21bf` / `d243bb0` | Ten-route shell, truthful dashboard/load state, auxiliary-failure isolation and stale-scope guards implemented; focused JS/Go checks green. |
| 18-01 acceptance | Accepted and closed at `ab3fdc4` | Exact Phase 18-01 Wails artifact passed native UI Automation route/focus/modal/min-size checks; durable JSON/PNG evidence is saved under the phase evidence directory. |
| Tray exit behavior reset | Complete; local commit node pending | User rejected tray-time background-service stop. Windows tray now exits Desktop only and preserves CLI/MCP background service; stopping local ADM remains an explicit manual main-window action. Focused Desktop Go PASS. |
| User-priority Skill bulk-management sub-slice | Committed `c4f8d60` | Explicit current-Environment availability check, stable-ID selection/batch delete and fresh-probe one-click unavailable cleanup implemented. Helper tests 5/5, combined JS 11/11, Chromium 47 checks at three sizes/scales, focused Go gate PASS; Wails build `run_f8b9b1ac92f345b5` PASS. Full 18-03 remains gated behind 18-02. |
| 18-02 Task 1 Workspace/Environment lists | Committed `9fe1564` | Added pure loaded-snapshot `project-pages.js` filtering/join/count helper, Workspace and Environment search/count UI, Workspace鈫扙nvironment presentation-only filter navigation, Workspace-name/selection joins, current-context markers and invalid Workspace-filter retention. Helper suite 16/16 PASS, Chromium 61 checks at three sizes/scales PASS, focused Desktop/management Go PASS, diff check PASS. |
| 18-02 Task 2 Environment detail/context | Committed `fac96bb` | Shared detail modal reuses the scoped management inspection/availability result, groups identity/authority/capability/unresolved facts, keeps Memory values explicit, captures modal mutation Environment IDs, invalidates private detail on context transitions and guards late A鈫払 responses. Chromium 71 checks at three sizes/scales PASS, combined helper regression 16/16 PASS, focused Desktop/management Go PASS, diff check PASS. |
| 18-02 Task 3 Runtime subviews/output | Complete; local commit node pending | Added local Verifiers/Processes/Runs subviews with independent states and no tab reads, bounded current-owner output identity, existing truncation metadata, Environment/subview/resource stale guards and stable action identity. Successful mutations are not replayed when follow-up refresh fails. Helper regression 20/20 PASS, Chromium 80 checks at three sizes/scales PASS, focused Desktop/management Go PASS, diff check PASS. The user-priority Skill global availability correction and Desktop startup-local-service option are complete in the current local commit; Task 4 packaged/native acceptance remains next after this checkpoint. |

| Skill global availability + Desktop startup local ADM option | Committed in current checkpoint | Global `skill_availability_list` checks source/artifact/support roots without Environment state; Skill bulk cleanup uses a fresh catalog-scoped probe; connection profiles add `start_service_on_desktop_launch` and only the active local loopback profile auto-starts on Desktop startup. Evidence: helper regression 20/20 PASS, focused Go async run `run_d17bf3d3621b5684` PASS, Chromium smoke 82 checks x 3 PASS, diff check PASS. |
| Skill/MCP filtered management UX | Complete; committed in current checkpoint | Skill Sources are editable via `skill_source_update` without implicit refresh; MCP/Skill search supports literal text or `/pattern/flags`; current-Environment-unselected filters were added; bulk default and current-Environment enable/disable actions operate only on the current visible filtered rows. Evidence: JS syntax PASS, helper regression 20/20 PASS, Chromium smoke 91 checks x 3 PASS, focused Go async run `run_208c048021def316` PASS, diff check PASS. |
| Global Memory route lazy-load UX | Complete; see Git log for commit hash | Memory route now lazy-loads Global Memory values for the human management view after a connected snapshot, while successful refresh resets stale disconnected Memory text to `尚未加载 Global Memory`. This does not add Agent automatic Memory injection. Evidence: JS syntax PASS, helper regression 20/20 PASS, Chromium smoke 89 checks x 3 PASS, focused Desktop Go PASS. |
| 18-02 verification summary | Complete; see Git log for commit hash | `18-02-SUMMARY.md` records B01-B07 PASS and B08 automated PASS / exact visible native pending. Evidence: helper regression 20/20 PASS, Chromium smoke 89 checks x 3 PASS, focused Go PASS, Wails build `run_cbaafab3ac0e9fe4` PASS, artifact SHA-256 `7990B4C439E36CEE84912549CE6C2639576939750A4480C72A2498562B63314B`, exact artifact second-instance launch exited 0 while the existing user Desktop owned the single-instance window. |
| MCP/Skill exposure clarification | Recorded; see Git log for commit hash | User confirmed this must not be confused with automatic injection: MCP/Skill exposure remains Environment-scoped pull/access through `environment_mcp_*` and `environment_skill_*`; Desktop visibility/defaults do not inject Agent context. Automatic Agent Context Bundle / capability injection belongs to later Phase 20. |
| 18-03 Task 1 MCP import/probe freshness | Complete; committed in current checkpoint | Import preview/apply now uses connection + input fingerprint freshness, input/options changes invalidate Apply, Apply is single-use, and explicit MCP probe results are keyed to the initiating Environment. Evidence: JS syntax PASS, Chromium smoke 95 checks x 3 PASS, helper regression 20/20 PASS, focused Go PASS. |

| 18-03 Task 2 Skill/source management readability | Complete; see Git log for commit hash | Skill page now has local Skills/Sources subviews, source search and Skill source dropdown filters, separated Source/Artifact/default/current-Environment/global-availability facts, and stable Source refresh/edit/remove plus Skill default/selection/delete controls. No Skill artifact read, source refresh or Agent context injection occurs on subview/filter navigation. Evidence: JS syntax PASS, helper regression 20/20 PASS, Chromium smoke 101 checks x 3 PASS, focused Desktop/management Go PASS. |

| 18-03 Task 3 explicit-scope Memory page | Complete; see Git log for commit hash | Memory route now shows Global and current Environment-private scopes in one page. Global lazy-loads for the human view; Environment-private Memory remains explicit Load and is bound to connection + current Management Environment. Environment detail now links to Memory and no longer owns/clears private Memory values. Evidence: JS syntax PASS, helper regression 20/20 PASS, Chromium smoke 109 checks x 3 PASS, focused Desktop/management Go PASS. |

| 18-03 Task 4 system controls/diagnostics | Complete; see Git log for commit hash | Gateway/profile/endpoints/lifecycle, exec allowlist and Desktop settings are grouped without changing Core/authorization. Environment detail has Summary/Diagnostics subviews; Diagnostics render only existing InspectEnvironment facts/unresolved IDs and subview switches make no probe/verifier/reconnect/Memory calls. Evidence: JS syntax PASS, helper regression 20/20 PASS, Chromium smoke 122 checks x 3 PASS, focused Go PASS, diff check PASS. |
| 18-03 verification summary | Complete; see Git log for commit hash | `18-03-SUMMARY.md` records C01-C07 PASS and C08 automated PASS / exact visible native pending. Evidence: helper regression 20/20 PASS, Chromium smoke 122 checks x 3 PASS, focused Go PASS, Wails build `run_d23b26baf7b1e4d2` PASS, artifact SHA-256 `78881B25AAB6C744532AC65CDF884006AB50040A0755482C80EC3965CF0D233F`, exact artifact second-instance launch exited 0 while the existing user Desktop owned the single-instance window. |

| 18-04 integrated acceptance checkpoint | Superseded by final closeout | Earlier checkpoint recorded D01-D09 PASS and D10 pending; final closeout now records D01-D10 accepted with a D10 artifact-path note. |

| 18-04 user feedback context/diagnostics polish | Committed; see latest local Git log | User screenshot showed ADM connection context/layout and Environment diagnostics route confusion. Fixed Management Context route visibility, made diagnostics a standalone route, flattened ADM connection layout, removed duplicate gatewayState id, and updated helper/browser/Go evidence. Wails final-name build is blocked only by the running final exe file lock; polish artifact build succeeded via `run_401fb4c5bf51b1d3` with SHA-256 `4D3ACB64B781447A49543D4C68AC0989884C01763ABB237B7F91123FFA74DEB8`. |

## Continuation protocol

Before the next work unit, update this table and STATE with the latest completed file/task, pending checks and next action. Commit each complete plan node locally after diff checks. If stopped before a clean commit, keep partial work marked incomplete and release the writer when possible. If a tool response is lost, inspect Git/run evidence before retrying a mutation.

Environment: `env_43a2d0ca74fbc0f1`. The current session acquired its own bounded writer lease; the next session must inspect and acquire under a fresh identity. This log is not proof of a current lease; Runtime is authoritative.

No verification Run remains active from Phase 18 closeout. Phase 18 is complete; Phase 19 planning is next. Do not push/tag/release.

| Phase 18 closeout | Complete; see Git log | D01-D10 accepted. Final native evidence uses the Wails-built build/bin final-name artifact because the active Gateway child locked the dist final exe; this is recorded as an artifact-path note. Latest full Go `run_60ca8af474ac99d3` PASS, vet `run_d1f5e27c8f11f6dc` PASS, browser 131 checks x 3 PASS, native evidence saved under `evidence/18-04-native-final-*`. Phase 19 planning is next; not started. |
