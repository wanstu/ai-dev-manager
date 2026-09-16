# 18-02 Summary — Workspace, Environment and Runtime page refinement

Date: 2026-09-12
Status: automated/browser/Wails-build verification complete; exact visible native acceptance pending because an existing user Desktop instance owns the single-instance window.

## Scope delivered

18-02 refined the existing Desktop management shell without changing the Core authority model:

- Workspace rows now have loaded-snapshot search, visible/total counts, Environment counts from the same snapshot, long-path-safe rendering and presentation-only navigation to filtered Environments.
- Environment rows now show Workspace identity, root/state, selection counts, writer observation and current Management Environment markers without retargeting on filters.
- Environment detail now reuses one scoped inspection/availability read, groups Identity / Runtime authority / Capability issues / Unresolved references, keeps private Memory values explicit and rejects late stale responses.
- Runtime now has local Verifiers / Processes / Runs subviews with independent states, explicit refresh, bounded current-owner output, truncation labels and stale resource/output guards.

User-priority corrections landed during this 18-02 execution window and are recorded here because they affected the same Desktop management surface:

- Tray exit lifecycle was simplified/split after dogfood feedback. The current safe rule is explicit lifecycle choice; no Windows yes/no custom-button ambiguity is used.
- Skill bulk availability is now global catalog structural availability, not current-Environment enabled state.
- Desktop connection profiles can opt in to starting a local loopback ADM Service on Desktop startup.
- Skill Source definitions can be edited without implicit refresh.
- Skill and MCP pages support literal or `/pattern/flags` search, current-Environment-unselected filters, and current-visible-result bulk toggles for new-Environment defaults and current-Environment enablement.
- Global Memory management view now lazy-loads values when the human operator opens the Memory route; this does not implement Agent automatic Memory context injection.

## Commit trail

- `9fe1564 feat(desktop): refine workspace and environment lists`
- `fac96bb feat(desktop): clarify environment details and context`
- `ee8faac feat(desktop): organize runtime views and output`
- `073fab1 fix(desktop): split tray exit lifecycle choices`
- `746ed8c feat(desktop): add startup service option and global skill checks`
- `903004f feat(desktop): refine skill and mcp filtered management`
- `e169b1d fix(desktop): lazy-load global memory page`

## Acceptance evidence

### Automated/frontend

- `node --check cmd/ai-dev-manager-desktop/frontend/app.js` — PASS.
- `node --check tests/desktop-ui/browser-smoke.cjs` — PASS.
- Helper regression — PASS, 20/20:
  - `tests/desktop-ui/navigation.test.cjs`
  - `tests/desktop-ui/dashboard.test.cjs`
  - `tests/desktop-ui/project-pages.test.cjs`
  - `tests/desktop-ui/runtime-view.test.cjs`
  - `tests/desktop-ui/skill-bulk.test.cjs`
- Production-asset Chromium smoke — PASS, 89 checks at each size/scale:
  - 1120x760 @ 1.0
  - 820x560 @ 1.0
  - 1120x760 @ 1.25

The browser smoke covers B01-B08 behavior across Workspace/Environment filtering, modal focus/draft guards, detail stale-response rejection, Runtime subview/output identity, profile switching, non-Git fixture behavior, Skill/MCP filtered bulk operations and Global Memory route lazy-load. Local Edge returned empty `--dump-dom` output in this environment, so the smoke harness selects Chrome when available.

### Go/adapter

- Focused 18-02 Go gate — PASS:
  - `go test -count=1 ./cmd/ai-dev-manager-desktop ./internal/desktop ./internal/management`

Recent related focused runs also passed while landing the user-priority Desktop management corrections:

- `run_208c048021def316` — PASS for `./internal/catalog ./internal/app ./internal/management ./internal/desktop ./internal/adminmcp ./internal/gateway ./cmd/ai-dev-manager-desktop`.
- `run_cbaafab3ac0e9fe4` — PASS for exact Wails build command below.

### Wails build and native launch

- Exact Wails build — PASS via ADM async Run `run_cbaafab3ac0e9fe4`:
  - `powershell -NoProfile -File scripts/build-desktop.ps1 -OutputName adm-desktop-phase18-02-windows-amd64.exe`
- Artifact:
  - `D:\projects\ai-dev-manager-v2\dist\adm-desktop-phase18-02-windows-amd64.exe`
  - size: `17,335,808` bytes
  - SHA-256: `7990B4C439E36CEE84912549CE6C2639576939750A4480C72A2498562B63314B`
- Non-disruptive second-instance smoke — PASS:
  - launching the exact artifact produced PID `46500`, which exited within 5 seconds with exit code `0` because an existing user Desktop single-instance window was already running.

Exact visible native UI acceptance for the new artifact is still pending. I did not close or kill the user's active Desktop process to free the single-instance lock, so this summary does not claim that the exact `adm-desktop-phase18-02-windows-amd64.exe` window was visibly exercised.

## Acceptance mapping

- B01 PASS — Workspace search/count and Workspace-to-Environment filter navigation are covered by browser smoke and helper tests.
- B02 PASS — modal draft/error/focus and captured ID operations are covered by browser smoke plus Core/adapter tests.
- B03 PASS — Environment rows/detail expose Workspace identity, writer facts, counts and unresolved/capability facts without implicit Memory value reads.
- B04 PASS — delayed A then B detail/log races are rejected by generation/resource guards in browser smoke.
- B05 PASS — Runtime subviews switch locally with independent list errors and no implicit backend reads.
- B06 PASS — explicit logs/output are bounded current-owner observations; successful mutation versus failed refresh remains distinct.
- B07 PASS — existing run/stop/cancel controls preserve writer/owner identity and no implicit lease takeover is added.
- B08 AUTOMATED PASS / EXACT VISIBLE NATIVE PENDING — production-browser layout/focus at 1120x760, 820x560 and 125% passes; Wails build and second-instance smoke pass; exact visible native route/dialog checks remain pending until the existing Desktop single-instance window can be closed or replaced safely.

## Handoff

18-02 code and automated verification are complete enough to preserve the implementation node. Before declaring full native 18-02 acceptance, run the exact artifact visibly when the active Desktop single-instance lock is available. 18-03 remains the next planned implementation slice only after respecting this pending native evidence decision; 18-04 still performs mandatory integrated final acceptance.
