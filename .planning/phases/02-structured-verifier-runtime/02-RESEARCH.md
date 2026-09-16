# Phase 2 Research — Structured Verifier Runtime

## Summary

Phase 2 makes "change → verify" a first-class, structured Agent capability by wrapping the already-policy-enforced `Runtime.Exec` in a small verifier layer: persisted, Environment-scoped verifier *definitions* + a structured *VerifierResult*, exposed to Agents through the ADM Gateway as list/run tools. All four VERIFY requirements are satisfiable without introducing any new execution primitive — verification reuses the existing allowlisted-executable + environment-root `cwd` containment of `Runtime.Exec`. Git is not involved: a verifier is a named (executable, args, cwd, kind, timeout) declaration run inside the Environment root, so a non-Git Environment runs it exactly as a Git one would. Missing verifier configuration must not block file/work development, so list/run are isolated additive tools and there is no environment-creation or inspection coupling to verifiers.

Smallest production-quality tracer-first shape: one new `internal/verifier` package holding a pure `Service` (definition CRUD + `Run`/`RunAll` policy mapping) over the existing `store.Store`, Environment-scoped definitions persisted in the `model.State` (global allowlist of runnable definitions reused across Environments is explicitly rejected this Phase). Agent-facing surface = `environment_verifier_list` + `environment_verifier_run`. Verification is invoked with `writer_owner` (verifiers may mutate build/test outputs under the root, so they are a write-class operation like `exec`). One real Streamable HTTP Gateway acceptance runs V2's own `go test ./...` as a declared `test` verifier on a non-Git checkout of this repository.

## Current Architecture

- **Runtime (`internal/runtime/runtime.go`)** is the only execution boundary. `Runtime.Exec(ctx, executable, args, cwd, timeoutMS, maxOutputBytes)` already resolves `executable` against the Runtime allowlist, enforces environment-contained cwd, applies timeout and output bounds, hides the Windows console, returns nonzero exit as a structured `CommandResult`, and returns start/timeout failures as errors.
- **CommandResult** is `{ExitCode, Stdout, Stderr}`. There is no `Duration`/`TimedOut`/`Summary` field today; timeout is signaled only as a Go error. This is the gap VERIFY-03 should close at the verifier layer, not by changing the generic Exec contract.
- **App service (`internal/app/service.go`)** is the execution seam owner. `Service.Exec` enforces the writer lease and keeps it alive while invoking `Runtime.Exec`. `Service.Runtime(environmentID)` builds a Runtime from `env.Root` plus globally allowed executables.
- **Gateway (`internal/gateway/server.go`)** exposes thin MCP handlers over app services; `exec` requires `writer_owner`.
- **State model (`internal/model/types.go`)** currently stores global execution allowlist and Environment-scoped capability selections. Verifier definitions are project-root facts and should live directly on Environment rather than using the global catalog pattern used for shared Skill/MCP capabilities.
- **Persistence (`internal/store/store.go`)** already provides atomic state updates; under the repository's no-pre-stable-compatibility rule, adding an optional Environment field does not require a migration shim.
- **Acceptance precedent** from Phase 1 uses a real Streamable HTTP MCP client against `NewHTTPHandler(service)` and proves real-host capability consumption. Phase 2 should reuse that acceptance shape for `go test ./...` through the verifier tool.
- **Negative-capability precedent**: optional Git failure is local and does not disable unrelated file operations. Missing verifier config must behave the same way.
- **Exec allowlist scope** remains global; verifier definition scope is per-Environment. This keeps one execution security control while keeping project-specific verifier declarations local to the Environment.

## Recommended Design

Introduce `internal/verifier` as a small verifier layer over existing state + Runtime execution. Definitions persist on each Environment. Running a verifier resolves an Environment-scoped definition by stable verifier ID, enforces writer ownership, executes through `Runtime.Exec`, then maps the outcome into a structured verifier result with identity/status/duration/timeout/output.

Key decisions:

- **Definition ownership/persistence/scoping:** Environment-scoped, persisted inline on `Environment` as `Verifiers []VerifierDefinition`. A verifier is a fact about one project root: executable, args, cwd, kind and timeout. Global shared verifier definitions are a non-goal for Phase 2.
- **Writer semantics:** `environment_verifier_list` is read-only and requires no writer. `environment_verifier_run` requires the Environment writer because verifier commands can mutate project/build outputs just like generic `exec`.
- **Runtime policy reuse:** verifier execution must call the existing Runtime path. Executables remain subject to the global allowlist; cwd remains Environment-contained; timeout/output bounds remain Runtime-enforced. Verifier must not create a bypass execution path.
- **Result shape:** do not change the generic `CommandResult` unless implementation proves it necessary. The verifier layer measures duration and maps timeout/nonzero exit into a typed `VerifierResult`.
- **Gateway surface:** `environment_verifier_list` and `environment_verifier_run` are the required Agent-facing tools. Human UI remains frozen; CLI definition management is optional and should be added only if needed to satisfy the Phase acceptance path.
- **No-config semantics:** zero configured verifiers is valid. List returns empty; attempting to run a missing/disabled verifier fails locally and clearly. Normal Environment/file development remains unaffected.

## Data Model

```go
type VerifierDefinition struct {
    ID             string   `json:"verifier_id"`
    Kind           string   `json:"kind"` // test|lint|build|custom
    Enabled        bool     `json:"enabled"`
    Executable     string   `json:"executable"`
    Args           []string `json:"args,omitempty"`
    Cwd            string   `json:"cwd,omitempty"` // empty = Environment root
    TimeoutSeconds int64    `json:"timeout_seconds,omitempty"`
    Name           string   `json:"name,omitempty"`
}
```

Environment gains:

```go
Verifiers []VerifierDefinition `json:"verifiers,omitempty"`
```

Verifier results are ephemeral, not persisted:

```go
type Result struct {
    ID         string `json:"verifier_id"`
    Kind       string `json:"kind"`
    Status     string `json:"status"` // passed|failed|skipped
    ExitCode   int    `json:"exit_code"`
    DurationMs int64  `json:"duration_ms"`
    TimedOut   bool   `json:"timed_out"`
    Summary    string `json:"summary,omitempty"`
    Stdout     string `json:"stdout,omitempty"`
    Stderr     string `json:"stderr,omitempty"`
}
```

Timeout is represented as `Status=failed` plus `TimedOut=true`, matching the earlier validated ADM verifier pattern while preserving explicit failure identity.

## Runtime/Gateway Flow

`environment_verifier_list {environment_id}`:

1. App resolves the Environment.
2. Return that Environment's verifier definitions.
3. No writer is required.

`environment_verifier_run {environment_id, writer_owner, verifier_id}`:

1. Resolve Environment and verifier definition.
2. Missing/disabled verifier → local tool error.
3. Build Runtime from Environment root + global executable allowlist.
4. Require the matching writer owner and maintain the writer heartbeat while the process runs.
5. Call `Runtime.Exec` with verifier executable/args/cwd/timeout/output limit.
6. Measure duration and classify:
   - exit 0 → passed;
   - nonzero exit → failed with exit code;
   - timeout → failed with `timed_out=true`;
   - start/policy failure → local tool error.
7. Return identity + bounded structured result.

Both tools run over the existing Stdio and Streamable HTTP Gateway with no new transport layer.

## Security/Policy

- A verifier binary must already be trusted by the existing executable allowlist. Defining a verifier never grants execution authority.
- Verifier cwd is still resolved through Runtime containment; absolute/escape/symlink escapes remain rejected.
- Verifier run requires the Environment writer and uses the existing lease-heartbeat/cancellation behavior.
- Output remains bounded by the existing Runtime output limit.
- Verifier definitions contain executable/args/cwd only; no new secret storage is introduced.
- A broken/missing/disabled verifier must fail locally and must not make unrelated Gateway tools unavailable.

## Validation Architecture

1. **Model/persistence:** Environment verifier definition persists/reloads; no verifier round-trips as empty/absent; missing ID returns a clean error.
2. **Verifier mapping:** helper-process tests for pass, nonzero fail, timeout, disabled, missing, stdout/stderr and output bounds.
3. **App seam:** verifier list is Environment-scoped; run requires matching writer; wrong/missing owner fails; no verifier does not affect normal Runtime/file behavior.
4. **Gateway tools:** list/run are surfaced; disabled/missing verifier fails locally; a broken verifier does not break `gateway_info`, `read`, etc.
5. **Non-Git acceptance:** a plain non-Git Environment can run a configured verifier.
6. **Negative capability:** zero verifier config still permits normal tree/read/write/edit operations.
7. **Real Streamable HTTP acceptance:** through a real MCP client, configure a `test` verifier for `go test ./...`, acquire writer, call `environment_verifier_run`, assert verifier identity + passed + exit code 0. Also prove one failing/timeout verifier returns a structured bounded failure.

## Implementation Order

1. Add `VerifierDefinition` to the model and Environment state.
2. Add Environment-scoped verifier definition operations with stable IDs.
3. Add `internal/verifier` result/classification layer and focused tests.
4. Add App list/run seams; reuse writer heartbeat from `Exec`.
5. Add Gateway `environment_verifier_list` and `environment_verifier_run` plus integration tests.
6. Add only the minimal management/CLI configuration path required by acceptance; keep Desktop frozen.
7. Run real Streamable HTTP `go test ./...` acceptance and negative failure/timeout coverage.
8. Update contract/planning verification artifacts only after implementation evidence exists.

## Risks/Non-goals

- **Timeout identity:** current Runtime reports timeout as an error rather than a field. Prefer deriving `TimedOut` in verifier without changing generic Exec; only make a minimal Runtime signal change if tests prove string-based classification unreliable.
- **Writer lease:** long verifiers must reuse the existing Exec heartbeat/cancel pattern rather than inventing another lifecycle.
- **Scope creep:** no global shared verifier catalog, verifier-file autodiscovery, process lifecycle, worktrees, Agent Runs, MCP expansion, GSD orchestration, Desktop polish, installer work, or migration layer in Phase 2.
- **Completion standard:** CRUD/config alone does not complete the Phase. Phase 2 closes only after a real Agent-facing Gateway run of V2 `go test ./...` and structured fail/timeout behavior are proven.

## RESEARCH COMPLETE
