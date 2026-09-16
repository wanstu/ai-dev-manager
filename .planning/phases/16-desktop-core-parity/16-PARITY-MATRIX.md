# Phase 16 Desktop Core Parity Matrix

## Priority legend

- **P0 / RC blocker:** unsafe or unusable without this.
- **P1 / RC target:** strongly expected for a useful Desktop RC; may use documented CLI/Gateway fallback only if implementation risk is too high.
- **P2 / Post-RC:** useful, but not required for local 1.0 RC.
- **N/A:** intentionally not a Desktop concern.

## Current inventory

| Core area | Core surface exists | Current Desktop exposure | Gap | RC priority |
|---|---:|---|---|---|
| Gateway lifecycle | Yes | Status/start/stop via adapter/frontend | Needs smoke confirmation only | P0 |
| Workspace list/add/rename/remove/inspect | Yes | Exposed | Needs smoke confirmation only | P0 |
| Environment list/create/rename/remove/inspect | Yes | Exposed | Needs smoke confirmation only | P0 |
| CapabilityReport / CapabilityFact | Yes | MCP/Skill and Environment detail now surface relevant state/reason evidence | Broader all-capability presentation can follow dogfood if needed | P1 |
| MCP typed catalog add/default/remove | Yes | 16-02 adds guided Streamable HTTP/stdio config, defaults and explicit global delete semantics | Manual GUI dogfood only | P0 |
| MCP Environment selection | Yes | 16-02 adds explicit Management Environment context and per-definition enable toggle | Manual GUI dogfood only | P0 |
| MCP health/status/probe | Yes | 16-02 adds explicit per-MCP probe; refresh never probes automatically | Manual GUI dogfood only | P0 |
| MCP import adapters | Yes | 16-02 adds JSON/JSONC preview/apply with warnings/errors/reference requirements | Manual GUI dogfood only | P0 |
| MCP tool inventory/refresh | Yes through Gateway owner surfaces | Not exposed in Desktop | Post-RC unless needed during smoke | P2 |
| Skill source add/list/refresh/remove | Yes | 16-02 adds source add/list/refresh/remove/status | Manual GUI dogfood only | P0 |
| Skill Environment selection | Yes | 16-02 adds explicit Management Environment context and per-Skill enable toggle | Manual GUI dogfood only | P0 |
| Skill availability/support inventory/read diagnostics | Yes | 16-02 shows availability state/reason and source/artifact context | Support file browsing/read can be post-RC | P1 |
| Exec allowlist | Yes | Add/remove/list exposed | Needs smoke confirmation only | P0 |
| Exec command execution | Yes | Not exposed | CLI/Gateway fallback acceptable for RC; Desktop execution UI can be post-RC | P2 |
| File tree/read/search/write/edit/delete | Yes | Not exposed as file manager | CLI/Gateway fallback acceptable for RC; do not build a broad editor for RC | P2 |
| Verifier list/run/results | Yes | Exposed in RC1 Runtime panel; run requires an already-active writer | Manual GUI dogfood only | P1 |
| Process lifecycle/logs/ports | Yes | Exposed in RC1 Runtime panel with list/logs/ports/stop | Manual GUI dogfood only | P1 |
| Generic run lifecycle | Yes | Exposed in RC1 Runtime panel with list/output/cancel | Manual GUI dogfood only | P1 |
| Git status/diff/branch | Yes | Not exposed | Useful but optional; CLI fallback acceptable | P2 |
| Managed worktree isolation | Yes | Not exposed | Optional; post-RC unless blocking daily use | P2 |
| Global Memory | Yes | List/read/write/delete exposed with explicit load/read path | Ensure snapshot does not leak values | P0 |
| Environment-private Memory | Yes | List/read/write/delete exposed after explicit Environment detail load | Ensure inspect/snapshot expose count only by default | P0 |
| 14-01 endpoint investigation | Yes | Gateway only | Post-RC; not required for Desktop RC | P2 |
| GitNexus/provider integration | Planned only | Not exposed | Post-RC unless dogfood proves blocker | P2 |
| Temporary resource lifecycle | Planned only | Not exposed | Post-RC unless unmanaged resources block daily use | P2 |
| CI auto build | Added in Phase 16 | GitHub Actions workflow | Local gate passed; observe a real GitHub run before public promotion | P0 |

## Initial RC implementation targets

1. ✅ CI baseline and local smoke build.
2. ✅ MCP visual management: typed add/import/list/default/Environment selection/explicit health probe.
3. ✅ Skill visual management: source add/list/refresh/remove, Skill list, Environment selection, availability reasons.
4. ✅ Relevant CapabilityReport facts integrated into MCP/Skill and Environment detail views.
5. ⏳ Real Wails GUI human click-through remains RC dogfood; automated launch and binding smoke are complete.
6. ✅ Verifier/process/run visibility added for RC1 without adding command-start/orchestration semantics.
7. ✅ RC1 smoke evidence and known limitations recorded in `16-RC1-NOTES.md`.

## Keep out of RC unless proven blocker

- Full file manager/editor.
- Full Git/worktree UI.
- GitNexus/provider integration.
- Temporary resource cleanup implementation.
- Installer/tray/autostart/updater/signing.
