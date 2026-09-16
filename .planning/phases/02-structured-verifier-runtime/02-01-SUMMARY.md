---
phase: 02-structured-verifier-runtime
plan: 01
subsystem: runtime
tags: [verifier, runtime, mcp-gateway, non-git, nyquist]
requires:
  - 01-skill-runtime
provides:
  - Environment-scoped structured verifier definitions
  - structured pass/fail/timeout verifier results over the ADM Gateway
  - verifier execution through existing Runtime.Exec allowlist/cwd/output policy
  - real Streamable HTTP non-Git go test ./... acceptance
affects: [external-mcp-runtime, external-agent-dogfood, agent-runs, gsd-runtime]
tech-stack:
  added: []
  patterns:
    - verifier definitions are Environment-scoped facts while executable authority remains global
    - verifier execution reuses Runtime.Exec and writer heartbeat semantics
    - timeout identity is derived from an app-owned verifier deadline context
    - recursive real-suite acceptance is isolated in one excluded acceptance test file
key-files:
  created:
    - internal/verifier/service.go
    - internal/verifier/service_test.go
    - internal/app/verifier_internal_test.go
    - internal/gateway/verifier_acceptance_test.go
  modified:
    - internal/model/types.go
    - internal/app/service.go
    - internal/app/service_test.go
    - internal/gateway/server.go
    - internal/gateway/server_test.go
    - cmd/ai-dev-manager/main.go
    - cmd/ai-dev-manager/main_test.go
key-decisions:
  - "Verifier definitions persist inline on Environment; no global verifier catalog was added."
  - "Defining a verifier never grants executable authority; RunVerifier still passes through Runtime.Exec allowlist and cwd containment."
  - "environment_verifier_list is writer-free; environment_verifier_run requires the matching Environment writer."
  - "V-R1/V-R2/V-R3 live only in internal/gateway/verifier_acceptance_test.go, and the non-Git copy excludes/asserts absence of that file before inner go test ./...."
requirements-completed:
  - VERIFY-01
  - VERIFY-02
  - VERIFY-03
  - VERIFY-04
coverage:
  - id: V-U1
    description: "exit 0 returns passed with exit code 0 and timed_out false"
    verification:
      - kind: unit
        ref: "internal/verifier/service_test.go#TestClassifyRuntimeOutcomes"
        status: pass
  - id: V-U2
    description: "nonzero exit returns failed with the real exit code"
    verification:
      - kind: unit
        ref: "internal/verifier/service_test.go#TestClassifyRuntimeOutcomes"
        status: pass
  - id: V-U3
    description: "deadline returns failed with timed_out true and measured duration"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierTracerPassFailTimeoutAndPolicyError"
        status: pass
  - id: V-U4
    description: "chatty verifier output stays bounded inside the structured result"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierBoundsOutputAndPreservesStructuredPass"
        status: pass
  - id: V-U5
    description: "start/policy failure remains a local error distinct from a failed verifier result"
    verification:
      - kind: unit
        ref: "internal/verifier/service_test.go#TestClassifyRuntimeOutcomes"
        status: pass
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierTracerPassFailTimeoutAndPolicyError"
        status: pass
  - id: V-U6
    description: "missing or disabled verifier IDs fail locally"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierRejectsMissingAndDisabledDefinitionsLocally"
        status: pass
  - id: V-I1
    description: "verifier definitions persist per Environment and zero-config reloads empty"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestVerifierDefinitionsPersistPerEnvironmentAndZeroConfigReloadsCleanly"
        status: pass
  - id: V-I2
    description: "environment_verifier_list is Environment-scoped and writer-free"
    verification:
      - kind: integration
        ref: "internal/gateway/server_test.go#TestGatewayVerifierListAndRunAreEnvironmentScopedAndWriterGated"
        status: pass
  - id: V-I3
    description: "environment_verifier_run requires the matching writer and verifier execution heartbeats the lease"
    verification:
      - kind: integration
        ref: "internal/gateway/server_test.go#TestGatewayVerifierListAndRunAreEnvironmentScopedAndWriterGated"
        status: pass
      - kind: integration
        ref: "internal/app/verifier_internal_test.go#TestRunVerifierHeartbeatsWriterDuringLongRun"
        status: pass
  - id: V-I4
    description: "verifier cwd remains Runtime-contained including parent, absolute, and symlink escapes"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierUsesRuntimeCwdContainment"
        status: pass
  - id: V-I5
    description: "a stored verifier does not grant executable allowlist authority"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestRunVerifierTracerPassFailTimeoutAndPolicyError"
        status: pass
  - id: V-I6
    description: "Gateway exposes list/run and broken verifier failures do not break unrelated tools"
    verification:
      - kind: integration
        ref: "internal/gateway/server_test.go#TestGatewayVerifierListAndRunAreEnvironmentScopedAndWriterGated"
        status: pass
  - id: V-I7
    description: "zero verifier configuration does not block normal file/runtime development"
    verification:
      - kind: integration
        ref: "internal/app/service_test.go#TestZeroVerifierConfigurationDoesNotBlockNormalDevelopment"
        status: pass
  - id: V-R1
    description: "a configured verifier runs on a real non-Git Environment over Streamable HTTP"
    verification:
      - kind: e2e
        ref: "internal/gateway/verifier_acceptance_test.go#TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy"
        status: pass
  - id: V-R2
    description: "V2 go test ./... runs through environment_verifier_run on the non-Git repository copy"
    verification:
      - kind: e2e
        ref: "internal/gateway/verifier_acceptance_test.go#TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy"
        status: pass
  - id: V-R3
    description: "real HTTP failing and timeout verifiers return identity plus structured bounded failure"
    verification:
      - kind: e2e
        ref: "internal/gateway/verifier_acceptance_test.go#TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy"
        status: pass
status: implementation-complete
---

# Phase 2 Plan 02-01: Structured Verifier Runtime Summary

**Structured verification is now a real Environment-scoped Agent capability: definitions persist with the Environment, execution reuses the existing Runtime policy boundary, and real HTTP acceptance proves non-Git `go test ./...`, fail, and timeout behavior.**

## Accomplishments

- Added `VerifierDefinition` to `Environment` and a small `internal/verifier` service for scoped definition management, stable `vf_` IDs, validation, timeout policy, and structured result classification.
- Added App `RunVerifier` with matching-writer enforcement, shared writer-heartbeat semantics, app-owned timeout identity, and execution through existing `Runtime.Exec` allowlist/cwd/output controls.
- Added minimal human declaration CLI: `environment verifier add|list|remove`; no Desktop verifier feature work and no CLI run path.
- Added only the locked Agent Gateway surface: `environment_verifier_list` and `environment_verifier_run`.
- Proved all Nyquist V-U1..V-U6 and V-I1..V-I7 negative/policy cases, including zero-config development, allowlist non-escalation, cwd/symlink containment, bounded output, missing/disabled IDs, and writer heartbeat.
- Proved V-R1/V-R2/V-R3 over a real Streamable HTTP MCP client on a non-Git repository copy. The copy excludes `verifier_acceptance_test.go` and asserts its absence before the inner `go test ./...`, preventing self-recursive acceptance execution.

## Verification Evidence

- Focused verifier/App/Gateway/CLI tests: pass.
- Real acceptance: `go test ./internal/gateway -run ^TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy$ -count=1 -v` — pass.
- Full suite: `go test ./...` — pass, including the mandatory real verifier acceptance.
- Static checks: `go vet ./...` — pass.
- Diff hygiene: `git diff --check` — pass (Windows LF→CRLF notices only; no whitespace errors).

## Deviations / Correctness Fixes

### 1. V-R2 self-recursion blocker fixed before implementation

The first plan-check failed because the non-Git copy would have retained the acceptance test that itself invokes `go test ./...`. The corrected plan isolates V-R1/V-R2/V-R3 in `internal/gateway/verifier_acceptance_test.go`, excludes that file from the copy, and asserts it is absent before the inner test suite. A real `gsd-plan-checker` rerun returned `PLAN CHECK PASS` before Task 1 began.

### 2. Nested Windows test startup required a less brittle test-only timeout

The first real V-R2 run terminated normally but exposed a test-only timing issue: the writer-heartbeat helper verifier had a 5-second timeout and nested Windows `go test` process startup took about 5.5 seconds. The helper verifier timeout was widened to 30 seconds; production verifier timeout semantics were unchanged. The next real V-R2/V-R3 run passed.

## Scope Control

- No new execution primitive.
- No Runtime/CommandResult redesign.
- No global verifier catalog or persisted verifier results.
- No Git/worktree prerequisite.
- No Desktop verifier work.
- No process lifecycle, Agent Run, GSD executor, external MCP expansion, or distribution work.

## Next Gate

Implementation evidence is complete. Phase 2 still requires canonical GSD verification/UAT and planning-state transition before it can be marked complete or merged to `master`.

---
*Phase: 02-structured-verifier-runtime*
*Plan: 02-01*
