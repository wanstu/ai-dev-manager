---
phase: 03-external-mcp-runtime-completion
plan: 01
subsystem: external-mcp-runtime
tags: [mcp, streamable-http, health, gateway, secrets, environment-policy]
requires:
  - 02-structured-verifier-runtime
provides:
  - Streamable HTTP MCP connection definitions with transport and header references
  - Environment-gated on-demand MCP health probing with four explicit states
  - external MCP tool discovery and invocation through the ADM Gateway
  - activation-boundary environment-variable resolution without persisted resolved secrets
  - structured app-owned MCP errors without endpoint or secret leakage
affects: [management, desktop, cli, external-agent-dogfood]
tech-stack:
  added: []
  patterns:
    - MCP activation is Environment-gated rather than implied by catalog existence
    - health is an on-demand probe, not a background monitor or persistent session
    - each probe/list/call creates an independent Streamable HTTP client session
    - resolved endpoint/header values exist only at the connection boundary
    - app.MCPError is the canonical structured MCP runtime error type
key-files:
  created:
    - internal/app/mcp_health.go
    - internal/app/mcp_health_test.go
    - internal/gateway/mcp_acceptance_test.go
  modified:
    - internal/model/types.go
    - internal/catalog/service.go
    - internal/catalog/service_test.go
    - internal/gateway/server.go
key-decisions:
  - "Phase 3 supports Streamable HTTP only; stdio and other transports remain deferred."
  - "Catalog entries store HeaderRefs and unresolved endpoint/header strings; os.ExpandEnv is used only at activation boundaries."
  - "Health has exactly configured, disabled, healthy, and error states and is probed per MCP ID."
  - "Gateway list/call operations remain Environment-gated and use app.MCPError rather than raw endpoint-bearing errors."
  - "No connection pooling, persistent MCP sessions, or background health monitor was added."
requirements-completed:
  - MCP-RUN-01
  - MCP-RUN-02
  - MCP-RUN-03
  - MCP-RUN-04
  - MCP-RUN-05
status: implementation-complete
---

# Phase 3 Plan 03-01: External MCP Runtime Tracer Summary

**The external MCP catalog is now a real runtime capability rather than configuration-only metadata: an Environment can enable one Streamable HTTP MCP, probe its actual health, list its remote tools, and call a remote tool through ADM without bypassing Environment policy or exposing resolved secrets.**

## Accomplishments

- Enriched MCP catalog entries with `Transport` and `HeaderRefs`; empty transport remains backward-compatible by normalizing to `streamable-http`.
- Added `AddMCPConfig` while preserving the existing `AddMCP` convenience path. Unsupported transports such as stdio are rejected in this phase rather than silently accepted.
- Added app-owned `MCPHealthState`, `MCPHealthStatus`, and canonical `MCPError`.
- Added `ProbeMCPHealth` with the four locked states: `configured`, `disabled`, `healthy`, and `error`.
- Health probing resolves endpoint/header environment references only when connecting, injects resolved headers through the MCP Streamable HTTP transport, and never persists or returns resolved secret values.
- Added Gateway `environment_mcp_status`; enriched `environment_mcp_tools` and `environment_mcp_call` so they use Environment selection, health, structured errors, secret-safe connection handling, and per-operation sessions.
- Added structured `missing_tool_name` handling for empty external tool names.
- Added a full real Streamable HTTP acceptance lifecycle: enable → healthy → list tools → call tool → disable → disabled → break upstream → error, while proving unrelated Gateway tools still work after the MCP fails.

## Verification Evidence

- Catalog focused tests: `go test ./internal/catalog/...` — pass.
- App health tests: configured, disabled, healthy, connection refused, timeout, unresolved secret refs, header/env expansion, auth failure, and secret non-exposure — pass.
- Phase quick sampling: `go test ./internal/catalog/... ./internal/app/... ./internal/gateway/...` — pass.
- Gateway lifecycle acceptance: `go test ./internal/gateway -run TestMCPHealthLifecycleEndToEnd -count=1 -v` — pass.
- Gateway package: `go test ./internal/gateway/...` — pass.
- Full suite: `go test ./...` — pass.
- Static checks: `go vet ./...` — pass.
- Diff hygiene: `git diff --check` — pass; Windows LF→CRLF notice only.

## Implementation Commits

- `71254d8` — `feat(mcp): enrich catalog connection definitions`
- `bad8bdd` — `feat(mcp): add on-demand health probe`
- `c1d013b` — `feat(mcp): complete gateway external runtime`

## Correctness Fixes / Deviations

### Worktree Git marker acceptance fix

Phase 3 quick sampling exposed a Phase 2 verifier acceptance issue where a Git worktree's `.git` marker file was copied into the supposed non-Git acceptance copy. The test-only copy filter was minimally corrected before continuing Phase 3. Verifier runtime semantics were unchanged.

### Timeout test fixture

The first timeout fixture waited forever on the server request context. The MCP SDK sends a bounded cancellation notification when the caller context expires, so the deliberately non-returning handler made test cleanup pathological. The fixture was changed to a finite delayed response while retaining a shorter caller deadline; runtime timeout semantics were unchanged.

## Scope Control

- No stdio transport.
- No legacy SSE transport.
- No OpenAPI adapter.
- No OAuth implementation.
- No connection/session pooling.
- No MCP child-process ownership.
- No background health monitor.
- No Desktop feature expansion.
- No package/release work.

## Deferred Feedback Recorded During Execution

- Future MCP configuration should cover validated server names, optional descriptions, stdio/SSE/Streamable HTTP/OpenAPI transport-specific settings, none/header-token/OAuth authentication, and explicit env/header configuration. MCPHub is a useful design reference for separating these concerns, but this does not alter the locked Phase 3 Streamable HTTP tracer.
- Desktop ordinary `go build ./cmd/ai-dev-manager-desktop` can create a binary that later reports missing Wails build tags; improving that build UX remains deferred because Desktop/package work is outside Phase 3.

## Next Gate

Plan 03-02 adds only thin human-management access to the already-proven health probe: management delegate, Desktop adapter passthrough, and CLI `mcp status`. Then Phase 3 runs its final validation and canonical GSD verification/transition.

---
*Phase: 03-external-mcp-runtime-completion*
*Plan: 03-01*
