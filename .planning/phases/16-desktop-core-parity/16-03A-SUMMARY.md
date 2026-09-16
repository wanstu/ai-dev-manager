# 16-03A Summary — Agent/Admin MCP Surface Split

## Outcome

ADM now exposes two MCP surfaces over the same Core service/runtime owner:

- `/mcp` — Agent development surface.
- `/admin/mcp` — privileged Admin/management surface.

The split is implemented by scoped tool registration rather than duplicating Core services or runtime state.

## Agent surface

The Agent surface retains Environment-scoped development capabilities such as:

- Workspace/Environment inspection and capability diagnostics;
- writer lease lifecycle;
- verifier/process/run lifecycle;
- bounded files/search/read/write/edit/delete;
- allowlisted exec execution and allowlist inspection;
- Environment-selected external MCP inspect/refresh/tools/call/status;
- Environment Skill availability/files/read;
- optional Git/worktree development operations;
- Environment-private Memory operations.

## Admin-only surface

The following host-management mutations are filtered out of ordinary Agent MCP and remain available through Admin MCP:

- Workspace add/rename/remove;
- Environment create/rename/remove;
- executable allowlist add/remove;
- global MCP list/add/update/remove/default/import preview/apply;
- Environment MCP selection mutation;
- global Skill list/add/remove/default and Skill source add/list/refresh/remove;
- Environment Skill selection mutation;
- global Memory write/delete.

Admin MCP is a superset and can also use shared/Environment-scoped diagnostic/development tools when needed by Desktop/CLI management flows.

## Protocol identity

`gateway_info` now reports the surface explicitly:

- `surface: agent` with the development-gateway role;
- `surface: admin` with the management-gateway role.

The Admin MCP implementation name is distinct (`ai-dev-manager-v2-admin`) while `/healthz` continues to identify the overall ADM HTTP service.

## Compatibility and runtime ownership

- Existing `New(service)` and stdio Gateway semantics remain Agent-facing.
- `NewAdmin(service)` is available for an Admin MCP server in tests/in-process clients.
- HTTP Gateway mounts both `/mcp` and `/admin/mcp`.
- Both HTTP surfaces share the same `runtimeOwner`; MCP session/health/process/run observations are not duplicated.
- Current loopback/Host restrictions remain unchanged. This slice does not enable remote Admin MCP.

## Tests migrated

Acceptance tests that intentionally perform host-management operations now use the Admin surface. Agent acceptance tests remain on the Agent surface.

New/updated assertions prove:

- Admin-only tools are absent from Agent inventory;
- Admin MCP contains required management operations;
- Agent development tools remain available;
- `/mcp` and `/admin/mcp` are separate Streamable HTTP MCP paths;
- restart persistence still works when Admin creates state and Agent consumes it afterward;
- worktree Agent operations remain available while generic Environment removal uses Admin MCP.

## Validation

Passed locally on `master`:

- `go test -count=1 ./internal/gateway`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`

`git diff --check` reported only Windows LF→CRLF warnings.

Implementation commit:

- `2c6ab1c` — `feat(16-03a): split Agent and Admin MCP surfaces`

## Remaining 16-03 work

- 16-03B: configurable Desktop connection profile + health/liveness against a chosen local ADM endpoint.
- 16-03C: Desktop normal management through Admin MCP instead of direct writable state-file access.
- 16-03D: converge ordinary CLI management on the same Admin MCP contract.
- 16-03E: remote Admin MCP only after authentication/TLS/authorization design.

No remote exposure, authentication scheme, or direct-file fallback behavior was added in 16-03A.
