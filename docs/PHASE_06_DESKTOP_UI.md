# Phase 06 — Desktop Management UI

## Requirement IDs

- ADM-DESKTOP-001
- ADM-MGMT-001
- ADM-DEV-001
- ADM-DEV-003

## Goal

Add a cross-platform desktop management shell using Wails while keeping ADM product semantics in the existing application/management layers.

The desktop app is a human management surface. It does not replace the Agent Gateway and must not become a prerequisite for normal ADM development.

## Delivery slices

### Slice A — Desktop shell and snapshot

Status: delivered.

- add a separate `ai-dev-manager-v2-desktop` entrypoint
- use Wails v2 and bind the existing `desktop.Adapter`
- reuse `store.DefaultPath()` and the existing app/management services
- embed plain HTML/CSS/JavaScript assets directly in the Go binary
- render counts and lightweight lists from `GetSnapshot`
- provide an explicit Refresh action
- show loading and error states
- no npm/Vite/Node build prerequisite
- no REST or running Gateway prerequisite

### Slice B — Workspace / Environment management

Status: delivered.

- add Workspace registration form
- add Workspace rename/remove actions through the desktop adapter
- add Environment create form with explicit Workspace selection and optional root
- add Environment rename/remove actions through the desktop adapter
- add Environment detail panel using `InspectEnvironment`
- surface backend lifecycle/safety errors instead of duplicating those rules in JavaScript
- make destructive action copy explicit that ADM records are removed without deleting project directories/files

### Slice C — Catalog / allowlist / Memory management

Status: delivered.

- add/remove exec allowlist entries through the desktop adapter
- add/remove MCP and Skill catalog entries and change default-include settings
- enable/disable MCP and Skill IDs for the selected Environment only
- add explicit Global Memory load/write/delete panel
- add explicit Environment-private Memory load/write/delete panel scoped to the selected Environment
- keep Memory values out of Snapshot and Environment inspect responses
- preserve support for explicit empty Memory values
- keep catalog/default/selection validation in the existing management/application services

### Slice D — Gateway lifecycle and desktop polish

Status: delivered.

- add Gateway status, detached start, stop, and refresh controls to the desktop UI
- share HTTP health/status/termination helpers with the CLI instead of creating a second lifecycle implementation
- launch the same `gateway.RunHTTP` implementation through an internal detached desktop child mode
- require `/healthz` readiness before desktop start reports success
- refuse automatic stop/start recovery when the endpoint is incompatible
- keep legacy same-executable upgrade recovery in the CLI rather than duplicating it in the desktop
- keep tray, autostart, single-instance handling, notifications, and generic process management out of this phase

Phase 06 status: complete.

## Acceptance tests for Slice A

- `go test ./...` still passes
- desktop command compiles on the current Windows development machine
- desktop command uses the normal ADM state path
- bound object is `desktop.Adapter`
- frontend calls `GetSnapshot` through Wails binding
- snapshot page renders Workspace / Environment / allowlist / MCP / Skill / Global Memory counts
- refresh reloads snapshot state
- frontend assets are embedded and require no Node tooling
- CLI/Gateway command remains unchanged

## Acceptance tests for Slice B

- desktop command still compiles on the current Windows development machine
- frontend binds Workspace add/rename/remove through `desktop.Adapter`
- frontend binds Environment create/inspect/rename/remove through `desktop.Adapter`
- Workspace removal safety remains enforced by the backend management boundary
- Environment removal does not delete project files and active Writer protection remains backend-enforced
- Environment detail comes from the shared sanitized inspect view and does not dump private Memory values
- `go test ./...`, `go vet ./...`, and `git diff --check` pass

## Acceptance tests for Slice C

- desktop command still compiles on the current Windows development machine
- exec allowlist add/remove uses `desktop.Adapter`
- MCP/Skill add/remove/default mutations use `desktop.Adapter`
- per-Environment MCP/Skill toggles remain scoped to the selected Environment
- Global Memory values are read only after an explicit Global Memory load action
- Environment-private Memory values are read only after an explicit load action for the selected Environment
- Memory write forms allow explicit empty values
- Snapshot/inspect paths still do not expose Memory values
- `go test ./...`, `go vet ./...`, and `git diff --check` pass

## Acceptance tests for Slice D

- desktop Gateway status distinguishes stopped, running, and incompatible endpoints
- detached desktop start runs the same `gateway.RunHTTP` server and waits for `/healthz`
- desktop stop uses the shared Gateway termination helper and refuses incompatible endpoints
- CLI current-version Gateway health/termination paths reuse the shared lifecycle helpers
- legacy same-executable upgrade recovery remains CLI-only
- a built desktop executable can run the hidden Gateway child on a temporary loopback port and serve a valid `/healthz`
- `go test ./...`, `go vet ./...`, and `git diff --check` pass

## Explicit non-goals for Slice A

- Workspace/Environment mutation forms
- Memory value editor
- Gateway controls
- tray integration
- autostart
- single-instance handling
- notifications
- REST API
- Git/worktree lifecycle
- isolation/orchestration

## New prerequisites

- Wails v2 runtime/library for the separate desktop executable
- OS WebView runtime required by Wails on the target platform

These prerequisites apply only to the desktop executable. They are not prerequisites for CLI, MCP Gateway, Workspace, Environment, or normal Agent development.
