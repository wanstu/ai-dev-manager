# Phase 18 — UI Refactor Design

Date: 2026-09-11
Source baseline: `0173259`
Scope decision checkpoint: `d95b088`
Status: reviewed design for planning; no UI implementation or visual acceptance yet

Read with `18-CONTEXT.md`, `18-01-PLAN.md`, `18-02-PLAN.md`, `18-03-PLAN.md`, `18-04-PLAN.md` and `18-VALIDATION.md`.

## 1. Outcome

The operator should immediately know which ADM connection and Environment they are managing, find a management area through a stable menu, and perform the existing operation without scrolling through unrelated panels. The first slice changes information architecture and presentation; it keeps the established product operations.

### Source-grounded findings

| Evidence at the baseline | Consequence for this design |
|---|---|
| `frontend/index.html` puts Gateway, six metrics, Environment context, MCP, Skill, Workspace/Environment, allowlist/Memory and Runtime into one long page. | Separate these into sections and keep connection/context visible across sections. |
| `frontend/app.js` is about 78 KB and directly binds control IDs, mutations, rendering and reads. | Keep the existing controls/handlers; extract only small navigation/dashboard helpers first. Rewriting every manager would make 18-01 too large. |
| `dialogs.js` owns native editor dialogs; Environment detail is a separate custom modal. | Both dialog systems must remain outside hidden route containers; preserve Escape, errors, opener focus and list scroll. |
| `connections.js` serializes bridge requests and waits for old-target continuations before switching profiles. | Retain that sequencing. A new shell must not put actionable navigation outside the transition lock or bypass the bridge proxy. |
| `refreshSnapshot` combines GetSnapshot/ListSkillSources and then Environment/Skill/Runtime reads under one catch. | Render a valid base snapshot independently. An auxiliary read failure must be shown at its own area instead of clearing every page. |
| `clearManagementData` calls renderSnapshotBase with synthetic empty arrays. | Separate disconnected/unknown from a successful empty snapshot; dashboard counters must not display false zeroes. |
| `managementEnvironmentID` and `selectedEnvironmentID` serve current management context and opened detail respectively. | Keep those roles explicit; navigation does not silently select an Environment or leave Memory/details associated with a different context. |
| `main.go` embeds plain assets, uses NewClientAdapter and opens at 1120x760 with minimum 820x560. | Keep Wails and plain deferred scripts. Design against the actual Desktop window sizes, not a new web app/framework. |
| `main_test.go` checks bindings and many source/asset markers. | Retain useful binding guards, but add behavior evidence; string presence does not prove that a hidden panel can be reached. |

The above is a source review, not a claim that screenshots or live click-through were performed.

## 2. Shell layout

| Zone | Contents / behavior |
|---|---|
| Persistent header | Compact ADM brand, current profile selector, connection status, last successful snapshot time, refresh. Link to ADM connection details. Move detailed endpoints/PID/version into the Gateway section. |
| Context row | Existing Management Environment selector, name/root summary and an explicit link to Environment details. Keep a visible no-selection/global-only mode. Selection is existing UI context, not a persisted Environment mutation. |
| Grouped left navigation | Overview; development (Workspace, Environment, Runtime); capabilities (MCP, Skill); Memory; system (ADM connection, exec allowlist, diagnostics entry, Settings). Text labels remain visible. |
| Active content | One section heading, description, primary action area and existing list/detail content. Render only the active section visibly; retain inactive DOM to preserve filters and drafts. |
| Shared overlays | Native editors, Environment detail/backdrop and status feedback are siblings of the page containers, not children of a hidden section. |

Use the current neutral background, white panels, dark text/actions and ADM assets. Shell text should normally be 13-14 px or larger, menu controls about 40 px high, and active/focus states distinguishable without color alone. Do not add a theme switch or visual dependency.

At 1120x760 use a roughly 200 px sidebar; at the existing 820x560 minimum use a roughly 160 px sidebar and a wrapping header/context row. Use minmax(0, 1fr) / min-width: 0 so paths and endpoints cannot force the entire window wider. Section content has one primary scroll area; existing bounded list/log scrolling remains usable. Validate long paths, long names and 125% scaling. No change to the native minimum window size is needed for 18-01.

## 3. Navigation and migration map

Section IDs are fixed presentation identifiers, represented by hashes such as `#/overview`. They never contain host paths, credentials, Environment IDs or Memory content.

| Menu label | Route / entry | 18-01 destination and preserved capability |
|---|---|---|
| 概览 | overview | Existing six snapshot metrics, truthful connection/data state and navigation shortcuts. |
| Workspaces | workspaces | Existing Workspace list/metadata and add/rename/remove actions; metadata-safe semantics unchanged. |
| Environments | environments | Existing list, create/rename/remove and detail/selection/Memory modal. |
| Runtime | runtime | Existing Verifier / Process / Run lists and explicit output/log/run/stop/cancel controls for the selected Environment. No new execution mode. |
| MCP | mcp | Existing mcpManager, filters, config/editor, import preview/apply and explicit probe. |
| Skills | skills | Existing skillManager, sources, filters, refresh and Environment selection. |
| Memory | memory | Existing Global Memory panel; add a clearly labeled link to the current Environment's existing detail/Memory controls. Values still require explicit Load. Separate full Memory tabs can follow later. |
| ADM 连接 | gateway | Existing connection add/edit/delete controls, endpoints/status and local start/stop controls. The single profile selector moves to the persistent header. |
| 执行许可 | exec-allowlist | Existing allowlist panel and add/remove forms. No terminal or arbitrary command launcher. |
| 设置 | settings | Existing launchAtLogin control and tray behavior hint; no new preference options/storage. |
| 环境诊断 | shortcut to environments | Focus the existing selected-Environment inspect entry; with no selection show the Environment selection instruction. Inspection remains an explicit click. Reuse existing capability reasons instead of adding an empty diagnostics page or new API. |

Ten real routes plus a diagnostics shortcut are sufficient for the first slice. A dedicated diagnostics section is a later candidate only if existing evidence warrants it.

Route rules:

- No hash / unknown token -> overview, replacing the invalid entry rather than growing history.
- Menu click, dashboard shortcut and hash back/forward use one route handler.
- Exactly one page and one active menu item; use aria-current="page", a focusable section heading and keyboard-operable controls.
- Routing alone does not clear the selected Environment, filters, lists or drafts and does not call management mutations.
- While an editor/detail modal or connection transition is active, reject/defer navigation and restore the current hash; never hide a draft underneath another page. After closing, return focus to a visible opener, otherwise to the active section heading.
- Put an explicit hidden-page CSS rule above layout interactions in precedence so grid/flex rules cannot unhide an inactive page. Hidden sections must be absent from keyboard navigation and the accessibility tree.
- No route restoration across app restart, new preference file, routing package or Core resource is introduced.

## 4. Dashboard data contract

Keep the overview deliberately small:

1. Connection/management data state and successful snapshot timestamp.
2. Six compact, clickable metrics: Workspace count, Environment count, MCP definition count, Skill count, exec allowlist count and Global Memory entry count.
3. Current Environment name/root and selection counts already present in the snapshot; explain the global-only state when none is selected.
4. Shortcuts to Workspace/Environment, MCP, Skills and ADM connection sections. Shortcuts navigate; destructive operations remain in their existing forms.

Do not add charts, task progress, inferred health, cross-Environment Run totals, historical trends or unbounded issue lists. Configured MCP/Skill counts are inventory, not proof of healthy/usable Runtime.

| Data state | Display | Authority / behavior |
|---|---|---|
| No profile / disconnected | “未连接 / 未加载”, counters as — | Shell, profile editor and Settings work; unavailable management actions have a reason. |
| Connecting / loading snapshot | Loading indicator, counters as — until valid data | No stale values from a previous target. |
| Successful empty snapshot | Numeric 0 and helpful registration/configuration entry | Only a successful valid response establishes emptiness. |
| Successful populated snapshot | Actual counts and last success time | Existing sanitized GetSnapshot response only; no Memory values. |
| Same-target refresh failed | Clearly labeled unavailable or stale last-success view | Never label old data fresh or report overall success. Old data must not silently authorize actions; existing adapter/Core checks remain authoritative. |
| Auxiliary sources/availability/Runtime read failed | Local area error + explicit retry; valid base snapshot stays visible | Never clear unrelated Workspace/MCP/Skill inventory solely for an auxiliary failure. |
| Profile / Environment changed or removed | Clear prior scope-bound details/output/Memory/observations | Late responses from a previous scope are discarded. |

Retain the established explicit connection/refresh and current-Environment read paths. Base counts should become visible after GetSnapshot succeeds without waiting for auxiliary reads. 18-01 does not add polling, per-Environment fan-out, automatic probes, source refresh or Memory reads; route changes by themselves perform no additional reads. Lazy page loading can be considered only in a later slice if measured load warrants it.

## 5. Frontend implementation boundary

| File / area | 18-01 change |
|---|---|
| frontend/index.html | Shell, section wrappers/headings/menu and dashboard links; move existing controls once, preserving IDs/forms. |
| frontend/styles.css | Scoped shell/navigation/dashboard states, hidden-page rule, min-size/long-text/focus styles. Avoid rewriting every manager's CSS. |
| frontend/navigation.js (new) | Small route table/normalization/activation helper, focus and modal/transition guard. No adapter access or persistence. |
| frontend/dashboard.js (new) | Presentation of supplied sanitized snapshot/data-state inputs; no own fetching or interpretation of private data. |
| frontend/app.js | Bind new helpers, explicit view load states, isolate base vs auxiliary read errors and reject stale scope-bound renders. Preserve current mutation handlers. |
| frontend/connections.js / dialogs.js | Only adjust shared shell lock, clearing and focus integration if needed. Preserve request serialization/profile transition behavior. |
| cmd/ai-dev-manager-desktop/main_test.go | Asset/binding checks for moved/new assets and retained capabilities; do not treat those checks as behavioral proof. |
| tests/desktop-ui/ (new) | Narrow routing/dashboard state and stale-response tests plus a browser fixture consuming the production assets with an injected fake Wails bridge. Keep fixtures out of embedded frontend assets. |

Keep the current deferred classic-script approach (dialogs, connections, new helpers, app), with collision-free helper APIs and no npm/Vite/React migration. Document helper loading order in the implementation. Keep `app.js` as the coordinator in 18-01; per-manager extraction belongs to later slices.

Transient data should be keyed by current connection identity and, where applicable, Environment ID/request generation. The existing serialized proxy is still used. Discard old-response rendering on a scope change; do not replay or retarget a pending mutation. Do not change Runtime authorization while improving presentation.

All production management continues via the existing Desktop adapter -> shared Admin MCP boundary. No REST layer, direct state.json access, new schema, Core endpoint or backend lifecycle change is authorized by 18-01.

## 6. Page refinement after the shell

The user requested all remaining Phase 18 slices be detailed before implementation so work can resume safely after interruption. They remain sequential execution plans and must be calibrated against predecessor evidence rather than treated as frozen code layouts:

| Plan | UI refinement using existing capabilities | Explicitly outside it |
|---|---|---|
| 18-02 | Workspace/Environment list-detail readability; clear root/writer/capability context; Runtime subviews and bounded output ergonomics. | Project discovery (19), async verifier protocol (21), new cleanup workflow (22). |
| 18-03 | MCP/Skill filters and state/reason hierarchy; explicit global vs Environment Memory scope; clearer system/diagnostic evidence using existing reads. | New MCP runtime, Skill interpreter, automatic Memory composition, broad Core parity expansion. |
| 18-04 | Mandatory cross-section accessibility, scaling/focus/empty-error/context-transition acceptance and exact Wails dogfood closeout. Evidence-backed UI fixes are allowed; speculative polish is not. | Installer/updater/signing, major theme system or new release train. |

Delivery order is 18-01 -> 18-02 -> 18-03 -> 18-04. Each successor first reconciles the actual prior implementation and may narrow file placement or duplicate checks, but it must not silently drop its scope/acceptance responsibilities. Phase 18 is complete only after the integrated 18-04 gate passes on the final UI.

## 7. Risks and review conditions

- Large DOM movement can lose control bindings or duplicate IDs: preserve one control instance and exercise every route/action family.
- Hidden ancestor / transition inert changes can make dialogs inaccessible: shared overlays and real browser focus checks are required.
- Single-page global state can leak previous target data: scope-keyed rendering tests cover quick Environment/profile switches and pending reads.
- Dashboard success can hide partial read failure: base and auxiliary status are distinct, with zero/unknown tested separately.
- Phase 18 can absorb every old parity gap: use the migration map as the preservation checklist; new capabilities go to STATE/appropriate later phase.
- No live visual evidence exists yet. Capture actual Wails smoke after implementation; do not claim a completed UI based on this design document.
