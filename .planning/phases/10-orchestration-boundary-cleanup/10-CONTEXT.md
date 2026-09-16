# Phase 10 Context — Orchestration Boundary Cleanup

## Goal

Remove ADM-owned Planner / Executor / Reviewer workflow semantics introduced by historical Phase 9 while preserving ADM's generic asynchronous command Run and every unrelated Core capability.

## Requirements

- BOUNDARY-01 — remove the Agent-facing workflow orchestration surface/domain while retaining generic single-command `run_` lifecycle.
- BOUNDARY-02 — no GSD `.planning` interpretation/state-advance API is merged or introduced.
- BOUNDARY-03 — cleanup must not regress files, exec, verifier, process, MCP, Skill, Git/worktree, Environment or generic Run behavior.

## Product boundary

ADM owns capability execution, lifecycle, authorization, routing and diagnostics.

Agent/GSD/orchestrator owns task planning, step sequencing, review policy, phase completion and next-task decisions.

## Locked cleanup decisions

1. Delete `run_workflow_start` from the Agent Gateway.
2. Delete the workflow-specific request/status/domain implementation and workflow-specific tests/acceptance.
3. Restore `agentRunStatus` / `ownedAgentRun` to a single-command task-neutral shape; do not preserve workflow compatibility fields during active development.
4. Preserve Phase 8 `run_start`, `run_list`, `run_status`, `run_cancel`, writer heartbeat, Runtime allowlist/cwd/managed-worktree validation, timeout/output bounds, owner cleanup and restart-empty semantics.
5. Do not merge, cherry-pick or recreate anything from `feat/gsd-phase-executor`.
6. Historical Phase 9 planning/evidence files remain as audit history. They may mention FLOW-01/run_workflow_start but must not be treated as current requirements.
7. No MCP or Skill redesign is implemented in this cleanup Phase; their current behavior is regression-protected, and completion work begins in Phase 11/12.

## Non-goals

- no new workflow/orchestrator abstraction;
- no GSD state automation;
- no parallel Agent runner;
- no MCP transport/auth expansion yet;
- no Skill dependency model yet;
- no Desktop redesign;
- no Git merge/push automation.

## Acceptance shape

The best proof is deletion plus regression:

- Gateway no longer lists or accepts `run_workflow_start`;
- generic real HTTP Run lifecycle still works across clients/restart;
- real external MCP and real Skill acceptance remain green;
- verifier/process/worktree/non-Git Gateway acceptance remain green;
- source search outside historical `.planning/phases/09-*` and rebaseline docs finds no workflow runtime/domain API.
