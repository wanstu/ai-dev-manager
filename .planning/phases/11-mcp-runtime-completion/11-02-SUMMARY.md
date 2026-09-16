---
phase: 11-mcp-runtime-completion
plan: "02"
subsystem: mcp-runtime
tags: [mcp, health, reconnect, inventory, diagnostics, runtime-owner]
requires:
  - phase: 11-mcp-runtime-completion
    plan: "01"
    provides: Typed MCP desired configuration and real HTTP/stdio owner-bound transport runtime
provides:
  - Owner-local MCP runtime observation with bounded public tool inventory
  - Configurable periodic health probing and fixed-interval optional automatic reconnect
  - Explicit environment_mcp_inspect and environment_mcp_refresh operations
  - Stable-ID MCP update with immediate runtime invalidation
  - Structured failure stages and safe no-replay tool-call semantics
  - Restart proof that desired policy persists while owner-local observation is rebuilt
affects: [11-03-mcp-import, 13-environment-capability-diagnostics, 15-desktop-core-parity]
implementation_commit: 4c4f4dca726228409130e79949c3fa9c11e0c1ed
tech-stack:
  added: []
  patterns: [owner-local observation, generation invalidation, single in-flight recovery, fixed-interval reconnect]
key-files:
  created: [internal/gateway/runtime_mcp_observation.go]
  modified: [internal/app/mcp_health.go, internal/catalog/mcp_service.go, internal/gateway/runtime_owner.go, internal/gateway/server.go, internal/gateway/mcp_acceptance_test.go, internal/gateway/runtime_owner_test.go]
key-decisions:
  - "Runtime health, timestamps, failure counters and tool inventory remain owner-local and are never persisted as desired state."
  - "MCP update preserves stable identity and invalidates existing owner session/observation so changed policy/config takes effect without Gateway restart."
  - "Health recovery may reconnect safely but never replays a failed arbitrary tool call."
  - "At most one probe/reconnect work item may be in flight per Environment/MCP generation."
requirements-completed: [MCP-COMP-04, MCP-COMP-05, MCP-COMP-06, LIFE-01, LIFE-02, LIFE-03, ADM-GW-003]
completed: 2026-09-08
status: complete
---

# Phase 11 Plan 02: MCP Health Monitor, Recovery, Inventory + Diagnostics Summary

**Persistent Gateway-owned MCP sessions now have an inspectable, self-recovering and revocation-safe runtime lifecycle without persisting transient state or replaying failed tool calls.**

## Accomplishments

- Added `MCPRuntimeObservation` keyed by Environment/MCP with desired-enabled state, transport, health state, failure stage, sanitized error kind/message, health timestamps, consecutive failure count, next reconnect time, in-flight state and bounded public tool inventory.
- Added a Gateway-owned monitor that schedules health probes from each MCP's persisted `health_policy`, bounds probes by configured timeout and uses fixed configured reconnect intervals when `auto_reconnect=true`.
- Added generation/in-flight guards so disable/remove/update invalidation prevents stale asynchronous recovery from resurrecting old runtime state and only one recovery operation can run per Environment/MCP generation.
- Added `environment_mcp_inspect` for sanitized desired config + current owner observation and `environment_mcp_refresh` for explicit session discard, reconnect/probe/discovery and inventory refresh.
- Added stable-ID `mcp_update`; successful update preserves existing Environment ID selections while immediately dropping owner-local runtime state so new transport/auth/health policy is used without restarting Gateway.
- Tool inventory is bounded to 256 public metadata entries; input schemas remain available through the real tool-list surface rather than being copied into observation state.
- Tool list/call failures invalidate stale sessions and return structured failure evidence. Failed arbitrary tool calls are never automatically replayed before or after reconnect.
- Kept protocol-version compatibility: legacy/stdio sessions use protocol Ping; negotiated MCP 2026-07-28+ sessions, where Ping was removed, use a safe discovery request for liveness.

## Verification Evidence

### Runtime policy behavior

- `TestRuntimeOwnerPolicyUpdateTakesEffectWithoutRestart` proves a 3-second health interval does not probe early, updating policy to 1 second takes effect without Gateway restart, old session state is invalidated, and `health_check_enabled=false` stops background probes.
- `TestRuntimeOwnerAutoReconnectFalseRequiresExplicitRecovery` proves omitted/false automatic reconnect does not recover in the background and explicit refresh can recover later.
- `TestRuntimeOwnerProbeTimeoutBoundsBackgroundPing` proves `probe_timeout_seconds` bounds a background health probe and records a structured Ping timeout.
- `TestRuntimeOwnerAllowsOnlyOneRecoveryInFlight` proves concurrent recovery attempts collapse to one in-flight connection attempt.
- Existing periodic failure/recovery coverage proves unhealthy Ping state evicts stale healthy sessions, fixed-interval automatic reconnect restores health, and disable invalidation prevents resurrection.

### Inventory, refresh and safe calls

- `TestRuntimeOwnerInspectAndRefreshTools` proves explicit refresh closes the stale session and replaces real inventory (`owned_first` -> `owned_second`) rather than returning cached inventory.
- `TestRuntimeOwnerToolFailureIsNeverReplayed` proves one failed tool call produces exactly one invocation and leaves structured call failure evidence.
- `TestRuntimeOwnerUsesPingForLivenessAndKeepsInventory` proves liveness probing is distinct from inventory discovery and owner observation retains bounded public inventory.

### Restart semantics

- `TestRuntimeOwnerRealHTTPRestartReconciliation` now persists a non-default health policy, starts/stops real Gateway processes, and proves:
  - stable desired MCP selection and health policy survive restart;
  - runtime owner identity changes;
  - inventory/health timestamps are freshly regenerated by the new owner rather than reused;
  - when the upstream is unavailable after a later restart, stale prior inventory/healthy observation is not resurrected.

### Regression gates

All closeout gates passed on Windows:

- `go test -count=1 ./internal/catalog ./internal/gateway`
- focused real restart + inventory-refresh acceptance
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`
- `go test -race -count=1 ./internal/gateway`

The full non-cached suite passed across CLI, Desktop adapter, app, catalog, environment, gateway, isolation, management, runtime, skill and verifier packages. Gateway race coverage completed without data-race reports.

## Deviations / Additional Work

### 1. Stable-ID MCP update became necessary for the plan's runtime-policy acceptance

The validation contract requires health interval/config changes to take effect without restarting Gateway and requires pending work to be invalidated when an MCP definition changes. The existing catalog only supported add/remove/default mutation, so the plan could not honestly satisfy that acceptance boundary.

`MCPService.UpdateMCPConfig` and Gateway `mcp_update` were added as a narrow Core capability. Update reuses the canonical MCP validator, preserves the MCP ID, rejects invalid updates atomically and invalidates owner runtime state on success. This also supplies the identity-preserving primitive required by Plan 11-03 `update_by_name` import behavior without implementing importer policy early.

### 2. Protocol Ping compatibility follows negotiated MCP version

The repository's MCP SDK supports protocol versions where Ping exists, while MCP 2026-07-28 removed Ping. Runtime probing therefore uses Ping where negotiated and a safe `tools/list` discovery request for newer sessionless HTTP sessions. This keeps real current-protocol acceptance working without adding legacy SSE behavior.

## Security / Boundary Review

- Observation structures contain reference keys and public tool metadata, not resolved header/env secret values.
- Runtime health/recovery state is not added to persisted `model.State`.
- Stdio continues to use the existing executable allowlist and Environment-rooted command authority from 11-01.
- Reconnect owns only connection/session recovery; it never invokes an arbitrary MCP tool.
- No Planner/GSD orchestration semantics were introduced.
- MCP failure remains capability-local; the complete regression suite keeps unrelated files/Skill/verifier/runtime behavior green.

## Next Phase Readiness

Plan 11-03 can now build import preview/apply on top of:

- typed canonical MCP definitions from 11-01;
- stable-ID validated update from 11-02;
- immediate owner-state invalidation after updates;
- structured inspect/diagnostic runtime facts.

Plan 11-03 remains unimplemented at this closeout. No importer format parsing, batch apply or conflict policy was started here.

---

*Phase: 11-mcp-runtime-completion*
*Plan: 11-02 complete; 11-03 next*
