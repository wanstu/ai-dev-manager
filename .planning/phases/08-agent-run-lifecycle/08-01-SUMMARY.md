# Phase 8 Plan 08-01 Summary — Agent Run Lifecycle

**Implementation commit:** `a74b318f883e65a2ff8eb62fc043271bb66c2277`
**Status:** implemented and locally verified; integration review pending
**Requirements:** ARUN-01, LIFE-01, LIFE-03, ADM-CORE-005, ADM-CORE-007, ADM-GW-001, ADM-GW-003, ADM-DEV-001, ADM-DEV-003

## Delivered

Phase 8 adds the first real Agent Run lifecycle as an owner-local asynchronous Runtime command. A Run receives stable `run_` identity and lifecycle state `running`, `succeeded`, `failed`, or `canceled`. It is owned by the existing persistent Gateway runtime owner rather than by the MCP request/client that starts it.

The first Run payload is intentionally one command only. `run_start` requires the matching Environment writer and resolves the existing app Runtime, so executable allowlist, Environment-relative cwd containment, managed-worktree revalidation, timeout, bounded output and OS process-tree cancellation remain the same authority already used by ordinary `exec`. No second execution policy was introduced.

The Gateway now exposes `run_start`, `run_list`, `run_status`, and `run_cancel`. A later MCP client connected to the same running Gateway can observe and cancel a Run by stable ADM identity after the launching client disconnects. Cancel requires the matching writer. While a Run is running, the owner heartbeats that writer lease.

Terminal Run observation is retained only in owner memory. Successful commands retain exit code 0 and bounded result; non-zero commands become `failed` with their exit code; timeout becomes `failed` with `error_kind=timeout`; explicit/owner cancellation becomes `canceled`. Failed authority checks are rejected before a Run is installed.

Gateway owner close cancels active Runs and waits boundedly for cleanup. Owner Environment drop also cancels/forgets affected Runs. Run identities/results are never added to `state.json`; a restarted Gateway owner starts with an empty Run set and old IDs are invalid.

## Acceptance highlights

- real long-running helper returns `run_` immediately while still serving a loopback port;
- launching MCP client disconnects; a later client lists/statuses the same Run;
- wrong writer cannot cancel; matching writer cancellation releases the helper port;
- success, non-zero failure, timeout and bounded-output paths are independently tested;
- forbidden executable and escaped cwd are rejected without installing a Run;
- owner close/drop cleanup passed;
- real Streamable HTTP Gateway shutdown/restart acceptance passed repeatedly; stale Run state is neither persisted nor resurrected;
- existing full Gateway suite and all repository packages remain green.

## Explicit non-goals preserved

Phase 8 does not add Planner/Executor/Reviewer contracts, multi-step workflow state, GSD phase execution, automatic worktree creation for Runs, parallel lane scheduling, automatic Git integration, Desktop Run UI, or Run persistence/recovery. Those belong to later phases.

## Verification

See `08-VERIFICATION.md`, `08-UAT.md`, and `evidence/regression.json`.
