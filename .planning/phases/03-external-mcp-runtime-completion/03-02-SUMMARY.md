---
phase: 03-external-mcp-runtime-completion
plan: 03-02
subsystem: management
status: implementation-complete
requirements-completed: [MCP-RUN-01, MCP-RUN-02, MCP-RUN-03, MCP-RUN-04, MCP-RUN-05]
key-files:
  modified:
    - internal/management/service.go
    - internal/desktop/adapter.go
    - cmd/ai-dev-manager/main.go
    - cmd/ai-dev-manager/main_test.go
    - internal/gateway/server.go
    - internal/gateway/mcp_acceptance_test.go
key-decisions:
  - "Human-facing MCP health is a thin pass-through over app.ProbeMCPHealth; no second health model was introduced."
  - "CLI mcp status requires both MCP ID and Environment ID because Environment selection remains the runtime authorization gate."
  - "MCP health state 'disabled' is distinct from operation error_kind 'not_enabled'."
  - "Desktop remains a pass-through only in Phase 3; no MCP health UI expansion was added."
coverage:
  - id: MCP-C1
    description: "A real local external MCP initializes and lists tools through ADM."
    human_judgment: false
    verification:
      - kind: e2e
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
  - id: MCP-C2
    description: "Environment enable/disable gates MCP activation and call access."
    human_judgment: false
    verification:
      - kind: e2e
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
  - id: MCP-C3
    description: "MCP health distinguishes configured, disabled, healthy, and error."
    human_judgment: false
    verification:
      - kind: unit
        ref: "internal/app/mcp_health_test.go"
        status: pass
      - kind: integration
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
  - id: MCP-C4
    description: "Bad or unresolved MCP configuration returns structured, secret-safe status/errors."
    human_judgment: false
    verification:
      - kind: unit
        ref: "internal/app/mcp_health_test.go#TestProbeMCPHealthAuthFailureDoesNotExposeSecrets"
        status: pass
      - kind: e2e
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
  - id: MCP-C5
    description: "A real external MCP tool call succeeds end-to-end through the ADM Gateway."
    human_judgment: false
    verification:
      - kind: e2e
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
---

# Phase 3 Plan 03-02: MCP Health Management Surface Summary

**External MCP runtime health is now available through the existing human management surfaces without creating a second runtime model.**

## Accomplishments

- Added `management.Service.MCPHealth(ctx, environmentID, mcpID)` delegating directly to `app.Service.ProbeMCPHealth`.
- Added `desktop.Adapter.ProbeMCPHealth(environmentID, mcpID)` as a thin management pass-through; no new Desktop UI feature work was introduced.
- Added CLI `mcp status --id MCP_ID --environment-id ENV_ID`, returning the same structured JSON health result used by Gateway and management.
- Added CLI discoverability coverage for `mcp status -h` and an actual status JSON path.
- Tightened Gateway operation errors so an Environment-disabled MCP reports structured `error_kind=not_enabled` while health status remains `disabled`.
- Preserved the Phase 3 runtime contract: Streamable HTTP only, on-demand per-operation sessions, no pooling/background monitor, secrets resolved only at activation boundaries.

## Verification Evidence

- Wave 2 focused tests: `go test ./internal/management/... ./internal/desktop/... ./cmd/ai-dev-manager/...` — pass.
- Full MCP lifecycle: `go test ./internal/gateway -run TestMCPHealthLifecycleEndToEnd -count=1 -v` — pass.
- Full repository: `go test ./...` — pass.
- Static checks: `go vet ./...` — pass.
- Diff hygiene: `git diff --check` — pass; only Windows LF→CRLF notices were emitted.

## Runtime Capability Proven Across Phase 3

- Catalog MCP definitions carry transport and header references while legacy empty transport normalizes to `streamable-http`.
- Environment selection gates health, tool listing, and tool calls.
- Health distinguishes `configured`, `disabled`, `healthy`, and `error` from real on-demand probes.
- Header/env secret references are expanded only when connecting; resolved values are not persisted or exposed in normal health/error output.
- Gateway can list and call a real external MCP tool through ADM.
- Broken external MCPs return structured local errors and do not make unrelated Gateway tools unavailable.
- CLI and Desktop adapter consume the same app-owned health semantics.

## Deferred / Out of Scope

- STDIO, SSE, OpenAPI transports.
- OAuth and richer authentication configuration.
- Persistent sessions/process ownership, pooling, background health monitoring.
- Desktop MCP health/configuration UI expansion.
- Packaging/release changes.

These remain backlog items and do not alter Phase 3 exit criteria.

## Next Gate

Implementation and automated validation are complete. Phase 3 still requires canonical GSD phase verification and project-state transition before being marked complete.

---
*Phase: 03-external-mcp-runtime-completion*
*Plan: 03-02*
