# Phase 10 Validation — Orchestration Boundary Cleanup

## Required automated truths

### P1 — Workflow tool is gone

Gateway tool listing does not contain `run_workflow_start`. Attempting to call that tool through the real MCP transport returns an unknown/unavailable tool result rather than invoking hidden compatibility behavior.

### P2 — Generic asynchronous Run remains intact

A real allowlisted command can still be started through `run_start`, observed from a later client, canceled by the matching writer, cleaned up on owner shutdown, and absent after restart.

### P3 — Run status is task-semantic-neutral

`run_status` contains command lifecycle/result facts only. It does not expose workflow plan/step/review/outcome fields or workflow-kind semantics.

### P4 — Authority is unchanged

Forbidden executable, escaped cwd and wrong-writer cancel remain rejected. Managed-worktree Runtime validation remains in the ordinary Run path.

### P5 — Core regressions stay green

At minimum verify:

- plain non-Git Gateway development;
- external MCP lifecycle/tool call;
- real Skill Environment gating/read;
- structured verifier execution;
- dev process lifecycle;
- managed worktree optional isolation;
- generic Run real HTTP restart lifecycle.

### P6 — No GSD implementation leaks into Core

Production source outside historical planning/evidence contains no `gsd_phase_*`, planning-provenance/state-advance implementation, or new Agent orchestration API.

## Final gates

- focused generic Run tests;
- real HTTP Run restart acceptance repeated;
- focused MCP/Skill/verifier/process/isolation regressions;
- `go test ./...` (or explicit complete package/test grouping if the connector execution ceiling prevents observing a single aggregate call; record that distinction honestly);
- `go vet ./...`;
- `git diff --check`;
- source grep confirms workflow production symbols are removed.

## Evidence rule

Deletion is not considered successful merely because the code compiles. Evidence must prove the generic Runtime path and the MCP/Skill core paths still work after orchestration removal.
