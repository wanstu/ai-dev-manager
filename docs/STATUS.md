# ADM V2 Development Status

> **Historical status document.** The stable product line is now **v1.2.0**; Phases 18–24 are complete, with Phase 25/26 remaining conditional. For current usage, read [`USER_GUIDE.md`](USER_GUIDE.md), [`cli.md`](cli.md), [`AGENT_GATEWAY.md`](AGENT_GATEWAY.md), and [`ARCHITECTURE.md`](ARCHITECTURE.md). For current development state use `.planning/STATE.md`; for product semantics use `PRODUCT_CONTRACT.md`.
>
> Planning reset (2026-09-06): `.planning/PROJECT.md`, `.planning/ROADMAP.md`, and `.planning/STATE.md` became the active development plan. This file and `docs/PHASE_*` are retained as historical implementation evidence. Statements below such as “Skill runtime partial” or “Phase 07 active” describe their historical checkpoint and are **not current v1.1 status**.

This file is the historical project-level view of implemented slices and prior development order.

`docs/PRODUCT_CONTRACT.md` defines product semantics. Phase documents describe delivered slices. This file tracks current implementation status and development order.

## Current milestone

First usable ADM V2: an Agent can develop software through MCP against an ordinary local directory without Git being a prerequisite.

The first usable ADM V2 milestone is accepted: the non-Git development loop passed through the Agent Gateway, and persisted context survived an actual temporary Gateway process restart and fresh MCP reconnect.

## Completed

### Git-independent development core

- Workspace registration for ordinary directories, including non-Git directories
- Environment creation rooted inside a Workspace
- tree / read / literal search
- write / exact edit / file delete
- single-writer lease per physical root
- explicitly allowlisted local command execution
- optional Git status / diff / branch capabilities
- persistent local state
- stdio and HTTP MCP Gateway

Tracked by `docs/PHASE_01_GIT_INDEPENDENT_CORE.md`.

### Development context

- Global MCP catalog: add / list / remove / set default
- Global Skill catalog: add / list / remove / set default
- Per-Environment MCP selections
- Per-Environment Skill selections
- Global Memory: list / read / write / delete
- Environment-private Memory: list / read / write / delete
- `environment_inspect`
- Gateway tools for the management capabilities above

Tracked by `docs/PHASE_02_DEVELOPMENT_CONTEXT.md`.

### Gateway lifecycle

- Foreground `gateway start`
- Detached `gateway start -d` / `--detach`
- `gateway status`
- `gateway stop`
- `gateway restart`

### Workspace lifecycle

- Workspace inspect by stable ID
- Workspace rename without moving the directory
- Workspace remove without deleting project files
- Removal is blocked while an Environment still references the Workspace

### Environment lifecycle management

- Environment rename by stable ID
- Rename changes only the human-readable name
- Root, Workspace association, MCP/Skill selections, private Memory, active Writer, and project files remain unchanged
- Environment list returns lightweight summaries with private Memory count but not values
- Environment inspect exposes Workspace relation, capabilities, resolved MCP/Skill entries, unresolved selection IDs, and private Memory count
- CLI and MCP share the same application-level Environment management view

Tracked by `docs/PHASE_04_ENVIRONMENT_MANAGEMENT_UX.md`.

### Exec allowlist lifecycle

- Add allowed executable
- List allowed executables
- Remove allowed executable
- Capability and execution permission update immediately after removal

### Management CLI parity — delivered slices

- Global MCP catalog CLI: add / list / remove / set default
- Global Skill catalog CLI: add / list / remove / set default
- Global Memory CLI: list / read / write / delete
- Per-Environment MCP selection CLI: enable / disable
- Per-Environment Skill selection CLI: enable / disable
- Environment-private Memory CLI: list / read / write / delete by explicit Environment ID
- CLI help exposes MCP, Skill, both Memory scopes, and Environment selection management

Tracked by `docs/PHASE_03_MANAGEMENT_CLI.md`.

### Management boundary

- Read-only installation snapshot backed by existing application services
- Thin named management mutations delegate existing validation and persistence
- Explicit Workspace/Environment detail and Memory read operations
- Wails-friendly pure-Go desktop adapter with exported methods and JSON-friendly inputs
- Overview/inspect responses remain sanitized; Memory values require explicit Memory reads
- No direct `state.json` access, second cache/store, or REST prerequisite

Tracked by `docs/PHASE_05_MANAGEMENT_BOUNDARY.md`.

### Desktop Management UI — shell and lifecycle

- Separate `ai-dev-manager-v2-desktop` executable entrypoint
- Wails v2 desktop shell binds the existing `desktop.Adapter`
- Uses the normal ADM state path and does not require a running Gateway
- Embedded plain HTML/CSS/JavaScript frontend; no npm/Vite/Node prerequisite
- Renders and refreshes Workspace, Environment, allowlist, MCP, Skill, and Global Memory snapshot counts
- Workspace add/rename/remove management through the existing adapter
- Environment create/inspect/rename/remove management through the existing adapter
- Environment detail uses the shared sanitized inspect view; private Memory values remain explicit-only
- Exec allowlist add/remove management through the existing adapter
- MCP/Skill catalog add/remove/default management plus per-Environment selection toggles
- Explicit Global and Environment-private Memory load/write/delete panels; Memory values stay out of Snapshot/inspect
- Destructive operations keep backend lifecycle/file-safety guards authoritative
- Desktop Gateway card reports stopped/running/incompatible and supports detached start / stop / refresh
- Desktop and CLI current-version Gateway health/termination paths share lifecycle helpers
- Desktop refuses incompatible endpoints instead of automatically killing an unknown listener
- Current Windows development machine successfully builds the desktop executable
- Official Wails release build is rooted at `cmd/ai-dev-manager-desktop` and exposed through `scripts/build-desktop.ps1`
- Built Wails release executable passed a short launch smoke against temporary ADM state

Tracked by `docs/PHASE_06_DESKTOP_UI.md`.

### First milestone dogfood

- Completed a real non-Git development loop through the Agent Gateway
- Observed a failing test, made a follow-up code change, and reran successfully
- Verified Git failure remains operation-local
- Verified persisted context across an actual temporary Gateway process restart and fresh MCP reconnect

Tracked by `docs/DOGFOOD_01_NON_GIT_LOOP.md` and `TestHTTPGatewayPersistsContextAcrossProcessRestart`.

## In progress / next slice

### Desktop dogfood / release readiness

Phase 07 is active. Slice A is delivered: the Wails release build now runs from `cmd/ai-dev-manager-desktop`, the root build script produces the packaged desktop executable, and the built executable passed a short launch smoke against temporary ADM state.

Slice B is now active against real ADM state. The first dogfood pass found four concrete blockers: Environment detail appeared non-functional because the rendered panel stayed below the visible viewport; routine CRUD success messages were incorrectly persistent page-level status; MCP/Skill selections were configuration records without an actual runtime/content integration; and Windows development commands could flash visible console windows over the desktop UI.

The current Slice B fix opens Environment detail in an in-place modal overlay without moving the page viewport, changes routine feedback to transient toast messages, requires real Streamable HTTP endpoints for newly managed MCP definitions and real instruction content for newly managed Skills, adds Environment-gated external MCP tool discovery/calls plus Environment Skill context to the Agent Gateway, and suppresses Windows console windows for Runtime command execution and Git probes. Legacy name-only catalog records are shown as unconfigured rather than usable integrations.

Concrete usability or lifecycle blockers continue to take priority; packaging metadata, tray, autostart, single-instance handling, notifications, installers, and other polish stay deferred until dogfood proves they are needed.

Expected management areas:

- Gateway status/lifecycle
- Workspaces
- Environments and Writer state
- Exec allowlist
- MCP catalog and Environment selections
- Skill catalog and Environment selections
- Global and Environment-private Memory

Wails keeps a Go backend while allowing an HTML/CSS/JS UI and leaves room for Windows/macOS/Linux support. OS-specific integrations should remain isolated behind platform-specific files.

## Later / not part of the immediate slice

These are not current prerequisites for normal ADM V2 development:

- Git worktree lifecycle
- clone/copy/container/VM isolation
- verifier configuration
- task/session attribution
- process/log/port management
- parallel Agent orchestration
- GSD workflow automation
- generic isolation/orchestration framework
- V1 state migration or compatibility

They should be pulled forward only by a concrete product or dogfood requirement.

## Known development-process incident

A previous set of feature worktrees was accidentally based on prior feature commits instead of the intended integration branch, creating unintended stacked branches and cherry-pick conflicts.

The incident is also recorded in ADM Global Memory under `development.process.worktree-base-incident`.

For independent feature work, the worktree base must be checked against the intended integration branch before development starts. Stacked branches should exist only when the dependency is intentional.

## Status maintenance

When a feature changes from planned to implemented, update this file in the same development slice so project status does not depend on chat history or memory.
