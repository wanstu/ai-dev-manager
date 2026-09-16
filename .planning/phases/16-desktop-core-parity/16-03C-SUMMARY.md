# Phase 16-03C Summary — Desktop Management Through Admin MCP

## Result

Completed the Desktop management-plane convergence for normal operation: production Desktop no longer opens ADM persisted state directly for management CRUD.

Implementation commit: `0dfe01d` — `feat(16-03c): route Desktop management through Admin MCP`

## Delivered

- Added Admin-only `management_snapshot` for the safe Desktop/CLI overview.
  - returns Workspace/Environment/allowlist/MCP/Skill summaries and Global Memory count;
  - does not expose Global Memory values or Environment-private Memory values.
- Added Desktop Admin MCP management client covering the existing Desktop management backend contract.
- Production Desktop now starts with `desktop.NewClientAdapter()` and no writable local management backend.
- `ConnectADM` performs liveness inspection and binds the management backend to the selected `/admin/mcp` only when ADM is running.
- Stopped/unreachable/incompatible connection state clears the active management backend.
- Local ADM bootstrap remains separate; after successful local start, Desktop binds to that service through Admin MCP.
- Stopping local ADM clears the management backend.
- Frontend startup/refresh/profile changes connect first, then load management data only from Admin MCP.
- When ADM is disconnected, Desktop clears Workspace/Environment/MCP/Skill/Memory overview instead of showing local state-file data.
- Direct `app.New(statePath)` remains only in the explicit `--gateway-child` local service bootstrap path.
- `desktop.NewAdapter(management.Service)` remains available only as an explicit local backend constructor for tests/offline-recovery callers; it is not the production Desktop path.

## Acceptance evidence

- A disconnected production-style client adapter cannot return a management snapshot.
- After connecting to a real HTTP Gateway, Desktop Workspace CRUD is persisted in the Gateway-owned ADM state.
- After the Gateway is stopped, reconnect reports stopped and subsequent management calls fail instead of falling back to a local state file.
- Agent `/mcp` does not expose `management_snapshot`; Admin `/admin/mcp` does.
- `management_snapshot` regression test verifies Global Memory material is not leaked.

## Validation

Passed for this implementation:

- `go test -count=1 ./internal/gateway` via ADM-owned process: `ok ai-dev-manager-v2/internal/gateway 74.669s`;
- `go test -count=1 ./internal/desktop ./cmd/ai-dev-manager-desktop`;
- full `go test -count=1 ./...` via ADM-owned process; Gateway portion `65.978s`, all packages green;
- `go vet ./...`;
- `go build ./cmd/ai-dev-manager ./cmd/ai-dev-manager-desktop`;
- `git diff --check` (Windows LF→CRLF warnings only);
- after final `gofmt`, Desktop focused tests and Agent/Admin HTTP inventory focused test passed again.

The JavaScript file was also read back through ADM's UTF-8 file API after a PowerShell console produced misleading mojibake; the source itself remained valid UTF-8 text. `node` was not added to the execution allowlist solely for a parser check.

## Remaining 16-03 work

- 16-03D: converge normal CLI management commands on the same Admin MCP contract; keep explicit local service bootstrap/offline recovery exceptions.
- 16-03E: authenticated/TLS-safe non-loopback remote Admin MCP, which may remain post-first-local-RC if not required.
- Real Wails GUI dogfood is still required before declaring the 1.0 RC.
