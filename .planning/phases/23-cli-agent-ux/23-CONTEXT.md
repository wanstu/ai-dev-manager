# Phase 23 Context — CLI Agent UX + MCP/Skill Provisioning

Date: 2026-09-13
Planning baseline: `bb1fc82` (`docs: close Phase 22 temporary environment acceptance`)
Status: COMPLETE. 23-01 MCP/Skill provisioning and diagnostics plus 23-02 Environment context/temporary lifecycle CLI and integrated fixed-head acceptance are delivered and validated.

## Goal

Make common ADM setup and Agent-support workflows easy to automate from the normal `adm` CLI without creating a second management API or CLI-only product semantics.

Phase 23 is a surface-convergence/usability phase over already delivered Core and Gateway capabilities. Normal CLI management remains an Admin MCP client. The CLI should make MCP/Skill provisioning, Environment diagnostics/context and the accepted temporary Environment lifecycle practical for shell scripts and humans while preserving stable-ID routing, operation-local optionality and the no-task-orchestration boundary.

## Contract mapping

Phase 23 implements/refines:

- ADM-GOAL-001/002 — useful Agent development setup without ADM task orchestration.
- ADM-CORE-003/004 — optional capability failures stay operation-local.
- ADM-CORE-012 — canonical global MCP/Skill catalog and Environment selection.
- ADM-CORE-015 — CLI human management reuses shared application state.
- ADM-CORE-017 — informative Environment views without private Memory leakage.
- ADM-CORE-018 — diagnosable MCP desired/runtime state and explicit refresh.
- ADM-CORE-019 — source-aware Skill availability without an ADM Skill interpreter.
- ADM-CORE-020 — authoritative Environment capability diagnostics.
- ADM-CORE-022 — explicit temporary Environment lifecycle and conservative cleanup.
- ADM-CLI-001 — scriptable CLI provisioning/context/lifecycle over Admin MCP.
- ADM-GW-001/003 — explicit stable IDs and local failures.
- ADM-MGMT-001 — one management/application boundary, no second persistence path.
- ADM-NONGOAL-001 — no task/GSD orchestration.

New prerequisites: none.

## Source calibration at `bb1fc82`

The current implementation already provides most Phase-23 backend semantics. The gaps are primarily CLI/client exposure and input ergonomics:

1. Production CLI routes normal `workspace` / `environment` / `exec` / `mcp` / `skill` / `memory` management through the shared Admin MCP client. `--adm-url` / `ADM_V2_URL` select the target; connection failure does not fall back to writable local state.
2. Successful normal management commands already emit pretty JSON. A broad new output framework is not required merely to call Phase 23 "script-friendly". `gateway`, `doctor` and `state` remain explicit local bootstrap/offline/recovery surfaces.
3. MCP import preview/apply currently accept only `--json-or-jsonc CONTENT`. RC3 dogfood recorded a real Windows PowerShell/native argument quoting failure for inline JSON and explicitly identified import-from-file as a future CLI convenience candidate.
4. Global `mcp probe` and Environment `mcp status` exist in CLI. Gateway/Admin MCP also already expose `environment_mcp_inspect` and `environment_mcp_refresh`, but `internal/adminmcp.Client` and CLI do not expose them.
5. Skill CLI has `source-list`, `source-add`, `source-refresh` and `source-remove`, but no source update command even though the Admin MCP client/Desktop already support `SkillSourceUpdate`. Source add currently accepts only one `--support-root` even though the source model supports multiple explicit support roots.
6. Global structural `SkillAvailabilityList` and Environment-specific `EnvironmentSkillList` / `EnvironmentSkillInspect` already exist in the Admin MCP client, but CLI does not expose them. Current `environment skill` CLI only enables/disables IDs.
7. Phase 20 delivered shared Agent/Admin `environment_context_bundle`; `internal/adminmcp.Client` and normal CLI do not expose it. Phase-20 planning explicitly deferred script-friendly CLI context output to Phase 23.
8. Phase 22 delivered shared Agent/Admin temporary Environment create/status/promote/cleanup. Desktop added Admin-MCP wrappers for status/promote/cleanup; normal CLI exposes none of the lifecycle. Phase-22 planning explicitly deferred CLI lifecycle UX to Phase 23.
9. `internal/adminmcp.Client.CapabilityReport` already calls the Gateway/Admin `environment_capability_report`, so current CLI help text claiming the production CLI returns only static app-level facts is stale and should be corrected rather than preserved as semantics.
10. Existing MCP/Skill/Core semantics are already accepted. Phase 23 should not redesign import normalization, MCP runtime policy, Skill discovery identity, temporary cleanup policy or Environment context composition.

## Locked design decisions

### 1. Admin MCP remains the normal CLI authority

Phase 23 adds no direct writable state fallback and no CLI-specific server protocol. New commands use the selected Admin MCP target exactly like current normal management.

`gateway`, `doctor` and `state` keep their existing explicit local bootstrap/offline/recovery role. Phase 23 does not turn them into a second management plane.

### 2. Stable scope stays explicit

No hidden current Workspace/Environment is added. Shell cwd is never treated as authorization or implicit Environment selection.

All Environment-specific commands require `--environment-id`. Temporary lifecycle mutations require explicit lifecycle owner input where the existing Core contract requires it.

### 3. Existing JSON is the automation baseline

Normal management already returns JSON on success. New provisioning/diagnostic/context/lifecycle commands also return canonical JSON values from the shared models.

Do not add a phase-wide table renderer, JSON envelope, jq-like query language or output-template subsystem. Human help remains text; command errors remain non-zero failures through the existing stderr path.

### 4. MCP import gets file/stdin transport convenience only

`mcp import-preview` and `mcp import-apply` accept exactly one of:

- `--json-or-jsonc CONTENT` — current inline form;
- `--file PATH` — read by the local CLI process;
- `--stdin` — explicitly read redirected stdin.

Preview/apply must share one helper so source selection/validation cannot diverge. Multiple simultaneous sources fail before calling Admin MCP. `--stdin` against an interactive terminal fails clearly instead of waiting ambiguously.

Local file/stdin reading changes only how content reaches the existing importer. It does not persist raw import blobs, weaken literal-credential reference conversion, auto-enable existing Environments or introduce filesystem authority on the Gateway host.

### 5. MCP diagnostic commands keep distinct meanings

CLI help and commands must distinguish:

- `mcp probe --id MCP_ID` — global bounded transient configuration/connection probe, no Environment selection;
- `mcp status --id MCP_ID --environment-id ENV_ID` — selected-Environment one-shot status probe;
- `mcp inspect --id MCP_ID --environment-id ENV_ID` — passive sanitized desired config + owner-local observation/inventory evidence;
- `mcp refresh --id MCP_ID --environment-id ENV_ID` — explicit owner-runtime discard/reconnect/Ping/inventory refresh.

`inspect` must not connect/probe. `refresh` may change owner-local observation/session state only; it does not call business tools, modify the catalog, alter Environment selection or require a writer.

### 6. Skill source configuration and availability stay separate

Add CLI source update over the already existing shared source model. `source-add` and `source-update` support repeated `--support-root` flags so the CLI can represent the canonical list without comma parsing.

Source update does not implicitly refresh. Refresh remains an explicit command.

Expose:

- global structural Skill availability independent of Environment selection;
- Environment-specific Skill availability list/inspect independent of artifact-content reads.

Availability/list commands do not read full `SKILL.md`, execute Skill instructions, or mutate source/catalog state.

### 7. Environment context is the Phase-20 bundle, not a new summary

Preferred CLI command:

`adm environment context --environment-id ENV_ID [--path REL] [--max-depth N --max-entries N --max-digest-entries N --max-output-bytes N]`

It calls the existing shared `environment_context_bundle` through Admin MCP and emits the same bounded model. No hidden selection, writer acquisition, MCP probe/connect, verifier/process/Run execution, Memory value injection or full Skill content read is added.

Correct the stale `capability-report` help so it reflects the current Admin-MCP/Gateway-owner enriched production path.

### 8. Temporary Environment CLI is a direct Phase-22 lifecycle projection

Preferred CLI shape:

- `adm environment temporary create --workspace-id WS_ID --name NAME --owner-id OWNER --ttl-seconds N [--session-id ID] [--run-id ID] [--mode existing_root|managed_worktree] [--root PATH] [--base-ref REF]`
- `adm environment temporary status --environment-id ENV_ID`
- `adm environment temporary promote --environment-id ENV_ID --owner-id OWNER`
- `adm environment temporary cleanup --environment-id ENV_ID --owner-id OWNER [--execute]`

Cleanup defaults to preview. There is no `--force` option. The CLI does not invent a lifecycle owner, creator surface, current Run or task identity. `run_id`/`session_id` remain provenance only.

Because the CLI calls the existing Admin MCP tool, persisted creator-surface authority remains the Admin MCP surface; the CLI must not forge or override it.

### 9. No opportunistic Phase-24 refactor

Phase 23 may add small reusable CLI input/flag helpers needed by its commands, but it does not perform the planned Desktop/CLI package boundary split. Keep changes inside existing CLI/Admin-MCP seams unless a direct blocker requires a minimal shared helper.

## Explicit non-goals

1. No second Admin API, direct normal-mode `state.json` writes or fallback management backend.
2. No implicit current Workspace/Environment, cwd-based authorization or shell-session project state.
3. No new MCP transport/import normalization semantics, OAuth lifecycle or secret store.
4. No automatic MCP enablement for existing Environments after import; no business-tool calls during inspect/refresh.
5. No Skill execution engine, prompt interpretation, automatic artifact reads or source refresh on source edit.
6. No universal CLI formatter/table/template/query subsystem.
7. No task record, planner, GSD state advancement, parent/child Agent workflow or automatic Git integration.
8. No force temporary cleanup, automatic GC, branch deletion, merge/rebase/push or hidden lifecycle owner defaults.
9. No Desktop feature work in Phase 23.
10. No Phase-24 package/surface split, release/tag/push or subagents.

## Delivery sequence

### 23-01 — MCP/Skill provisioning and diagnostics

Add local file/stdin MCP import input, owner-runtime MCP inspect/refresh CLI parity, Skill source update/multi-support-root provisioning and global/Environment availability diagnostics. Keep every operation on Admin MCP and prove no implicit mutation/probe/content-read side effects.

### 23-02 — Environment Agent workflows and integrated closeout

Add the Phase-20 Environment context bundle and Phase-22 temporary Environment lifecycle to CLI, correct stale help, prove no-fallback/stable-ID/JSON behavior through real Admin MCP acceptance, then run fixed-head full Go/vet/CLI build acceptance and close Phase 23.

Do not start Phase 24 automatically after closeout.
