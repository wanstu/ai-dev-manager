# 18-03 Summary — MCP, Skill, Memory and system page refinement

Date: 2026-09-12
Status: automated/browser/Wails-build verification complete; exact visible native acceptance pending because an existing user Desktop instance owns the single-instance window.

## Scope delivered

18-03 refined MCP, Skill, Memory and system/diagnostic management over the existing shared Core. It did not add a second state store, new backend diagnostics API, automatic Agent prompt injection or hidden Runtime probing.

Delivered behavior:

- MCP management now keeps global definition, new-Environment default, current-Environment enablement and explicit Runtime probe distinct. Import preview/apply is keyed by connection generation and input fingerprint; changing content/options invalidates Apply; Apply is single-use.
- MCP probe results are keyed to the initiating Environment and MCP ID. A late result from Environment A cannot populate Environment B.
- Skill management is split into local Skills and Sources subviews. Skills can be filtered by search/state/source, with separate Source, Artifact, Support roots, default, current-Environment enablement and global availability facts. Sources can be searched, edited, refreshed or removed explicitly.
- Skill and MCP filtered bulk actions operate only on the current visible rows.
- Memory management is unified into one Memory page with Global and current Environment-private scopes. Global values lazy-load when the human opens the Memory route; Environment-private values require an explicit load and bind to the current Management Environment, not to the Environment detail modal lifetime.
- Environment detail now has Summary and Diagnostics subviews. Diagnostics render only existing `InspectEnvironment` payload facts, unresolved IDs, source/reason/generated/observed fields and honest unknown states.
- Gateway, exec allowlist and Settings pages are grouped by existing responsibilities without changing profile selection, loopback/bootstrap restrictions, command execution authority or Desktop preference persistence.
- A planning note records that current MCP/Skill exposure is pull/access through `environment_mcp_*` / `environment_skill_*`; MCP inventories, Skill instructions and Memory values are not automatically injected into Agent context. Automatic Agent Context Bundle / capability injection remains later Phase 20 scope.

## Commit trail

- `3a2eeb5 docs: clarify agent capability exposure semantics`
- `fae44cb feat(desktop): guard MCP import and probe scope`
- `f323ba3 feat(desktop): organize Skill source views`
- `f44f37b feat(desktop): unify explicit-scope Memory management`
- `24e523e feat(desktop): clarify system and environment diagnostics`

Related user-priority work landed before the formal 18-03 nodes and is carried as predecessor context, not as automatic injection semantics:

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
- Production-asset Chromium smoke — PASS, 122 checks at each size/scale:
  - 1120x760 @ 1.0
  - 820x560 @ 1.0
  - 1120x760 @ 1.25

The browser smoke covers C01-C08 behavior across MCP import freshness, stale Apply invalidation, explicit probe identity, Skill/Sources subviews, source/state/search filters, filtered bulk actions, Global and Environment-private Memory scope handling, detail Summary/Diagnostics switching and grouped system controls. It also rechecks affected A/B scope/focus behavior. Diagnostics subview switching makes no extra adapter call; no MCP probe, verifier run, reconnect or Memory value read is triggered by entering Diagnostics.

### Go/adapter

- Focused 18-03 Go gate — PASS:
  - `go test -count=1 ./cmd/ai-dev-manager-desktop ./internal/desktop ./internal/management`

No backend API was added for Task 4 diagnostics. Existing Admin MCP/Desktop adapter calls remain the authority surface.

### Wails build and native launch

- Exact Wails build — PASS via ADM async Run `run_d23b26baf7b1e4d2`:
  - `powershell -NoProfile -File scripts/build-desktop.ps1 -OutputName adm-desktop-phase18-03-windows-amd64.exe`
- Artifact:
  - `D:\projects\ai-dev-manager-v2\dist\adm-desktop-phase18-03-windows-amd64.exe`
  - size: `17,356,800` bytes
  - SHA-256: `78881B25AAB6C744532AC65CDF884006AB50040A0755482C80EC3965CF0D233F`
- Non-disruptive second-instance smoke — PASS:
  - launching the exact artifact produced PID `29868`, which exited within 5 seconds with exit code `0` because an existing user Desktop single-instance window was already running.

Exact visible native UI acceptance for this named artifact remains pending. I did not close or kill the user's active Desktop process to free the single-instance lock, so this summary does not claim that the exact `adm-desktop-phase18-03-windows-amd64.exe` window was visibly exercised.

## Acceptance mapping

- C01 PASS — default inclusion, current-Environment enablement and explicit MCP probe remain separate. Filtered bulk operations send current visible IDs only; configured/enabled is not treated as healthy.
- C02 PASS — import preview/apply uses connection + input fingerprint freshness. Input/options/profile changes invalidate Apply; stale/error preview cannot apply; apply is single-use and does not persist literal secret values from import preview UI logic.
- C03 PASS — Skill/Sources filters and actions preserve source/artifact/selection/availability identity; filtering does not read artifacts, refresh sources, enable Skills automatically or execute Skills.
- C04 PASS — Global and Environment-private Memory scopes are visibly distinct. Snapshot/diagnostic rendering does not read Environment-private values; Global route lazy-load is a human management-view behavior, not Agent injection.
- C05 PASS — Environment-private Memory actions bind to current Management Environment and connection/scope state. Closing Environment detail cannot clear or retarget loaded Memory page values; switching Environment/profile clears scope values and rejects old data.
- C06 PASS — Gateway profile/endpoints/lifecycle, exec allowlist and Desktop settings are grouped while preserving existing profile/request/loopback behavior and error boundaries.
- C07 PASS — editor/selection/import/source/Memory actions remain reachable with modal/focus guards and retained error drafts. Existing route/detail shortcuts still close only the intended modal and navigate to the intended route.
- C08 AUTOMATED PASS / EXACT VISIBLE NATIVE PENDING — Environment Diagnostics uses existing inspection facts/reasons/source/freshness/unresolved IDs and honest unknown states. It never probes, reconnects, runs verifiers, reads Memory values or fans out over all Environments. Production-browser layout/focus passes at 1120x760, 820x560 and 125%; Wails build and second-instance smoke pass; exact visible native checks remain pending under the active Desktop single-instance window.

## Handoff

18-03 code and automated verification are complete enough to preserve the implementation node. Before declaring full native 18-03 acceptance, run the exact artifact visibly when the active Desktop single-instance lock is available.

18-04 remains mandatory. It must perform integrated cross-route acceptance against the final code, including final full repository tests/vet and exact named Wails artifact evidence. The 18-02 and 18-03 exact visible native checks remain pending until they can be run without disrupting the active Desktop instance.
