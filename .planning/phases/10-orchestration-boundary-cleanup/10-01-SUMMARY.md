# Phase 10 Plan 10-01 Summary — Orchestration Boundary Cleanup

**Implementation commit:** `36202489c13111cba2621b6d5a373707b7fde3f2`
**Status:** implemented and locally verified; integration review pending
**Requirements:** BOUNDARY-01, BOUNDARY-02, BOUNDARY-03, ARUN-01, ADM-GOAL-002, ADM-GW-003

## Delivered

Phase 10 removes the mis-scoped ADM-owned Planner/Executor/Reviewer workflow layer and restores the Agent Run contract to one task-semantic-neutral asynchronous command.

Removed production behavior:

- `run_workflow_start` Gateway tool and workflow request inputs;
- `StartWorkflowRun` and the workflow plan/step/review/outcome runtime domain;
- workflow-specific Run kind/status fields;
- workflow-only unit and real HTTP acceptance suites.

Retained behavior:

- `run_start`, `run_list`, `run_status`, `run_cancel`;
- writer heartbeat while a Run is active;
- executable allowlist and Environment cwd/managed-worktree validation;
- timeout and bounded stdout/stderr;
- owner Environment-drop / owner-close cleanup;
- owner-local Run observation only, with no restart resurrection.

The Gateway regression now explicitly proves that `run_workflow_start` is absent from the advertised tool list and cannot be called through the official MCP client.

## Boundary result

Production source under `internal/` contains no `StartWorkflowRun`, workflow Run/domain symbols, or `gsd_phase_*` implementation. `.planning/STATE.md` appears only as literal Skill test content; ADM contains no planning-provenance interpreter or state-advance implementation.

The abandoned `feat/gsd-phase-executor` branch was not touched or merged.

## Acceptance highlights

- focused generic Run lifecycle/authority/cleanup tests passed;
- real Streamable HTTP Run restart acceptance passed;
- plain non-Git Gateway, external MCP lifecycle/tool call, Skill Environment gating/read, verifier, dev process lifecycle and managed worktree isolation regressions passed together;
- final `go test ./...` passed after the strengthened retired-tool negative test;
- `go vet ./...` passed;
- `git diff --check d52af4...HEAD` passed.

## Product effect

ADM is again a local AI development control plane rather than a task orchestrator. External Agents/GSD/orchestrators may compose ADM files, exec, process, verifier, MCP, Skill, Git/worktree and generic Run capabilities, but ADM no longer owns planning, sequencing, review policy or phase completion.

## Verification

See `10-VERIFICATION.md`, `10-UAT.md`, and `evidence/regression.json`.
