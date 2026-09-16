# Phase 10 UAT — Orchestration Boundary Cleanup

**Status:** passed
**Date:** 2026-09-08
**Implementation:** `36202489c13111cba2621b6d5a373707b7fde3f2`

## User-facing acceptance

### 1. ADM no longer advertises workflow orchestration

Pass. A real Gateway MCP client lists the normal ADM tools but not `run_workflow_start`. Calling the retired name does not produce a successful tool result.

### 2. Generic Run remains usable

Pass. A single allowlisted command can still be started as `run_`, observed across client sessions, canceled by the matching writer, and cleaned up by owner shutdown. Run state remains command lifecycle/result data only.

### 3. Restart does not resurrect Runs

Pass. Real HTTP Gateway restart acceptance proves old owner-local Run identities are absent after restart while ordinary Gateway development remains available.

### 4. Existing Core capabilities still work

Pass. Acceptance coverage after cleanup includes:

- ordinary non-Git Environment development;
- external MCP health, tool discovery and tool call;
- enabled/disabled real Skill artifact access;
- verifier execution;
- long-lived dev process lifecycle;
- managed worktree optional isolation.

### 5. ADM does not take over GSD policy

Pass. Production source contains no GSD phase tool/API, `.planning` provenance interpreter or STATE advancement implementation. The abandoned GSD executor branch was not merged.

## Outcome

From an Agent/user perspective, Phase 10 is a subtraction of the wrong abstraction rather than a replacement feature. ADM remains the local capability/runtime control plane; planning, sequencing, review policy and phase completion stay with the external Agent/GSD/orchestrator.
