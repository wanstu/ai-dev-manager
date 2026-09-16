---
phase: 11-mcp-runtime-completion
plan: "01"
subsystem: mcp-runtime
tags: [mcp, streamable-http, stdio, secrets, process-lifecycle]
requires:
  - phase: 03-external-mcp-runtime-completion
    provides: Environment-gated Streamable HTTP MCP list/call foundation
  - phase: 05-persistent-runtime-ownership
    provides: Gateway-owned persistent MCP sessions
provides:
  - Dedicated persisted MCPDefinition model separated from Skill catalog entries
  - Validated Streamable HTTP and stdio transport configuration
  - Activation-only secret and environment reference resolution
  - Allowlisted Environment-rooted stdio process ownership and cleanup
affects: [11-02-health-reconnect-inventory, 11-03-mcp-import, 15-desktop-core-parity]
actuals:
  tokens: 18360
  tasks: 5
  commits: 1
tech-stack:
  added: []
  patterns: [typed desired configuration, activation-only secret resolution, owner-bound command transport]
key-files:
  created: [internal/catalog/mcp_service.go]
  modified: [internal/model/types.go, internal/app/mcp_health.go, internal/gateway/runtime_owner.go, internal/gateway/server.go, internal/runtime/runtime.go]
key-decisions:
  - "Stdio commands use the existing Runtime allowlist and Environment root rather than a separate process authority path."
  - "The long-lived command context belongs to the Gateway runtime owner while request context bounds protocol connection setup."
patterns-established:
  - "Persist references and desired policy; keep resolved values, sessions and child processes runtime-only."
requirements-completed: [MCP-COMP-01, MCP-COMP-02, MCP-COMP-03, MCP-COMP-06, ADM-CORE-003, ADM-CORE-004, ADM-CORE-007, ADM-GW-003]
coverage:
  - id: D1
    description: "Typed MCP definitions validate transport-specific HTTP/stdio configuration and explicit health policy."
    requirement: MCP-COMP-01
    verification:
      - kind: unit
        ref: "internal/catalog/service_test.go#TestMCPDefinitionsAreTypedAndCallerOwnedCollectionsAreCloned"
        status: pass
      - kind: unit
        ref: "internal/catalog/service_test.go#TestMCPDefinitionValidationIsTransportLocal"
        status: pass
    human_judgment: false
  - id: D2
    description: "Secret-backed headers and stdio environment references resolve only at activation and are not persisted."
    requirement: MCP-COMP-03
    verification:
      - kind: unit
        ref: "internal/app/mcp_health_test.go#TestResolveMCPActivationSeparatesDesiredAndRuntimeSecretValues"
        status: pass
      - kind: integration
        ref: "internal/gateway/mcp_acceptance_test.go#TestGatewayStdioMCPRealTransportAuthorityAndCleanup"
        status: pass
    human_judgment: false
  - id: D3
    description: "Streamable HTTP remains operational and stdio tool list/call uses the existing executable authority."
    requirement: MCP-COMP-02
    verification:
      - kind: integration
        ref: "internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/gateway/mcp_acceptance_test.go#TestGatewayStdioMCPRealTransportAuthorityAndCleanup"
        status: pass
    human_judgment: false
  - id: D4
    description: "Disabling an MCP or closing its Gateway owner terminates the owned stdio child process."
    requirement: ADM-GW-003
    verification:
      - kind: integration
        ref: "internal/gateway/mcp_acceptance_test.go#TestGatewayStdioMCPRealTransportAuthorityAndCleanup"
        status: pass
    human_judgment: false
duration: 55min
completed: 2026-09-08
status: complete
---

# Phase 11 Plan 01: Typed MCP Configuration and Transport Runtime Summary

**Dedicated MCP desired configuration with real Streamable HTTP and allowlisted, Environment-rooted stdio sessions owned by the persistent Gateway runtime**

## Performance

- **Duration:** 55 min
- **Started:** 2026-09-08T12:02:00Z
- **Completed:** 2026-09-08T12:57:25Z
- **Tasks:** 5
- **Files modified:** 20

## Accomplishments

- Split MCP persistence from the generic Skill catalog into a typed transport/auth/health-policy definition.
- Added activation-only expansion of HTTP header and stdio environment references without returning or persisting resolved values.
- Added official SDK command transport support under the existing executable allowlist, Environment root and Gateway owner lifecycle.
- Exposed typed definitions through CLI, management, Desktop adapter and Gateway inputs.
- Added real HTTP and stdio list/call, authority and child-cleanup acceptance coverage.

## Task Commits

1. **Tasks 1-5: typed model, activation, stdio ownership, public inputs and acceptance** — `50e2322` (feat)

## Files Created/Modified

- `internal/catalog/mcp_service.go` — typed MCP CRUD and transport-specific validation.
- `internal/model/types.go` — persisted MCP definition and health policy shapes.
- `internal/app/mcp_health.go` — activation boundary and transient HTTP/stdio probing.
- `internal/gateway/runtime_owner.go` — persistent transport dispatch and lifecycle ordering.
- `internal/runtime/runtime.go` — allowlisted Environment-rooted command construction.
- `cmd/ai-dev-manager/main.go` — typed HTTP/stdio CLI configuration.
- `internal/gateway/mcp_acceptance_test.go` — real command transport authority and cleanup acceptance.

## Decisions Made

- Reused the Runtime allowlist for stdio instead of adding an MCP-specific executable permission model.
- Bound stdio processes to the runtime owner's context so an individual Agent request ending cannot kill a shared session.
- Closed MCP sessions before canceling the owner context, allowing graceful stdio shutdown before process-tree cancellation fallback.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected owner shutdown ordering for command transports**

- **Found during:** Task 5 transport cleanup acceptance
- **Issue:** canceling the owner context before closing the SDK session made graceful stdio close return `exec: canceling Cmd: invalid argument` on Windows.
- **Fix:** close owned MCP sessions first, then cancel the shared owner context before closing other resources.
- **Files modified:** `internal/gateway/runtime_owner.go`
- **Verification:** stdio disable/owner-close acceptance and full Gateway suite pass.
- **Committed in:** `50e2322`

**Total deviations:** 1 auto-fixed bug. **Impact:** required for deterministic Windows child-process cleanup; no scope expansion.

## Issues Encountered

- The stdio acceptance initially compared a nested JSON string against an unescaped Windows path; the assertion now compares the encoded path while still verifying the real child working directory.

## User Setup Required

None - stdio MCP users configure their own executable allowlist and environment references through normal ADM management surfaces.

## Next Phase Readiness

Plan 11-02 can build protocol Ping health monitoring, inventory refresh and fixed-interval optional reconnect on the typed definitions and owner-bound sessions delivered here.

---

*Phase: 11-mcp-runtime-completion*
*Completed: 2026-09-08*
