# Phase 21-02 Summary — Agent/Admin verifier lifecycle surface and long-operation acceptance

Date: 2026-09-13
Implementation head: `3ee045c` (`feat(gateway): expose async verifier lifecycle and diagnostics`)
Depends on: `5e772e7` (`feat(verifier): add owner-local async verifier lifecycle`)
Status: complete and accepted.

## Delivered

Phase 21-02 exposes the Phase-21 owner-local verifier lifecycle through the shared Agent/Admin Gateway surface without changing verifier definitions, generic `run_`, process semantics, or persisted state.

Shared tools now include:

- `environment_verifier_run_start`
- `environment_verifier_run_list`
- `environment_verifier_run_status`
- `environment_verifier_run_cancel`

`environment_verifier_list` still lists definitions. `environment_verifier_run` remains the synchronous compatibility path and now explicitly describes itself as blocking while directing long/heavy verification to the async lifecycle.

The async surface uses stable `vfrun_` IDs, bounded live stdout/stderr with truncation flags, terminal `verifier.Result`, launch-writer cancellation authority, owner-local cleanup, and no persisted/restarted observations. Agent and Admin expose the same verifier lifecycle tools over the same Gateway owner.

## Blocking verifier diagnostic

The synchronous verifier path now distinguishes a real outer caller/request interruption from the verifier's own configured timeout.

- caller interruption returns stable diagnostic kind/text `blocking_request_interrupted` and directs long work to `environment_verifier_run_start`;
- configured verifier timeout remains a normal classified failed `verifier.Result` with `timed_out=true`;
- successful short blocking execution is unchanged;
- no duration threshold, automatic async conversion, hidden continuation, retry, or replay was added.

## Context guidance

`environment_context_bundle` now factually recommends `environment_verifier_run_start/status/cancel` for long/heavy verification while retaining blocking `environment_verifier_run` for short/direct use. Existing context tests continue to prove bundle composition is passive: it does not run verifiers, probe MCPs, execute Git, read Memory values, or read Skill contents.

## Integrated acceptance — G01–G12

### G01 — optionality

A plain non-Git Environment with no verifier remains readable through ordinary file operations. Async start for an absent verifier fails locally and does not create a verifier-run resource.

### G02 — stable lifecycle across clients

Real Streamable HTTP acceptance starts a verifier from client A, closes A, reconnects client B to the same owner, and observes the same `vfrun_` through list/status. Client B sees bounded live output and the terminal result.

### G03 — cancellation authority

Wrong-writer cancel is rejected without changing the run. Matching launch writer reaches `canceled`; terminal status remains queryable while the owner lives.

### G04 — verifier semantics

Real HTTP async acceptance covers pass, non-zero failure and configured timeout. Terminal results preserve verifier identity/kind, passed/failed status, exit code, duration, timeout flag, summary and bounded output.

### G05 — bounded output

A large-output helper requested with `max_output_bytes=64` exposes exactly bounded stdout/stderr plus `stdout_truncated=true` and `stderr_truncated=true` while running and after completion.

### G06 — no ghost resources

Missing verifier, disabled verifier, wrong writer, forbidden executable, escaped cwd and tampered managed-worktree branch identity all fail before a `vfrun_` is installed.

### G07 — synchronous diagnostic

Application-level acceptance cancels the outer caller context and receives `blocking_request_interrupted` with async-start guidance. A separate configured-timeout regression remains a failed timed-out verifier result rather than a tool error.

### G08 — cleanup/restart/no persistence

21-01 owner tests retain Environment-drop and owner-close cancellation coverage. 21-02 real HTTP acceptance closes an owner with an active verifier, starts a fresh owner/server, confirms the verifier-run list is empty and the old `vfrun_` is invalid. `state.json` is checked for absence of verifier-run IDs/observations.

### G09 — existing Run/process boundaries

Final focused acceptance includes `AgentRun` and dev-process tests together with verifier tests. Async verifier remains a separate `vfrun_` resource and does not alias `run_` or `proc_`.

### G10 — context/privacy boundaries

Context guidance changed only as passive factual guidance. Existing Memory/Skill/MCP/Git non-execution/non-injection assertions remain green. Verifier list/status are read-only; a terminal list/status check proves writer `LastSeenAt` and `ExpiresAt` are unchanged.

### G11 — fixed-head regression/artifact

All final acceptance below ran from implementation head `3ee045c`:

- focused app/Gateway verifier + generic Run/process/context gate: `run_00198299d68611ab` PASS; app 7.346s, Gateway 108.690s;
- full repository `go test -count=1 ./...`: `run_2f67a98f9388baf6` PASS; Gateway 122.556s;
- `go vet ./...`: `run_137dfd200fb22288` PASS;
- fixed-head CLI/Gateway build: `run_83f6d82aa3907bea` PASS;
- `git diff --check`: PASS on the clean implementation head.

Artifact:

- path: `dist/ai-dev-manager-phase21-02-3ee045c.exe`
- bytes: `16,926,720`
- SHA-256: `A5BADEB350B6F7901F8C42D90037596E5EEA040DF4A2F07B1817EFB0CBE43F4D`

No Wails build was required because Phase 21 changed no Desktop code.

### G12 — closeout

This summary plus `21-CLOSEOUT.md` records delivered scope and acceptance. Project planning/state documents are advanced only after the fixed-head gates above passed.

## Additional focused evidence before fixed-head gate

- application caller-interruption/configured-timeout/context focused tests: PASS, 7.490s;
- async verifier HTTP/owner focused: `run_bc13922f97b96e90` PASS, Gateway 9.730s;
- async verifier + generic AgentRun/dev-process regression: `run_c4c06ebdffef3547` PASS, Gateway 14.960s;
- async verifier surface after expanded pass/fail/timeout acceptance: `run_9e75ceadc72a19c1` PASS, Gateway 10.447s.

## Boundaries preserved

No CI/job orchestration, retry policy, task/GSD semantics, persisted verifier history, raw PID control, Desktop feature expansion, normal CLI verifier lifecycle, new Git/MCP/Skill prerequisite, push, tag, release, or subagent work was introduced.
