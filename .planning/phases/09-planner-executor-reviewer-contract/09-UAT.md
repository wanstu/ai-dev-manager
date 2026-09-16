# Phase 9 UAT — Planner / Executor / Reviewer Contract

Status: automated UAT passed; integration review passed after dynamic Runtime/writer authority fixes. No human-only step is required for the Phase 9 contract.

## UAT 1 — Accepted workflow is auditable

Passed.

A workflow started through the Agent Gateway exposes one stable `run_` identity. Its status contains the materialized plan, stable ordered steps, executor results, verifier-backed review result, and final `accepted` outcome. A later MCP client can inspect the same audit state.

## UAT 2 — Review rejection is a decision, not an orchestration crash

Passed.

A verifier that executes normally and returns a failed verification yields:

- Run state `succeeded`;
- workflow outcome `rejected`;
- review state `rejected`;
- structured verifier evidence;
- no orchestration error classification.

This is the key FLOW-01 distinction required before GSD state advancement can be designed in Phase 10.

## UAT 3 — Runtime/orchestration failures remain visible and distinct

Passed.

- executor non-zero failure is recorded on the affected step and classified `executor_step_failed`;
- reviewer invocation/configuration failure is classified `reviewer_error`;
- neither is confused with a normal review rejection.

## UAT 4 — Existing authority remains authoritative

Passed.

Executor preflight and execution reuse the existing Runtime allowlist/cwd boundary. Unsafe executor inputs fail locally before Run installation. Integration review additionally proved every later step rechecks the matching writer and resolves a fresh Runtime, so mid-workflow writer takeover, allowlist revocation, or managed-worktree identity changes cannot ride a stale start-time snapshot. Reviewer execution reuses existing Environment verifier definitions and writer authority. Workflow cancellation reuses ordinary writer-gated `run_cancel`.

## UAT 5 — Real Streamable HTTP and owner-local observation

Passed.

`TestWorkflowLifecycleThroughRealStreamableHTTP` exercises the workflow through the real MCP Streamable HTTP handler, disconnects/reconnects clients, and inspects accepted/rejected workflow state from a later client. The workflow observation is not serialized into `state.json`.

Repeated fresh acceptance (`-count=5`) passed.

## UAT 6 — Phase boundary

Passed.

No LLM subagents, GSD phase executor/state mutation, parallel lane orchestration, or automatic Git integration is present in the Phase 9 diff.

## Result

Phase 9 UAT passes for FLOW-01 and integration review has passed on final source `50fe9ae`. Local master integration is the next gate; Phase 10 must not start until Phase 9 integration is separately decided.
