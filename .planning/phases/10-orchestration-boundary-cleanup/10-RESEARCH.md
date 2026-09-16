# Phase 10 Research — Orchestration Boundary Cleanup

## Source audit

Historical Phase 9 added orchestration in only a small set of production paths:

- `internal/gateway/runtime_workflow.go` — workflow plan/step/review/outcome domain and execution state machine.
- `internal/gateway/runtime_run.go` — workflow-aware Run kind/status/owned state fields.
- `internal/gateway/server.go` — `RunWorkflowStepInput`, `RunWorkflowStartInput` and `run_workflow_start` registration.

Workflow-only tests:

- `internal/gateway/runtime_workflow_test.go`
- `internal/gateway/workflow_acceptance_test.go`

The Phase 8 baseline at `4520baf` already contains the desired generic single-command Run implementation. Comparing current source to that commit gives a precise cleanup reference rather than inventing another abstraction.

## Keep / remove split

### Keep

- persistent Gateway owner and `runs map[string]*ownedAgentRun`;
- stable `run_` identity;
- `StartAgentRun` command preflight and async execution;
- `run_start`, `run_list`, `run_status`, `run_cancel`;
- writer authorization/heartbeat;
- app Runtime resolution and managed-worktree revalidation;
- timeout, bounded output and process-tree cancellation;
- owner cleanup/drop/restart-empty observation.

### Remove

- `agentRunKind` / workflow-kind distinction if it exists only to distinguish the removed workflow type;
- `Workflow *workflowRunStatus` in Run status;
- `workflow *ownedWorkflowRun` in owned Run state;
- all workflow plan/step/review/outcome types and methods;
- `run_workflow_start` Gateway tool and its input structs;
- workflow-only tests and real HTTP acceptance.

## Why deletion is safer than a compatibility layer

The project explicitly has no pre-stable compatibility burden. Keeping deprecated workflow fields or a disabled compatibility endpoint would preserve the wrong product abstraction and encourage future dependencies. The clean boundary is to delete the mis-scoped surface while retaining Git history/evidence.

## Negative acceptance priorities

This Phase is primarily a regression safety exercise. The important failure modes are:

1. accidentally deleting generic Run lifecycle while deleting workflow code;
2. breaking Gateway owner cleanup because workflow used the same `ownedAgentRun` structure;
3. breaking server tool registration near the Run tools;
4. accidentally weakening Runtime/writer authority;
5. treating MCP/Skill/verifier/Git as prerequisites during cleanup;
6. accidentally importing abandoned Phase 10 GSD code.

## Regression anchors

Use current retained tests, especially:

- Phase 8 Agent Run unit + real HTTP restart acceptance;
- Runtime owner/process acceptance;
- external MCP lifecycle/real call acceptance;
- real Skill Gateway acceptance;
- verifier real/non-Git acceptance;
- managed worktree optional-isolation acceptance;
- plain non-Git Gateway development acceptance.

## Phase boundary

Do not use Phase 10 cleanup as an excuse to redesign Run, MCP or Skill. The desired result is smaller code and a corrected product boundary. MCP completion begins only after this cleanup is integrated.
