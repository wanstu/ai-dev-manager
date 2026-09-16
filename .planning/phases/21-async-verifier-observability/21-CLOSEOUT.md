# Phase 21 Closeout — Async Verifier + Long Operation Observability

Date: 2026-09-13
Status: COMPLETE
Planning commit: `2aceec3`
21-01 implementation: `5e772e7`
21-02 implementation: `3ee045c`

## Outcome

Phase 21 is complete. ADM now supports a distinct Gateway-owner-local asynchronous verifier lifecycle for configured Environment verifiers without turning verification into CI/task orchestration and without replacing the existing generic `run_` lifecycle.

The delivered `vfrun_` resource provides stable owner-local identity, start/list/status/cancel, bounded live stdout/stderr, explicit truncation evidence, writer heartbeat, matching-launch-writer cancellation, classified terminal `verifier.Result`, Environment/owner cleanup, and no persistence or restart resurrection.

The existing synchronous verifier remains available. It has no arbitrary duration cutoff and is not silently converted to background work. If its outer caller/request context interrupts execution, it returns the stable `blocking_request_interrupted` diagnostic pointing to the async start tool; the verifier's own configured timeout remains a normal failed timed-out verifier result.

## Delivered contract

- `ADM-CORE-021` is implemented.
- Agent and Admin expose the same async verifier tools:
  - `environment_verifier_run_start`
  - `environment_verifier_run_list`
  - `environment_verifier_run_status`
  - `environment_verifier_run_cancel`
- `environment_verifier_list` remains definition inventory.
- `environment_verifier_run` remains blocking compatibility execution.
- async verifier execution reuses the same Environment/verifier/writer/Runtime/allowlist/cwd/timeout/classification authority as the synchronous path.
- list/status are read-only and do not renew writer leases.
- async runs survive launching-client disconnect while their Gateway owner lives.
- active runs are canceled on Environment drop or owner shutdown.
- restarted/fresh owners begin with no verifier-run observations; `state.json` contains no verifier-run history/results.
- no automatic retry/replay or task/CI semantics were introduced.

## Acceptance

Integrated G01–G12 evidence is recorded in `21-02-SUMMARY.md`.

Final implementation head `3ee045c` passed:

- focused verifier/context/generic Run/process gate `run_00198299d68611ab`;
- full repository `run_2f67a98f9388baf6`;
- vet `run_137dfd200fb22288`;
- fixed-head artifact build `run_83f6d82aa3907bea`;
- clean implementation-head `git diff --check`.

Fixed-head artifact:

- `dist/ai-dev-manager-phase21-02-3ee045c.exe`
- 16,926,720 bytes
- SHA-256 `A5BADEB350B6F7901F8C42D90037596E5EEA040DF4A2F07B1817EFB0CBE43F4D`

No Wails build was required because Phase 21 has no Desktop code changes.

## Product boundaries after closeout

Phase 21 does not add a pipeline/job graph, retries, Planner/Executor/Reviewer semantics, GSD phase state, automatic continuation after request loss, persisted run history, raw PID control, or a verifier prerequisite for ordinary Environment/file work. Generic `run_` and `proc_` remain separate owner resources.

Phase 20 context composition remains passive; it only gained factual guidance recommending the async verifier lifecycle for long/heavy verification.

Phase 19 B09 native GUI click-through remains pending under its existing availability rule and is not changed by Phase 21.

## Next

Phase 22 — Temporary Task Environments + Safe Cleanup Workflow — remains planned and is **not started** by this closeout. It must be opened explicitly before feature work.

No push, tag, release, or subagents were used for Phase 21.
