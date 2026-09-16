# Phase 2 — Structured Verifier Runtime Validation Map

**Phase:** 02-structured-verifier-runtime  
**Goal:** Make change → verify a first-class structured Agent capability instead of an ad-hoc `exec` convention.  
**Requirements:** VERIFY-01, VERIFY-02, VERIFY-03, VERIFY-04  
**Precedent:** `.planning/phases/01-skill-runtime/01-VERIFICATION.md`, `01-UAT.md`, `01-01-SUMMARY.md`.

Phase 1 proved real capabilities with a real Streamable HTTP MCP client against `NewHTTPHandler(service)`, Environment-gated tools, and the rule that CRUD/metadata alone never closes a Phase. Phase 2 uses the same standard.

## Acceptance Layers

### 1. Unit — verifier classification + policy

Target: `internal/verifier/*_test.go` using a helper-process/scriptable subprocess seam.

- **V-U1** exit 0 → `{status:passed, exit_code:0, timed_out:false}`.
- **V-U2** nonzero exit → `{status:failed, exit_code:<n>, timed_out:false}` with captured bounded stdout/stderr.
- **V-U3** timeout → `{status:failed, timed_out:true}` plus measured `duration_ms`; never a hang and never a false pass.
- **V-U4** output above the Runtime bound is truncated/bounded inside the structured result rather than becoming a transport failure.
- **V-U5** start/policy failure (unresolvable executable, not allowlisted, invalid cwd) is a local error distinct from a returned failed verifier result.
- **V-U6** disabled or missing verifier ID returns a clean local error.

### 2. Integration — persistence, app seam, Gateway

Targets: `internal/app/service_test.go`, `internal/gateway/server_test.go`, store/environment tests.

- **V-I1** Environment `Verifiers []VerifierDefinition` round-trips through persistence; zero config reloads as absent/empty with no phantom verifier.
- **V-I2** `environment_verifier_list` is Environment-scoped and needs no writer; it cannot expose another Environment's definitions.
- **V-I3** `environment_verifier_run` requires the matching `writer_owner`, reusing the existing `Service.Exec` RequireWriter + heartbeat pattern; missing/wrong owner fails locally and a long verifier keeps the lease alive.
- **V-I4** verifier cwd goes through Runtime containment; escape and symlink-outside-root cases are rejected.
- **V-I5** defining a verifier never grants execution authority: an executable absent from the global allowlist may be listed as configuration but `run` fails locally.
- **V-I6** Gateway exposes `environment_verifier_list` + `environment_verifier_run`; a missing/disabled/broken verifier fails locally and does not break unrelated Gateway tools.
- **V-I7** with zero verifiers, normal `tree`/`read`/`search`/`write`/`edit`/`delete`/`exec` behavior remains available.

### 3. Real Streamable HTTP acceptance

Target: dedicated `internal/gateway/verifier_acceptance_test.go` with a real MCP client over `NewHTTPHandler(service)`. The non-Git repo-copy fixture must exclude this one acceptance test file and assert it is absent from the copy, preventing recursive V-R2 re-entry while leaving the rest of the V2 suite available to the inner `go test ./...`.

- **V-R1** a **non-Git** Environment runs a configured `test` verifier after `go` is explicitly allowlisted and a writer is acquired; result includes verifier identity + `{status:passed, exit_code:0}`.
- **V-R2** the same non-Git acceptance invokes V2's own `go test ./...` through a declared verifier and returns `passed`, exit code 0, and bounded output.
- **V-R3** one deliberately failing verifier and one deliberate timeout both return identity + structured bounded failure over the real HTTP client; timeout is explicitly identified by `timed_out:true`.

## VERIFY → Evidence Map

| Requirement | Truth to prove | Evidence |
|---|---|---|
| VERIFY-01 | Verification is optional; no verifier is required to create/inspect/develop an Environment. | V-I7 plus empty `environment_verifier_list`. |
| VERIFY-02 | Environment/project can declare test/lint/build/custom definitions and run them without Git. | V-I1, V-I4, V-R1. |
| VERIFY-03 | Agent Gateway returns verifier identity, structured status, exit code, duration, timeout identity, and bounded output. | V-U1..V-U6, V-R2, V-R3. |
| VERIFY-04 | Missing verifier config does not block normal Environment/file development. | V-I7; missing-ID run fails locally while file/runtime tools stay green. |

## ROADMAP Success Criteria → Evidence Map

| Criterion | Evidence |
|---|---|
| 1. A non-Git Environment can run a configured verifier. | V-R1 |
| 2. Pass/fail/timeout return verifier identity and structured bounded results. | V-U1..V-U4 + V-R3 |
| 3. Missing verifier configuration does not block normal Environment/file development. | V-I7 |
| 4. V2 `go test ./...` can be invoked through the ADM verifier capability. | V-R2 |

## Required Negative Tests

- Plain non-Git directory remains developable and can run a configured verifier; Git is not the gate.
- Zero verifier configuration still permits normal file/runtime work.
- Missing/disabled verifier ID returns a clean local error; unrelated Gateway tools remain usable.
- Verifier executable not on the global allowlist fails at run time; definition existence never grants execution authority.
- Verifier cwd outside the Environment root, including symlink escape, is rejected.
- `environment_verifier_run` without or with the wrong `writer_owner` fails locally.

## Timeout / Failure / Bounded-Output Checks

- Timeout → `{status:failed, timed_out:true}` + `duration_ms`; never a hang and never `passed`.
- Nonzero exit → `{status:failed, exit_code:<n>}` distinct from timeout.
- Output remains bounded by Runtime `maxOutputBytes`; truncation/bounding is represented inside the result rather than as a transport failure.
- Start/policy failure remains a local tool error distinct from a completed verifier failure result.

## Writer / Allowlist / cwd Policy Checks

- **Writer:** list is read-only; run requires matching writer owner with the existing heartbeat/cancel semantics.
- **Allowlist:** verifier execution reuses `Runtime.Exec` against the global executable allowlist; no new execution primitive or bypass path.
- **cwd:** verifier cwd resolves through Runtime containment; root default and contained subdirectories are valid, escapes are rejected.

## Real `go test ./...` Acceptance

The normal full `go test ./...` suite must stay green as a precondition. Independently, V-R2 proves the **product capability** by invoking `go test ./...` through `environment_verifier_run` on a non-Git Environment, so Phase 2 criteria 1 and 4 are proven without Git.

## Phase Exit Gate

Phase 2 closes only when all of the following hold:

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `git diff --check` is clean.
4. **V-R2** passes: a real Streamable HTTP MCP client runs V2 `go test ./...` via `environment_verifier_run` on a non-Git Environment and returns verifier identity + `{status:passed, exit_code:0}`.
5. **V-R3** passes over the same real HTTP boundary: one failing and one timeout verifier each return identity + structured bounded failure, with timeout explicitly identified.
6. **V-R1, V-I1..V-I7, V-U1..V-U6** are green, including all required negative tests.
7. `.planning/PROJECT.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, Phase verification/UAT artifacts are updated only after the above evidence exists.

Unit tests alone do not close this Phase. CRUD/configuration alone does not close this Phase.

## VALIDATION READY
