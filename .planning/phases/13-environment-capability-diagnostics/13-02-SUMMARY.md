---
phase: 13-environment-capability-diagnostics
plan: "02"
subsystem: capability-diagnostics
tags: [capability, diagnostics, gateway, runtime-owner, mcp, process, run]
requires:
  - phase: 13-environment-capability-diagnostics
    plan: "01"
    provides: Shared CapabilityReport/CapabilityFact model and resilient side-effect-free application-level Environment report
provides:
  - Gateway `environment_capability_report` surface
  - Runtime-owner enrichment for MCP health/tool-inventory observations
  - Runtime-owner enrichment for process and generic run lifecycle availability
  - CLI and management capability-report wrappers over the static application report
  - Same shared CapabilityFact schema for app-static and owner-enriched diagnostics
implementation_commit: 46c86f3
tech-stack:
  added: []
  patterns: [side-effect-free diagnostics, owner-local observation enrichment, schema reuse, no implicit probe/reconnect]
key-files:
  created: [internal/gateway/capability_report.go, internal/gateway/capability_report_test.go, internal/management/capability_report_test.go]
  modified: [cmd/ai-dev-manager/main.go, cmd/ai-dev-manager/main_test.go, internal/gateway/server.go, internal/management/service.go]
key-decisions:
  - "Gateway-owner capability enrichment reuses the Phase 13-01 CapabilityReport schema instead of introducing a second diagnostics model."
  - "Capability report reads existing owner-local observations only; it does not reconnect, Ping, refresh inventory, call MCP tools, run verifiers, start processes, or acquire writer leases."
  - "Enabled MCPs with no current owner observation are reported as degraded/not_observed rather than guessed healthy or forced through an implicit probe."
  - "CLI and management surfaces expose the application-level static report; Gateway adds owner-local runtime evidence when a runtime owner is present."
requirements-completed: [CAP-02]
completed: 2026-09-09
status: complete
---

# Phase 13 Plan 02: Runtime Observation Enrichment + Agent-facing Capability Report Summary

**Environment capability diagnostics now have one canonical Agent-facing report.** Application code provides the resilient static facts from 13-01, while a live Gateway runtime owner can enrich the same facts with owner-local MCP/process/run observations without mutating runtime state.

## Accomplishments

- Added Gateway-owner `CapabilityReport` enrichment over the existing application report.
- Added Gateway tool `environment_capability_report`.
- Added CLI command `environment capability-report --environment-id ENV_ID` plus alias `environment capabilities`.
- Added management wrapper `EnvironmentCapabilityReport` for human/UI callers.
- Added owner-local evidence for:
  - MCP runtime observations;
  - Gateway owner identity and counts;
  - dev-process lifecycle availability;
  - generic async run lifecycle availability.
- Preserved 13-01 static app report as the base source of truth for file, exec, verifier, Git, isolation, MCP desired config, Skill availability, process and run capability facts.
- Kept `environment inspect` compatible while exposing the canonical `capability_report` embedded in the inspection output.

## Runtime-owner enrichment behavior

For enabled MCPs that are statically available:

- existing healthy owner observation keeps the fact `available` and adds observation evidence;
- existing error owner observation marks the fact `unavailable` with the owner-observed structured reason;
- existing disabled owner observation marks the fact `disabled`;
- no owner observation marks the fact `degraded` with `not_observed`.

For process/run lifecycle facts:

- when static runtime/exec prerequisites are satisfiable and a Gateway owner is available, the lifecycle facts become `available` with owner evidence;
- if static prerequisites are unavailable or unconfigured, the static reason remains authoritative.

## Side-effect boundary

The report is intentionally read-only by default:

- no MCP reconnect;
- no protocol Ping;
- no MCP inventory refresh;
- no MCP tool call;
- no verifier execution;
- no process/run start;
- no writer lease acquisition.

The report can therefore be safely requested often by Agents or UI without changing Environment state.

## Verification Evidence

### Owner observation enrichment

- `TestGatewayCapabilityReportEnrichesOwnerObservationWithoutSideEffects` proves a Gateway owner can enrich healthy/error MCP facts and process/run lifecycle facts while a fake connector records zero connection attempts.
- The same test proves the Agent-facing `environment_capability_report` tool exposes owner evidence and structured MCP runtime failure facts.

### Restart/staleness behavior

- `TestGatewayCapabilityReportAfterOwnerRestartDoesNotReuseStaleObservation` proves a new runtime owner does not inherit stale in-memory MCP observations; the report honestly returns `degraded/not_observed` without reconnecting.

### CLI and management surfaces

- `TestEnvironmentListAndInspectShowManagementContextWithoutMemoryValues` proves CLI `environment capability-report` returns the static report and does not leak private Memory values.
- `TestEnvironmentHelpExplainsRenameSafety` proves the new command is discoverable in CLI help.
- `TestManagementEnvironmentCapabilityReportDelegatesToApplicationSchema` proves management delegates to the application report schema.

### Regression gates

Validation passed on Windows:

- `go test -count=1 ./internal/gateway -run TestGatewayCapabilityReport`
- `go test -count=1 ./cmd/ai-dev-manager ./internal/management ./internal/gateway`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check` with only Windows LF→CRLF warnings
- `go test -race -count=1 ./internal/gateway -run TestGatewayCapabilityReport`

## Security / Boundary Review

- No Planner/Executor/Reviewer, GSD phase API, `.planning` state advancement or task orchestration semantics were added.
- The report never executes Skill instructions and never interprets Skill prose.
- The report does not expose resolved MCP secrets, private Memory values, arbitrary process IDs or unbounded logs.
- Runtime observations remain owner-local and are not persisted as durable state.
- Optional capability failures remain per-capability facts and never poison the whole Environment.

## Phase 13 Result

With 13-01 and 13-02 complete, ADM now has a canonical Environment capability diagnostics model that can answer what is usable, what is unavailable, why, and which ADM evidence supports the result. Static application facts and live Gateway-owner observations share one schema, while the Agent/GSD layer remains responsible for planning/orchestration decisions.

## Next Phase Readiness

Phase 14 can consider evidence-first investigation helpers only after review under the Core boundary. Temporary resource lifecycle/retention remains a separate future design item captured under `.planning/rebaseline/2026-09-09-temporary-resource-lifecycle.md`.

---

*Phase: 13-environment-capability-diagnostics*
*Plan: 13-02 complete; Phase 13 complete; Phase 14 pending*
