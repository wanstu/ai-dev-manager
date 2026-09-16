---
phase: 13-environment-capability-diagnostics
plan: "01"
subsystem: environment-capability-diagnostics
tags: [capability, diagnostics, environment, static-report, runtime, mcp, skill]
requires:
  - phase: 12-skill-runtime-completion
    plan: "02"
    provides: Environment-specific Skill availability states and structured Skill read diagnostics
provides:
  - Shared CapabilityReport, CapabilityFact and CapabilityEvidence model
  - Resilient application-level Environment capability report
  - Structured static facts for file, exec, verifier, Git, isolation, MCP, Skill, process and run capabilities
  - Legacy capability string derivation from structured facts for compatibility
  - Optional failure isolation for unresolved MCP/Skill, broken Skill, invalid verifier and invalid roots
implementation_commit: b0eb0c74e6f43844b3a754feaa2d34c93bb82da6
tech-stack:
  added: []
  patterns: [shared model types, application-level static aggregation, validator reuse, side-effect-free diagnostics, optional failure isolation]
key-files:
  created: [internal/model/capability.go, internal/app/capability_report.go, internal/app/capability_report_test.go]
  modified: [internal/app/service.go, internal/gateway/server.go, cmd/ai-dev-manager/main.go, cmd/ai-dev-manager/main_test.go]
key-decisions:
  - "Capability diagnostics are side-effect-free by default: they do not acquire writer leases, execute verifiers, start processes, call MCP tools or refresh MCP sessions."
  - "EnvironmentInspection now carries a structured capability_report while preserving the older capabilities string list by deriving it from available facts."
  - "Writer-gated facts expose requires_writer=true and the currently visible writer lease state without claiming caller ownership."
  - "Static MCP facts validate desired configuration and secret-reference resolution without probing network health; owner-local observation enrichment is left for 13-02."
  - "Skill facts reuse Phase 12 Environment-specific availability states instead of reimplementing Skill artifact checks."
requirements-completed: [CAP-01, CAP-02, ADM-CORE-003, ADM-CORE-004, ADM-GW-003]
completed: 2026-09-09
status: complete
---

# Phase 13 Plan 01: Resilient Capability Fact Model + Static Environment Report Summary

**Environment inspection now returns one structured, resilient capability report.** A caller can see what is available, unavailable, disabled, unconfigured or degraded in one Environment without probing every individual tool and without a broken optional component becoming a global Environment failure.

## Accomplishments

- Added shared application/model-level types:
  - `model.CapabilityReport`
  - `model.CapabilityFact`
  - `model.CapabilityEvidence`
  - stable `CapabilityState` values: `available`, `disabled`, `unconfigured`, `unavailable`, `degraded`.
- Added `Service.EnvironmentCapabilityReport` and static component evaluators for:
  - file tree/read/search;
  - file write/edit/delete;
  - shell exec;
  - verifier definitions;
  - Git status/diff/branch;
  - managed worktree isolation/root validity;
  - selected, unresolved and disabled MCP definitions;
  - selected, unresolved, disabled and broken Skill entries;
  - process/run lifecycle prerequisites.
- Refactored `InspectEnvironment` so it no longer depends on all-or-nothing `Capabilities(ctx, env.ID)` Runtime construction. It now includes `capability_report` and preserves the existing `capabilities` string list by deriving legacy strings from available structured facts.
- Reused real operation boundaries instead of creating permissive diagnostics:
  - Environment/worktree validation still flows through isolation validation.
  - Runtime root validation still uses `runtime.New`.
  - Exec, verifier and stdio MCP command availability uses `runtime.PrepareCommand` without starting a process.
  - Skill availability reuses Phase 12 Environment-specific availability logic.
  - MCP static desired facts reuse typed MCP config validation and secret-reference checks.
- Modeled writer-gated capabilities with `requires_writer=true` and current lease evidence while keeping inspection read-only.
- Kept owner-local runtime observation out of 13-01. Process/run and live MCP health facts are marked as static/degraded or desired-state facts only; 13-02 remains responsible for Gateway-owner observation enrichment.
- Updated the Gateway `environment_inspect` description and CLI help text to mention structured capability facts.
- Fixed the CLI test stdout capture helper to read pipe output concurrently. This became necessary because `environment inspect` now emits a larger JSON report; without concurrent reading, the test helper could deadlock on pipe backpressure.

## Verification Evidence

### Static capability report matrix

- `TestCapabilityReportAggregatesStaticFactsAndKeepsLegacyCapabilities` proves a healthy report includes structured facts for file, exec, verifier, Git, MCP and Skill capabilities, and that legacy capability strings are derived from available facts.
- `TestCapabilityReportPlainDirectoryKeepsFilesButMarksGitUnavailable` proves plain non-Git directories keep file/search/write capability facts while Git facts report unavailable locally.
- `TestCapabilityReportHandlesTamperedRootWithoutErasingGlobalDiagnostics` proves a missing/tampered root makes root-dependent facts unavailable while global HTTP MCP and Skill diagnostics are still returned where their own checks permit.
- `TestCapabilityReportOptionalFailuresArePerResourceAndSanitized` proves unresolved MCP, unresolved Skill, broken Skill, disabled verifier and invalid verifier facts stay local and do not erase unrelated file facts.
- `TestCapabilityReportDoesNotExecuteVerifierOrLeakSecretOrMemorySentinels` plants verifier side-effect, MCP secret and private Memory sentinels and proves capability inspection neither runs the verifier helper nor leaks the sentinel values.
- `TestCapabilityReportWriterLeaseEvidenceIsReadOnly` proves writer evidence is absent/present according to current lease state and capability reporting itself does not acquire a writer lease.

### Timeout investigation and regression fix

During validation, `go test ./...` initially hit the existing 240s async run timeout. Focused `-timeout=90s -v` runs showed the apparent hang was not a capability report logic loop. It was caused by `cmd/ai-dev-manager` test helper `captureStdout`, which wrote the larger `environment inspect` JSON to an `os.Pipe` without a concurrent reader. The helper was fixed to read concurrently, after which:

- `go test -count=1 -timeout=120s -v ./cmd/ai-dev-manager` passed in under one second;
- `go test -count=1 -timeout=240s -v ./internal/gateway -run TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy` passed in about 34.5 seconds;
- full `go test -count=1 ./...` passed.

### Gates passed

- `gofmt -w` over modified Go files.
- `go test -count=1 ./internal/app`.
- `go test -count=1 ./internal/management`.
- `go test -count=1 -timeout=120s -v ./cmd/ai-dev-manager`.
- `go test -count=1 -timeout=240s -v ./internal/gateway -run TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy`.
- `go test -count=1 ./...`.
- `go vet ./...`.
- `go test -race -count=1 ./internal/app -run TestCapability`.
- `git diff --check` passed, with only Windows LF-to-CRLF working-copy warnings.

## Security / Boundary Review

- No Planner/Executor/Reviewer workflow code was reintroduced.
- No `gsd_phase_*` surface or `.planning` state-advance implementation was added.
- Capability inspection does not execute verifiers, commands, MCP tools or Skill instructions.
- Capability inspection does not acquire, renew, release or steal writer leases.
- MCP evidence includes reference keys and configuration facts, not resolved secret values.
- Environment summaries continue to expose private Memory count only, not values.
- Broken optional MCP/Skill/verifier/root-dependent components stay local to their own facts.

## Deferred to 13-02

- Gateway-owner observation enrichment for live MCP/process/run state.
- Canonical owner-aware Agent-facing report shape if it needs additional request context.
- Honest observation provenance/timestamps for owner-local facts after restart.

## Related deferred design

The temporary resource lifecycle/retention topic raised during this implementation is recorded separately in `.planning/rebaseline/2026-09-09-temporary-resource-lifecycle.md`. It remains a future lifecycle/retention design item, not part of 13-01 capability diagnostics.

---

*Phase: 13-environment-capability-diagnostics*
*Plan: 13-01 complete; Phase 13 remains in progress with 13-02 pending*
