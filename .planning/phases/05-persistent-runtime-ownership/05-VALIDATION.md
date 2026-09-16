---
phase: 05-persistent-runtime-ownership
status: passed
nyquist_compliant: true
wave_0_complete: true
---
# Phase 5 Validation

| ID | Acceptance | Evidence target |
|---|---|---|
| L1 | One running Gateway exposes one stable non-empty runtime owner ID to independent clients | `/healthz`, `gateway status`, `gateway_info`, owner tests |
| L2 | External MCP sessions are owned by the Gateway and reused across separate client sessions | deterministic fake-session connect count + real Streamable HTTP call |
| L3 | Desired state is persisted separately from observed state | same state file across restart; owner ID/session identity changes while Environment selection persists |
| L4 | Restart rebuilds from desired state and never carries stale healthy observation forward | restart acceptance with upstream healthy, stopped, then restored |
| L5 | Disable/remove and clean shutdown close owned sessions deterministically | fake-session close assertions + integration shutdown |
| L6 | Broken/disabled MCP remains operation-local and secrets stay out of status/errors | existing Phase 3 acceptance plus focused owner regressions |
| L7 | No MCP configuration and no Git do not block ordinary Environment/file development | existing core/gateway regression suite |
| L8 | Phase 6 scope is not implemented | diff review: no dev-process/log/port management API |

Required final regression: `go test ./...`, `go vet ./...`, `git diff --check`.

Real acceptance uses a task-owned Gateway/state/loopback port and a local Streamable HTTP MCP fixture. It must prove two client invocations share the same owner, restart changes owner identity, desired selection persists, stale health is not serialized, and cleanup releases owned sessions/resources. Do not disturb the shared 41137 Gateway.
