---
phase: 11-mcp-runtime-completion
plan: "03"
subsystem: mcp-import
tags: [mcp, import, jsonc, opencode, workbuddy, codex-plugin, claude-code, mcphub, gateway]
requires:
  - phase: 11-mcp-runtime-completion
    plan: "01"
    provides: Typed canonical MCP desired configuration and real HTTP/stdio activation
  - phase: 11-mcp-runtime-completion
    plan: "02"
    provides: Stable-ID MCP update, owner-local health/inventory observation and runtime invalidation
provides:
  - Sanitized MCP JSON/JSONC import preview for supported external source shapes
  - Atomic selected single/batch apply with error, skip and update_by_name conflict policies
  - Literal credential conversion into generated environment-reference requirements
  - Gateway Agent tools mcp_import_preview and mcp_import_apply
  - Management and CLI import-preview/import-apply surfaces backed by the same app boundary
  - Runtime-owner desired-definition fingerprint reconciliation for cross-entry update invalidation
affects: [12-skill-runtime-completion, 13-environment-capability-diagnostics, 15-desktop-core-parity]
implementation_commit: ea0d85af99f4591a431ba22dd8f1df036c76cac6
tech-stack:
  added: []
  patterns: [preview/apply split, normalized adapter boundary, atomic store update, generated reference requirements, desired fingerprint reconciliation]
key-files:
  created:
    - internal/app/mcp_import.go
    - internal/app/mcp_import_test.go
    - internal/app/env_template_test.go
    - internal/app/testdata/mcp_import/opencode.jsonc
    - internal/app/testdata/mcp_import/workbuddy.json
    - internal/app/testdata/mcp_import/codex-plugin.json
    - internal/app/testdata/mcp_import/claude-code.json
    - internal/app/testdata/mcp_import/mcphub.json
    - cmd/ai-dev-manager/mcp_import_cli_test.go
    - internal/gateway/runtime_owner_config_change_test.go
  modified:
    - cmd/ai-dev-manager/main.go
    - cmd/ai-dev-manager/main_test.go
    - internal/app/mcp_health.go
    - internal/catalog/mcp_service.go
    - internal/gateway/server.go
    - internal/gateway/server_test.go
    - internal/gateway/runtime_owner.go
    - internal/gateway/runtime_mcp_observation.go
    - internal/gateway/runtime_owner_test.go
    - internal/gateway/mcp_acceptance_test.go
    - internal/management/service.go
key-decisions:
  - "Importer compatibility is normalized at the app boundary; external source formats do not become ADM's internal MCP model."
  - "Preview is sanitized and side-effect-free; apply reparses source content and performs one atomic selected batch mutation."
  - "Name conflicts default to error; update_by_name preserves the ADM MCP ID and existing Environment selections."
  - "Literal header/env values are converted into generated environment-reference requirements; only reference templates are persisted."
  - "Imports write global MCP definitions only and never silently enable or disable existing Environments."
  - "Runtime owner reconciles persisted definition fingerprint changes so CLI/management/import updates invalidate old sessions even without a Gateway handler notification."
requirements-completed: [MCP-COMP-07, ADM-GW-003, ADM-CLI-001, ADM-MGMT-001]
completed: 2026-09-09
status: complete
---

# Phase 11 Plan 03: MCP JSON/JSONC Import Adapters Summary

**ADM can now preview and atomically import supported external MCP JSON/JSONC configurations into canonical global MCP definitions without leaking literal credentials or changing existing Environment selections.**

## Accomplishments

- Added app-level MCP importer with `PreviewMCPImport` and `ApplyMCPImport`.
- Added deterministic JSONC support for comments and trailing commas, while rejecting duplicate JSON object keys.
- Added supported source adapters for:
  - OpenCode;
  - WorkBuddy / CodeBuddy-style `mcpServers`;
  - Codex plugin MCP JSON direct server maps and `mcpServers` wrappers;
  - Claude Code top-level `mcpServers` and multi-project `projects.<scope>.mcpServers`;
  - supported MCPHub `servers` / `mcpServers` shapes.
- Added deterministic format detection. Generic `mcpServers` wrappers that match multiple adapters return `ambiguous_format` and require an explicit format choice.
- Added selected apply by MCP name and conflict policies: `error`, `skip`, and `update_by_name`.
- Added catalog-level atomic batch apply. The batch is fully normalized and validated before writing; failed batches do not partially persist.
- Added case-insensitive duplicate source-name detection in preview so `Foo`/`foo` conflicts are visible before apply.
- Converted literal environment/header values into generated reference requirements such as `${ADM_MCP_IMPORT_<NAME>_<FIELD>_<KEY>}`.
- Preserved auth prefixes like `Bearer ${REF}` while avoiding persistence or preview echo of literal credential-bearing values.
- Preserved `${VAR}` and `{env:VAR}` style references and added runtime support for `${VAR:-fallback}` activation templates used by some source configs.
- Added Gateway Agent tools `mcp_import_preview` and `mcp_import_apply`.
- Added management methods `MCPImportPreview` and `MCPImportApply` that reuse the same app importer boundary.
- Added CLI commands `mcp import-preview` and `mcp import-apply` for local management, using the same app boundary.
- Extended runtime owner with persisted-definition fingerprint reconciliation so definition changes made through Gateway, CLI, management or import apply invalidate stale owner sessions/observations consistently.

## Verification Evidence

### Import parsing and preview/apply behavior

- `TestMCPImportFixtureDetectionAndSanitizedPreview` covers deterministic fixtures for OpenCode, WorkBuddy, Codex plugin, Claude Code and MCPHub and verifies sanitized preview does not echo literal credential placeholders.
- `TestMCPImportAutoDetectionIsDeterministic` proves ambiguous generic `mcpServers` sources require explicit format selection and Claude multi-project sources require `source_scope`.
- `TestMCPImportCredentialConversionAndAtomicConflictPolicies` proves default conflict errors are atomic, `update_by_name` preserves ADM MCP ID and Environment selections, and generated header references persist without literal credential values.
- `TestMCPImportSelectedBatchRejectsInvalidCandidateWithoutPartialWrite` proves invalid selected candidates abort the whole batch without partial writes.
- `TestMCPImportRejectsDuplicateSourceKeys` rejects duplicate JSON keys before normalization.
- `TestMCPImportPreviewBlocksCaseInsensitiveDuplicateNames` surfaces case-insensitive source-name conflicts in preview and prevents partial apply.
- `TestMCPImportPreviewSurfacesUnsupportedSourceExtensions` surfaces unsupported adapter-specific extension fields as warnings.
- `TestMCPImportDefaultIncludeIsExplicit` proves imports do not default-enable future Environments unless explicitly requested.

### Runtime and Gateway acceptance

- `TestGatewayImportedHTTPMCPActivatesThroughRealRuntime` proves a Gateway-imported HTTP MCP can be explicitly enabled, then successfully list and call a real upstream tool through the Phase 11 runtime.
- `TestGatewayImportedStdioMCPActivatesThroughRealRuntime` proves a Gateway-imported stdio MCP still uses executable allowlist authority, Environment root cwd and activation-time env references.
- `TestGatewayImportUpdateByNameInvalidatesOwnedSession` proves Gateway `update_by_name` import closes stale owner sessions while preserving MCP ID and Environment selection.
- `TestRuntimeOwnerMonitorInvalidatesOutOfBandMCPConfigChange` proves the owner monitor invalidates sessions/observations after an out-of-band persisted definition update.
- `TestRuntimeOwnerReconcilesDefinitionUpdatesWithoutHandlerNotification` proves explicit access also reconciles out-of-band definition changes and reconnects with the new desired endpoint.
- `TestRuntimeOwnerStaleReconnectGenerationDoesNotBorrowUpdatedDefinition` proves a reconnect worker that started under an old generation exits before connect after desired configuration changes, so stale work cannot borrow the updated definition or create a duplicate recovery path.

### CLI, management and template behavior

- `TestMCPImportCLIUsesContentAndAtomicApply` proves CLI preview/apply uses JSON/JSONC content and writes canonical MCP definitions through the same app boundary.
- `TestExpandEnvironmentTemplateSupportsDefaultExpressions` and importer fallback coverage prove `${VAR:-fallback}` activation semantics work for unset or empty environment variables while unresolved required variables remain configured/unavailable.
- Management import methods are covered by the existing management service build/test gate and reuse app-level importer behavior.

### Regression gates

The following closeout gates passed on Windows:

- `go test -count=1 ./...` — full repository suite passed with exit code 0; `internal/gateway` completed in 49.310s, including the long verifier HTTP acceptance that earlier synchronous connector calls could not return visibly.
- `go test -race -count=1 ./internal/app ./internal/catalog`
- `go test -race -count=1 ./internal/gateway -run <importer/runtime-owner/update-reconciliation group>`
- targeted generation-safety coverage including `TestRuntimeOwnerStaleReconnectGenerationDoesNotBorrowUpdatedDefinition`, `TestRuntimeOwnerAllowsOnlyOneRecoveryInFlight`, and out-of-band definition reconciliation
- `go vet ./...`
- `git diff --check`

`git diff --check` completed with exit code 0 and no whitespace errors.

## Deviations / Additional Work

### 1. CLI import surface was added after the core Agent/management boundary

Plan 11-03 required Agent-facing Gateway and management import surfaces. CLI commands were added because they reuse the same app boundary and help local operators preview/apply imports during development. They do not create new product semantics: imports still write only global MCP definitions and never existing Environment selections.

### 2. Runtime owner fingerprint reconciliation was added to support cross-entry consistency

Gateway import apply can call `DropMCP` directly, but CLI and management may update persisted state from a different process. To keep owner-local sessions honest, runtime owner now tracks a sanitized desired-definition fingerprint per Environment/MCP and invalidates stale sessions/observations when persisted desired configuration changes.

This preserves the Plan 11-02 invariant that stale sessions are never reported healthy and ensures `update_by_name` from any entry point has the same runtime effect.

### 3. `${VAR:-fallback}` support was added at activation

Importer fixtures exposed Claude-style default expressions. Keeping them in persisted references without runtime support would produce misleading configured definitions. Activation template expansion now supports `${VAR:-fallback}` while preserving unresolved required-reference diagnostics.

## Security / Boundary Review

- Preview exposes names, transport/auth type, reference keys, warnings/errors and generated reference requirements, not resolved header/env secret values.
- Apply reparses the submitted source and persists canonical ADM definitions only; it does not persist source JSON or adapter-specific extensions.
- Literal credential-bearing values are converted to generated reference requirements and replaced by reference templates before persistence.
- Existing Environment selections are not changed by import add, skip or update-by-name.
- `update_by_name` preserves stable ADM MCP IDs, which preserves existing selections while invalidating old runtime state.
- Stdio imports still obey executable allowlist and Environment-rooted runtime authority.
- No Planner/GSD orchestration semantics were introduced.

## Phase 11 Closeout

Phase 11 is complete:

- Plan 11-01 completed typed desired MCP configuration and real HTTP/stdio transport activation.
- Plan 11-02 completed health/recovery/inventory/inspect/refresh and stable-ID runtime invalidation.
- Plan 11-03 completed preview/apply import adapters and atomic conflict/credential-reference policy.

Next roadmap work is Phase 12 Skill Runtime Completion. Do not start Phase 12 without an explicit authorization boundary.

---

*Phase: 11-mcp-runtime-completion complete*
*Plan: 11-03 complete; Phase 12 next*
