# Phase 16-03D Summary — CLI Admin MCP Convergence

## Outcome

Normal CLI management now uses the same Admin MCP contract as Desktop instead of directly opening and mutating ADM state.

Implementation commit: `ecf0516` — `feat(16-03d): route CLI management through Admin MCP`

## What changed

- Extracted the Desktop Admin MCP client into reusable `internal/adminmcp`.
- Production CLI routes `workspace`, `environment`/`env`, `exec`, `mcp`, `skill`, and `memory` through Admin MCP.
- Added management target selection:
  - default `http://127.0.0.1:43137`
  - `ADM_V2_URL`
  - top-level `--adm-url URL` / `--adm-url=URL`
- Connection failure does not fall back to the local state file.
- `gateway`, `doctor`, and `state` remain explicit local bootstrap/offline/recovery-oriented commands.
- Added Admin-only `environment_verifier_add` and `environment_verifier_remove` so verifier-definition management can converge without direct state mutation.
- CLI writer lease commands use the Admin MCP surface.
- Added app-level compatibility management methods used by direct handler tests; production top-level CLI selection still uses Admin MCP.
- Admin MCP client now preserves useful tool error text and normalizes/compacts optional arguments so nil Go maps/slices do not become invalid JSON `null` values against MCP schemas.
- Updated top-level CLI help to explain the Admin MCP model and the no-fallback rule.

## Safety / boundary behavior

- Agent MCP `/mcp` does not gain verifier-definition mutation tools.
- Verifier add/remove are Admin-only.
- Normal CLI management no longer selects a writable local state backend.
- Remote non-loopback server exposure/authentication is not enabled by this slice; that remains 16-03E/post-RC unless it becomes a blocker.

## Validation

Passed:

- focused CLI Admin MCP routing/no-fallback tests
- MCP/Skill CLI management through Admin MCP
- Global Memory CLI through Admin MCP
- verifier add/remove and writer acquire/release through Admin MCP
- Agent/Admin MCP inventory tests
- `go test -count=1 ./cmd/ai-dev-manager`
- `go test -count=1 ./...` (Gateway package: 56.624s)
- `go vet ./...`
- `go build ./cmd/ai-dev-manager`
- `go build ./cmd/ai-dev-manager-desktop`
- `git diff --check` (Windows LF→CRLF warnings only)

## Remaining RC work

1. Real Wails GUI dogfood for MCP/Skill management and connection/disconnection flows.
2. Fix only RC-blocking usability or parity defects found by that dogfood.
3. Re-run RC gate and GitHub Actions remotely when the branch is eventually pushed.
4. Keep authenticated non-loopback Admin MCP (16-03E), Phase 14 provider work, Phase 15 temporary lifecycle, and Phase 17 distribution post-RC by default.
