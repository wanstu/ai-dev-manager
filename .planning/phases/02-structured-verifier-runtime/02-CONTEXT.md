# Phase 02 Context — Structured Verifier Runtime

## Goal

Make “change → verify” a first-class, structured Agent capability instead of an ad-hoc `exec` convention. Phase 2 wraps the already-policy-enforced `Runtime.Exec` in a small, Environment-scoped verifier layer: persisted verifier definitions plus a structured verifier result, exposed through `environment_verifier_list` and `environment_verifier_run`.

No new execution primitive is introduced.

## Requirements

- **VERIFY-01** — Verification is an optional Runtime capability.
- **VERIFY-02** — Environment/project development can declare structured test/lint/build/custom verifiers without requiring Git.
- **VERIFY-03** — Verifier execution returns structured status, exit code, bounded output, timeout/failure identity, and is available through the Agent Gateway.
- **VERIFY-04** — Absence of verifier configuration does not block normal Environment/file development.

ROADMAP Phase 2 success criteria:

1. A non-Git Environment can run a configured verifier.
2. Pass/fail/timeout return verifier identity and structured bounded results.
3. Missing verifier configuration does not block normal Environment/file development.
4. V2 `go test ./...` can be invoked through the ADM verifier capability.

## Locked Constraints

- **Environment-scoped definitions, not a global catalog.** `Verifiers []VerifierDefinition` persists inline on each `Environment`.
- **No new execution path.** Verifier execution reuses `Runtime.Exec`; defining a verifier never grants execution authority.
- **Two Agent-facing Gateway tools only:**
  - `environment_verifier_list` — read-only, no writer required.
  - `environment_verifier_run` — writer required because verifier commands may mutate build/test outputs.
- **Result shape:** verifier identity, kind, `passed|failed|skipped`, exit code, duration, `timed_out`, summary, bounded stdout/stderr. Timeout is `status:"failed"` plus `timed_out:true`.
- **Do not change generic `Runtime.Exec` / `CommandResult` by default.** Timeout identity is derived at the verifier/app layer from the verifier-owned deadline context, not by brittle string matching.
- **No-config is valid.** Zero configured verifiers must not block Environment creation, inspection, file operations, or generic exec.
- **Writer semantics reuse existing Exec behavior.** `RunVerifier` uses the current RequireWriter + heartbeat/cancel pattern.
- **cwd and executable policy are inherited from Runtime.** cwd remains Environment-contained; executable authority remains the global allowlist.
- **Verifier results are ephemeral.** Do not persist results in Phase 2.
- **No migration compatibility layer.** The project is pre-stable; adding an optional Environment field does not justify a shim.

## Existing Architecture To Reuse

- `internal/runtime/runtime.go` — `Runtime.Exec`, executable allowlist enforcement, cwd containment, timeout, bounded output, Windows hidden process behavior.
- `internal/app/service.go` — `Service.Runtime`, `Service.Exec`, RequireWriter, heartbeat/cancel, Touch.
- `internal/model/types.go` — Environment state and global AllowedExecutables.
- `internal/store/store.go` — atomic state update/load.
- `internal/environment/service.go` — writer lease lifecycle and Environment lookup.
- `internal/gateway/server.go` — thin MCP tool handlers over app service.
- `internal/identity` — stable ID generation; use a verifier prefix such as `vf`.
- `internal/app/service_test.go` — helper-subprocess precedent for pass/fail/timeout process tests.
- `internal/gateway/server_test.go` — real Streamable HTTP MCP acceptance precedent from Phase 1.

## Decisions The Plan Settles

1. **Definition creation surface:** add a thin human CLI path `environment verifier add|list|remove`; do not add Desktop UI. The locked Agent surface remains list/run only.
2. **Definition service boundary:** new `internal/verifier` package owns Environment-scoped definition CRUD plus result/classification types. App owns writer policy and real Runtime execution.
3. **Timeout detection:** App owns the verifier deadline context and derives `timed_out` from `context.DeadlineExceeded`; no Runtime error-string parsing.
4. **Real `go test ./...` fixture:** put V-R1/V-R2/V-R3 in a dedicated `internal/gateway/verifier_acceptance_test.go`; copy this repository into a temporary non-Git root while excluding `.git`, planning artifacts, generated/build/cache output, and that one self-recursive acceptance test file. Assert the copied tree does not contain `verifier_acceptance_test.go`; then allowlist `go`, declare a verifier, and execute through the real HTTP Gateway client. The inner `go test ./...` still exercises the rest of the V2 suite without recursively re-entering V-R2.
5. **Environment list/inspect:** configured verifier definitions may appear with Environment facts; empty verifier lists remain omitted/empty. No private data is introduced.

## Exit Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `git diff --check` is clean.
4. Nyquist V-U1..V-U6, V-I1..V-I7, V-R1..V-R3 all pass.
5. Real Streamable HTTP acceptance proves a non-Git Environment can run V2 `go test ./...` through `environment_verifier_run` and receive verifier identity + `{status:"passed", exit_code:0}`.
6. The same real HTTP boundary proves one failed verifier and one timeout return structured bounded failures, with timeout explicitly identified.
7. PROJECT/ROADMAP/STATE/verification/UAT/SUMMARY are updated only after implementation evidence exists.

## Non-goals

- global verifier catalog or verifier-file autodiscovery
- persisted verifier results
- new execution primitive or generic `Runtime.Exec` / `CommandResult` redesign
- process/log/port lifecycle or persistent Runtime ownership
- Git worktree/isolation
- Agent Run lifecycle or Planner/Executor/Reviewer orchestration
- GSD phase executor/state-advance implementation
- external MCP runtime expansion
- Skill/Memory expansion
- Desktop feature work or distribution polish
- migration/compatibility layer
- unrelated source changes
