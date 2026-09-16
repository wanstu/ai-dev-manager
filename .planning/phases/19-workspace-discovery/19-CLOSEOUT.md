# Phase 19 Closeout — Workspace Discovery + Project Navigation

Date: 2026-09-12
Status: COMPLETE with native GUI acceptance pending by explicit B09 rule
Core commit: `f8b3197 feat(discovery): add bounded workspace project discovery`
Shared surfaces commit: `3537c54 feat(discovery): expose workspace discovery through Agent and CLI`
Final implementation commit: `8a5c99a feat(desktop): add explicit project discovery and root selection`

## Result

Phase 19 is complete at the product/code level. ADM can now inspect large registered Workspace roots through a bounded metadata-only discovery report, expose the same read through Agent/Admin/CLI, and let Desktop users explicitly discover candidate project roots and hand one into the existing Environment creation flow without hidden scanning or automatic creation.

The Environment side now also exposes a bounded directory digest through Runtime-authorized Environment roots, including an explicit Desktop detail action. None of these reads changes writer ownership, capability selection, current Management Environment or desired state.

No push, tag or release was performed. Phase 20 was not started.

## Delivered scope

- Bounded Core Workspace discovery and Environment tree digest under registered/Runtime-authorized roots.
- Metadata-only traversal with depth/entry/candidate/digest/output budgets, exclusions, partial coverage, stop reasons, diagnostics and omission counters.
- Agent and Admin MCP read-only tools with stable IDs and shared DTOs.
- Admin client, management service and normal CLI convergence with no local writable fallback.
- Explicit Desktop Workspace discovery dialog with local filtering and explicit rescan semantics.
- Scope/generation guards for Workspace, profile, dialog and Environment-detail lifecycle changes.
- Exact relative-root handoff into the existing Create Environment dialog; creation remains explicit and uses existing Core containment revalidation.
- Explicit Environment detail directory summary; no scan on detail open or Diagnostics navigation.
- Responsive long-path/narrow/scaled UI handling and preserved focus behavior.

## Acceptance evidence

### Core / Gateway / CLI

- 19-01 Core acceptance is recorded in `19-01-SUMMARY.md`.
- Agent/Admin in-memory MCP sessions return equivalent discovery/digest reports for disposable fixtures.
- Unknown stable IDs and Workspace/Environment parent traversal are rejected through Core, Gateway and CLI.
- Environment digest cannot expose sibling projects and tampered managed worktree roots remain rejected.

### Desktop

Production browser smoke on final implementation `8a5c99a`:

- `run_105573529a6a9764` — PASS.
- 1120x760: 184 checks PASS.
- 820x560: 184 checks PASS.
- 1120x760 @ 125% scale: 184 checks PASS.

The smoke includes unloaded/loading/results/no-match/empty/partial/failure states, no hidden scans from typing/filtering/navigation, duplicate-submit suppression, exact root handoff, failed-create form retention, Workspace A→B stale-response rejection, delayed discovery across profile switch, close/reopen races, explicit Environment digest reads, digest stale-response rejection, focus restoration and long-path overflow checks. Existing MCP/Skill/Memory/Runtime/connection regression checks remain active in the same suite.

Desktop helper regression remained 20/20 PASS.

### Full repository

- Full Go: `run_4df936e2226f72ec` — `go test -count=1 ./...` PASS.
- Vet: `run_f84bb4ef0561b86d` — `go vet ./...` PASS.
- CLI artifact build: `run_c8a7c2806498667f` — PASS.
- Wails production build: `run_88c7536254ea3e9d` — PASS using Wails v2.15.0 through the already-allowlisted Go executable.

### Artifacts

- `ai-dev-manager-phase19-02-8a5c99a.exe`
  - 16,717,824 bytes
  - SHA-256 `84f2ef07f0425a6e8ff3bdcf31c0bda42732adb86e2ba4834b0b8a8ba00cd17d`
- `cmd/ai-dev-manager-desktop/build/bin/adm-desktop-phase19-02-8a5c99a.exe`
  - 17,639,936 bytes
  - SHA-256 `2564136b5d2a3bb375a24b96e30731d3eb5744e5b4e9324a09ba3f1e2253d210`

### Native B09 note

Native Wails/WebView2 click-through is **pending**, not PASS. This session has no native GUI-control/screenshot capability. The B09 plan explicitly allows native unavailability to be recorded pending. The active Gateway was not replaced, terminated or repointed just to obtain a screenshot.

## Completion decision

B01–B08 and B10 are accepted. B09's build portion is accepted and its native click-through is explicitly pending under the plan's own availability rule. No code/product blocker remains open for Phase 19.

Phase 20 — Agent Context Bundle + Capability Injection — remains only the next planned roadmap candidate. It is not opened or started by this closeout.
