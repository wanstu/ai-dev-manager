# Desktop spacing dogfood fix

Baseline: e180cb9, master. User supplied three screenshots of Settings, Local service lifecycle, and MCP rows. This bounded follow-up takes priority over Phase 19, which remains unstarted.

Requirements: ADM-DESKTOP-001 and ADM-MGMT-001 presentation refinement; preserve ADM-CORE-007/012 authority and all existing handlers. New prerequisites: none.

Plan:
1. Add a padded content body to the Settings and lifecycle cards; remove the higher-specificity lifecycle padding conflict. Use neutral, readable guidance for informational text.
2. Make MCP rows adapt to available content width, preserving stable IDs, action order, enablement and explicit probes.
3. Verify actual production DOM geometry in the existing Chromium smoke at its three window/scale scenarios; build a uniquely named Wails artifact. No Core changes or new persistence.
4. Record evidence, commit scoped files, release writer. No push/tag/release. Native visual acceptance must be reported separately from browser/build evidence.

Non-goals: Phase 19 implementation, broad redesign, Runtime changes, catalog/selection semantics.

Acceptance: controls and notes have at least 14px inset; heading/body and note spacing do not overlap; narrow MCP rows stack actions beneath content; wide rows remain readable; existing bridge-call and route smoke remains green.

Status: implementation and browser/build verification complete; native visual acceptance pending.

Evidence (2026-09-12, baseline e180cb9 plus scoped frontend diff):
- Production browser smoke run_f8c4f7866b9fadda PASS, 149 checks each at 1120x760@1, 820x560@1, 1120x760@1.25. Includes non-empty MCP geometry, control/note insets and existing interaction checks.
- Wails build run_4cbae173cf05abdd PASS; dist/adm-desktop-spacing-fix-windows-amd64.exe.
- SHA-256: 227D1D1E9AFF39B85F297EDFB1D7CDDDEB9C4772CCFF3BBD96782F08E17E4E1B (Node crypto; Get-FileHash was unavailable in this PowerShell).
- git diff --check PASS before final documentation update; cached diff checked before commit.
- No Go/Runtime code changed; full Go/vet suites were not repeated for this HTML/CSS change.
- Native artifact was built but not launched/click-tested in this follow-up. User can inspect the three affected sections using the uniquely named build. Existing running Desktop/Gateway was left running.

Phase 19 remains unstarted; resume its detailed planning after this dogfood review.
