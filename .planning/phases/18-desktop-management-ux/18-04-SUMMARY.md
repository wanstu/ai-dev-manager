# 18-04 Summary — Integrated Desktop acceptance

Date: 2026-09-12
Status: COMPLETE
Tested code/evidence head before closeout docs: `201c1b9 fix(desktop): polish context and diagnostics routing`

18-04 performed final integrated acceptance against the completed Phase 18 Desktop UX. A user-observed visual/routing defect was fixed during 18-04: ADM connection/system pages no longer show irrelevant Management Context, the ADM connection page was flattened into readable grouped cards, duplicate `gatewayState` DOM IDs were removed, and Environment diagnostics is now a standalone `#/diagnostics` route instead of a disguised Environments/detail shortcut.

## D01-D10 result

- D01 PASS — connection/overview states stay truthful; unloaded/offline is not invented as zero; auxiliary failures do not erase a successful inventory.
- D02 PASS — navigation, context, metric shortcuts, standalone Diagnostics route and Management Context route visibility are scoped and side-effect-free.
- D03 PASS — Workspace/Environment flows remain non-Git by default and retain stable target identities.
- D04 PASS — MCP/Skill global definitions, defaults, current Environment selection, import/probe/source refresh and cleanup boundaries are explicit.
- D05 PASS — Global and Environment-private Memory are separated; Global lazy-load is human-view only; private Memory remains explicit and scope-bound.
- D06 PASS — Runtime subviews and output/log reads are explicit, bounded and stale-scope guarded.
- D07 PASS — delayed reads/saves/profile/Environment transitions do not retarget stale results or lose drafts.
- D08 PASS — Diagnostics renders only existing inspection facts/unresolved IDs and does not probe MCPs, run verifiers or read Memory values.
- D09 PASS — production-browser fixture covers keyboard/focus/layout/long-content behavior at 1120x760, 820x560 and 125% scale.
- D10 PASS_WITH_ARTIFACT_PATH_NOTE — Wails-built final-name build/bin artifact was visibly launched and clicked; dist final-name copy was blocked by the active Gateway child locking the existing dist exe, so the native evidence records the exact running build/bin path.

## Final verification

- Helper regression: PASS 20/20.
- Production browser smoke after final polish: PASS, 131 checks x 3.
- Focused Go after final polish: PASS, `./cmd/ai-dev-manager-desktop ./internal/desktop ./internal/management`.
- Full latest Go: PASS via `run_60ca8af474ac99d3`.
- Latest vet: PASS via `run_d1f5e27c8f11f6dc`.
- Wails polish build: PASS via `run_401fb4c5bf51b1d3`, `dist/adm-desktop-phase18-polish-windows-amd64.exe`, SHA-256 `4D3ACB64B781447A49543D4C68AC0989884C01763ABB237B7F91123FFA74DEB8`.
- Visible native acceptance: PASS_WITH_ARTIFACT_PATH_NOTE via `cmd/ai-dev-manager-desktop/build/bin/adm-desktop-phase18-final-windows-amd64.exe`, PID `13540`, title `adm-desktop — 1.0 RC`, window `1120x760`.

## Evidence files

- `evidence/18-04-native-final-ui-acceptance.json`
- `evidence/18-04-native-final-start.png`
- `evidence/18-04-native-final-gateway.png`
- `evidence/18-04-native-final-diagnostics.png`
- `evidence/18-04-native-final-settings.png`

## Completion decision

Phase 18 is complete. Phase 19 is the next planned roadmap candidate and remains not started.
