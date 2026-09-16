# Phase 24 Context — Desktop/CLI Surface Boundary Split

Date: 2026-09-14
Planning baseline: `2b14a57` (`docs: add comprehensive v1.1 user guides`)
Status: PLANNED / OPENED. No feature implementation has started.

## Goal

Separate the Desktop and CLI presentation surfaces enough that each can evolve, test and package independently while preserving one shared Core, one persisted state model and one normal management transport.

Phase 24 is an internal structure/refactor phase. It must make ownership boundaries clearer without changing product semantics delivered through Phases 10-23.

## Contract mapping

Phase 24 implements/refines:

- ADM-GOAL-001/002 — keep the development control plane useful without introducing task orchestration.
- ADM-CORE-003/004 — optional capability failures remain operation-local.
- ADM-CORE-015/017/020 — human management and diagnostics continue to reuse canonical application models.
- ADM-CLI-001 — normal CLI management remains a thin Admin-MCP client with explicit stable scope.
- ADM-MGMT-001 — one management boundary and no second persistence path.
- ADM-DESKTOP-001 — Desktop remains a management surface over the same Core/Admin MCP authority.
- ADM-GW-004 — local Gateway bootstrap/lifecycle semantics remain shared rather than surface-specific forks.
- ADM-SURFACE-001 — Desktop and CLI are distinct presentation packages over shared Admin MCP/Core contracts.
- ADM-NONGOAL-001 — no task/GSD orchestration.

New prerequisites: none.

## Source calibration at `2b14a57`

1. `cmd/ai-dev-manager/main.go` is still the CLI surface implementation and is roughly 90 KB. Command parsing, help/output, normal Admin-MCP management commands, explicit local `gateway`/`doctor`/`state` recovery commands and Gateway lifecycle helpers all live in `package main`.
2. `cmd/ai-dev-manager/admin_connection.go` owns CLI-specific ADM target parsing and URL normalization. It appends `/admin/mcp` itself and implements loopback-base-URL-to-listen conversion.
3. `cmd/ai-dev-manager/management_backend.go` defines one broad CLI backend plus capability-specific optional interfaces so tests can substitute fakes while the production implementation is `*adminmcp.Client`.
4. `internal/desktop` already provides a Desktop package, but `adapter.go` currently combines Wails-facing DTOs, ADM connection inspection, local Gateway bootstrap, Admin-MCP client installation, management methods and runtime methods in one file. `backend.go` defines broad Desktop-local management/runtime interfaces.
5. `internal/adminmcp.Client` is already the shared normal-management transport used by both CLI and Desktop. It is the correct place for Admin-MCP protocol/tool wrappers; Phase 24 must not create another client protocol.
6. CLI and Desktop duplicate Base URL / loopback validation. `internal/gateway/lifecycle.go` already owns canonical HTTP Gateway status, health URL derivation, `/mcp` and `/admin/mcp` endpoint facts and local lifecycle behavior, but its URL normalization/local-listen helpers are not currently shared by both surfaces.
7. Production Desktop uses `desktop.NewClientAdapter()` and installs `*adminmcp.Client` only after a successful ADM connection. `desktop.NewAdapter(management.Service)` exists only for tests/offline-style direct adapter exercise; production must not silently fall back to it.
8. Desktop frontend already talks only to the Wails-bound adapter and does not directly fetch `/admin/mcp`; Phase 18 extracted several frontend helper modules. Phase 24 does not need a visual redesign or new frontend framework.
9. Release artifacts are already correctly user-facing as `adm` and `adm-desktop`, while source directories remain `cmd/ai-dev-manager` and `cmd/ai-dev-manager-desktop`. A module-path, repository or source-directory rename is not required for product clarity.
10. Current surface tests are concentrated under `cmd/ai-dev-manager` and `internal/desktop`, so package extraction must retain real Admin-MCP/no-fallback acceptance rather than replacing it with only unit mocks.

## Locked design decisions

### 1. One Core/state/management authority remains mandatory

There is still one persisted ADM desired state and one Core/application semantic model.

Normal CLI and Desktop management continue through the same Admin MCP surface and `internal/adminmcp.Client`. No second REST API, local writable fallback, Desktop-only persistence, CLI-only state or duplicated business validation may be introduced.

Explicit local CLI `gateway`, `doctor` and `state` behavior remains a bootstrap/offline/recovery exception, not a peer normal management plane.

### 2. Surface separation is package ownership, not product duplication

CLI owns command grammar, flags, shell/stdin/file ergonomics, text help and JSON/stdout/stderr presentation.

Desktop owns Wails-facing DTOs, connection profiles, local-start UI integration and browser/frontend presentation state.

Core/application/Gateway/Admin-MCP packages own product behavior and transport semantics. A surface package must not reimplement MCP/Skill/Environment/retention/writer/runtime business rules.

CLI and Desktop must not import each other.

### 3. Reuse shared connection-target semantics

Base URL normalization, derived health/Agent MCP/Admin MCP endpoints and loopback-local lifecycle eligibility must come from one shared implementation rather than CLI and Desktop maintaining subtly different URL rules.

The preferred implementation is to extend/reuse the existing Gateway HTTP target/lifecycle helpers rather than invent a new transport protocol or persisted connection model.

Surface-specific configuration remains local: CLI still reads `--adm-url` / `ADM_V2_URL`; Desktop still stores its connection profiles.

### 4. Keep surface-local narrow interfaces for tests

Do not replace current surface-local test seams with one giant cross-surface interface package. The concrete shared production client is `*adminmcp.Client`; each surface may define the smallest interfaces it needs for substitution/testing.

Compile-time/test assertions should prove the production Admin MCP client satisfies those interfaces.

### 5. Extract CLI implementation behind a thin command entrypoint

Phase 24 should create a real internal CLI package so `cmd/ai-dev-manager` becomes process bootstrap rather than the home of all command behavior.

The internal CLI package should own command routing/help/input and the explicit local recovery commands. OS-specific detached Gateway process helpers may move with the CLI surface as needed.

`main` should remain responsible only for process-level startup concerns such as loading executable-adjacent `.env`, invoking the CLI runner, printing the final error and choosing the exit code.

No CLI command names, flags, success JSON shapes or failure semantics change as part of this extraction.

### 6. Split Desktop adapter responsibilities without changing the Wails contract

Keep `internal/desktop` as the Desktop surface package but separate connection/bootstrap, management facade, runtime facade and DTO/input responsibilities into focused files/components.

Existing Wails-bound exported method names and frontend behavior remain stable unless a direct compile-time requirement forces a mechanical rename. The frontend continues to call the Desktop adapter rather than Admin MCP directly.

Production `NewClientAdapter()` remains Admin-MCP-only. Any direct local management backend constructor remains explicitly test/offline-oriented and must not become an automatic fallback.

### 7. Build/release naming is already correct

Phase 24 preserves:

- user-facing CLI name `adm`;
- user-facing Desktop name `adm-desktop`;
- current GitHub Release artifact names;
- current Go module path and single repository.

Acceptance should make this distinction explicit, but Phase 24 does not rename the Go module, split repositories or move source directories merely for cosmetic symmetry.

### 8. This phase is refactor-only from a product perspective

No new MCP transport, Skill semantics, Environment lifecycle, Memory composition, Runtime resource, auth/TLS system, installer/updater, task orchestration or investigation helper is introduced.

Any behavior difference discovered during refactor is treated as a regression unless it is an existing bug required to complete the boundary split and is documented with focused acceptance.

## Explicit non-goals

1. No multi-repo split, Go module rename or compatibility migration.
2. No second state file/cache, REST management API or writable local fallback.
3. No new Desktop feature/visual redesign or CLI feature expansion.
4. No new MCP/Skill/Memory/Environment/Runtime semantics.
5. No remote Admin MCP authentication/TLS work; that remains separately deferred.
6. No installer/updater/signing/notification work from Phase 25.
7. No Phase-26 investigation helpers.
8. No task/GSD orchestration, hidden current Environment or automatic Git integration.
9. No push/tag/release as part of normal Phase-24 implementation nodes unless explicitly requested.
10. No subagents.

## Delivery sequence

### 24-01 — Shared ADM target and management boundary cleanup

Centralize Base URL/endpoint/local-loopback target semantics, make both surfaces consume the same target rules and strengthen production-client/no-fallback contract tests. Keep all user-visible behavior unchanged.

### 24-02 — CLI package extraction

Move the CLI implementation out of `cmd/ai-dev-manager` into a dedicated internal CLI surface package while leaving a thin executable bootstrap. Preserve every existing command, JSON/error contract and the explicit local bootstrap/recovery exceptions.

### 24-03 — Desktop adapter separation and integrated acceptance

Split Desktop adapter responsibilities into focused surface components/files without changing the Wails/frontend contract, add cross-surface boundary assertions, then run fixed-head CLI/Desktop/full Go/vet/browser/Wails/build acceptance and close Phase 24.

Do not start Phase 25 automatically after closeout.
