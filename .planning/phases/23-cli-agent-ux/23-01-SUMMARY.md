# 23-01 Summary — MCP/Skill provisioning and diagnostics

Date: 2026-09-13
Planning baseline: `2e4e575` (`docs: plan Phase 23 CLI agent UX`)
Implementation commit: `b733948` (`feat(cli): improve MCP and Skill provisioning`)
Status: complete and validated; 23-02 ready, not started.

## Delivered

Phase 23-01 closes the planned normal-CLI gaps without creating a second management plane. All new management commands continue to use the selected Admin MCP target; connection failure does not fall back to writable local `state.json` management.

### MCP import input ergonomics

`mcp import-preview` and `mcp import-apply` now share one CLI-only content resolver and accept exactly one of:

- `--json-or-jsonc CONTENT`;
- `--file PATH`;
- `--stdin`.

File content is read by the local CLI before the Admin MCP request. Explicit stdin rejects interactive-terminal use instead of waiting ambiguously. Zero or multiple sources fail before apply. The CLI does not persist raw import blobs or print raw import content in source-selection errors; existing importer normalization, atomicity and credential-reference conversion remain authoritative.

### MCP runtime diagnostics

Added thin Admin MCP client/backend/CLI projections for the already-existing owner-runtime operations:

- `adm mcp inspect --id MCP_ID --environment-id ENV_ID` — passive sanitized desired config + owner-local observation/inventory evidence; no connect/Ping/refresh;
- `adm mcp refresh --id MCP_ID --environment-id ENV_ID` — explicit owner-runtime reconnect/Ping/inventory refresh.

Existing meanings remain distinct:

- `mcp probe` is a global transient management probe;
- `mcp status` is an Environment-scoped one-shot status probe.

Refresh does not call MCP business tools, change the global definition, change Environment selections or require a writer.

### Skill provisioning and availability

- `skill source-add --support-root` is repeatable and maps directly to the canonical support-root list.
- Added `skill source-update --id SOURCE_ID --root PATH [--support-root PATH ...] [--default=true|false]` over the existing Admin MCP source-update operation.
- Omitting `--default` on source update preserves the current default-inclusion setting.
- Source update does not implicitly refresh; `source-refresh` remains explicit.
- Added `skill availability` for global structural availability independent of Environment selection.
- Added `environment skill list --environment-id ENV_ID` and `environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID` for Environment-specific availability.

Availability commands do not read full Skill instructions/support-file contents, execute Skills or mutate source/catalog state.

## P01–P12 acceptance

| Gate | Result |
|---|---|
| P01 | PASS — file-backed import preview matches the same normalized candidate semantics as inline content. |
| P02 | PASS — redirected stdin import apply succeeds, avoiding native-shell inline JSON quoting requirements. |
| P03 | PASS — zero/multiple content sources fail before apply and leave MCP catalog state unchanged. |
| P04 | PASS — credential-bearing literals are converted to references and remain absent from normal preview/persisted state. |
| P05 | PASS — probe/status/inspect/refresh remain separate discoverable CLI operations with distinct help/semantics. |
| P06 | PASS — passive inspect makes zero upstream requests without prior observation; refresh performs protocol lifecycle work and invokes zero business tools. |
| P07 | PASS — refresh preserves MCP desired definition and Environment enabled-ID selection. |
| P08 | PASS — Skill source add/update round-trip multiple support roots; update does not refresh until explicit source-refresh. |
| P09 | PASS — global structural availability is independent of Environment enablement; Environment list/inspect report disabled/broken states without artifact-content reads. |
| P10 | PASS — broken MCP/Skill/source behavior remains operation-local; plain non-Git Workspace/Environment management remains valid. |
| P11 | PASS — new commands use the normal Admin MCP no-fallback path when the selected management target is unavailable. |
| P12 | PASS — successful new commands emit valid JSON; help remains text and failures use the existing non-zero error path. |

Real acceptance lives in `cmd/ai-dev-manager/phase23_cli_test.go` and exercises the production top-level CLI target selection against disposable HTTP Gateway/Admin MCP owners rather than only helper mocks.

## Final validation on `b733948`

- Focused CLI/Admin-MCP/MCP/Skill gate: `run_06b50106d7daaae3` — PASS.
  - `go test -count=1 ./cmd/ai-dev-manager ./internal/adminmcp ./internal/gateway ./internal/catalog ./internal/skill -run "Phase23|MCP|Skill|AdminMCP"`
  - CLI 0.908s; Gateway 1.150s; catalog 0.148s; Skill 0.122s.
- Phase-23 CLI repeat: `run_fa71e6ac63dd1dac` — PASS.
  - `go test -count=3 ./cmd/ai-dev-manager -run "^TestPhase23"`
- Full repository: `run_59eda237328f72de` — PASS.
  - `go test -count=1 ./...`
  - Gateway 184.948s; all packages green.
- Vet: `run_7a57d5e80b84d6c6` — PASS (`go vet ./...`).
- `git diff --check` — PASS on the clean fixed implementation head.

No build artifact is required by 23-01; the Phase-23 fixed-head CLI artifact is reserved for 23-02 integrated closeout.

## Boundary notes

No direct normal-mode state-file writes, hidden current Environment, cwd-based authorization, new MCP import/runtime semantics, automatic Skill refresh, Skill execution engine, universal CLI formatter, task/GSD orchestration, Desktop feature work, Phase-24 refactor, push/tag/release or subagents were introduced.

23-02 may now expose the already accepted Phase-20 Environment context bundle and Phase-22 temporary Environment lifecycle through the same Admin MCP client. Do not start it in this execution node.
