---
status: complete
phase: 03-external-mcp-runtime-completion
source:
  - 03-02-SUMMARY.md
started: 2026-09-07T15:04:00+08:00
updated: 2026-09-07T15:04:00+08:00
coverage_mode: automated
---

## Current Test

[testing complete]

## Tests

### 1. Real external MCP initializes and lists tools through ADM
expected: An Environment-enabled real local Streamable HTTP MCP can initialize, report healthy, and expose its remote tool list through the ADM Gateway.
result: pass
source: automated
verification:
  - `internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd`

### 2. Environment gating controls MCP activation and access
expected: Disabled MCPs do not activate; enabled MCPs can list/call; disabling again blocks access with structured `not_enabled` errors.
result: pass
source: automated
verification:
  - `internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd`

### 3. Health reports configured / disabled / healthy / error correctly
expected: Configuration existence is not health; unresolved/empty configuration stays configured, disabled selection reports disabled, a working remote reports healthy, and failed connections report error.
result: pass
source: automated
verification:
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthConfigured`
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthDisabled`
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthHealthyAndHeaderExpansion`
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthConnectionRefused`
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthTimeout`

### 4. Bad configuration and authentication errors remain structured and secret-safe
expected: Unresolved secret refs are not mistaken for health, auth/connection errors use stable error kinds, and resolved endpoint/header secret values do not appear in normal status/error output.
result: pass
source: automated
verification:
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthUnresolvedSecretRefStaysConfigured`
  - `internal/app/mcp_health_test.go#TestProbeMCPHealthAuthFailureDoesNotExposeSecrets`
  - `internal/app/mcp_health_test.go#TestMCPErrorImplementsErrorWithoutSecretLeakage`
  - `internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd`

### 5. A real external MCP tool call succeeds end-to-end
expected: ADM calls a real remote MCP tool through the Environment-gated Gateway and returns the remote result; a later upstream failure does not break unrelated Gateway tools.
result: pass
source: automated
verification:
  - `internal/gateway/mcp_acceptance_test.go#TestMCPHealthLifecycleEndToEnd`

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

Local GSD `uat classify-coverage --summary 03-02-SUMMARY.md` returned `mode=coverage`, `all_auto_covered=true`, five `auto_passed` deliverables, zero `present` items, and zero classifier errors. No result in this file is represented as a manual user observation.

## Gaps

None.
