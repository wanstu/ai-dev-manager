# Phase 07 — Desktop dogfood and release readiness

## Requirement IDs

- ADM-CORE-012
- ADM-DESKTOP-001
- ADM-DEV-001
- ADM-DEV-003
- ADM-DEV-004

## Goal

Turn the Phase 06 desktop manager from a development-only `go build` target into a reproducible Wails release build, then use that build for concrete desktop dogfood before adding speculative desktop features.

## Delivery slices

### Slice A — Reproducible Wails release build

Status: delivered.

- keep `wails.json` beside the desktop Go `main` in `cmd/ai-dev-manager-desktop`
- build from that Wails project root so binding generation sees the correct Go package
- provide `scripts/build-desktop.ps1` as the root-level release build entrypoint
- pin the invoked Wails CLI to v2.15.0
- ignore generated Wails bindings and build output
- verify the official Wails build produces `build/bin/ai-dev-manager-v2-desktop.exe`
- smoke-launch the built desktop executable against a temporary ADM state directory and verify the process remains alive

### Slice B — Desktop dogfood

Use the built desktop executable against real ADM state for representative management tasks. Record and fix concrete usability or lifecycle blockers before pulling forward tray, autostart, single-instance handling, notifications, installers, or other desktop polish.

First real-state dogfood findings:

- Environment detail loaded below the visible management grid without moving the viewport, making the Detail action appear broken in the release window.
- CRUD success feedback was rendered in a persistent page-level status banner instead of transient local feedback.
- MCP and Skill catalog entries only stored names/default selections; Environment selections did not yet connect to an external MCP runtime or provide actual Skill instructions to the Agent Gateway.
- Windows development commands launched from the GUI/Gateway could briefly create visible console windows, causing repeated black-window flashes during Agent work.

Slice B fixes:

- opening Environment detail now uses an in-place modal overlay instead of moving the page viewport
- routine operation feedback is a transient toast; success feedback clears automatically and errors are time-bounded
- MCP definitions now require a Streamable HTTP endpoint through management creation surfaces
- Skill definitions now require actual instruction content through management creation surfaces
- the Agent Gateway can list/call tools on an external MCP only when that configured MCP is enabled for the target Environment
- the Agent Gateway can read actual Skill instructions enabled for an Environment
- legacy name-only MCP/Skill records remain readable but are explicitly shown as unconfigured instead of being presented as usable integrations
- Windows Runtime command execution and Git probes suppress console windows instead of flashing terminal windows over the desktop UI

The first external MCP runtime transport is Streamable HTTP. Additional transports such as stdio are not pulled into this dogfood fix unless real use requires them.

## Acceptance tests for Slice A

- running Wails from the repository root without the corrected project layout is known to fail with `no Go files`
- `cmd/ai-dev-manager-desktop/wails.json` is colocated with the desktop Go main
- no root `wails.json` remains
- `scripts/build-desktop.ps1` runs Wails from the desktop command directory
- the script completes successfully on the current Windows development machine
- the release executable is created under `cmd/ai-dev-manager-desktop/build/bin`
- the release executable starts and remains alive during a short smoke run using temporary ADM state
- generated `frontend/wailsjs` and `build` output are ignored
- `go test ./...`, `go vet ./...`, and `git diff --check` pass

## Explicit non-goals for Slice A

- tray integration
- autostart
- single-instance handling
- notifications
- installer design
- code signing
- automatic updates
- REST API
- Git/worktree lifecycle
- isolation/orchestration

## New prerequisites

None beyond the Wails v2 desktop dependency already introduced in Phase 06.
