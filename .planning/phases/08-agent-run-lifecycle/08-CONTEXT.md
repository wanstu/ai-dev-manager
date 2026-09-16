# Phase 8 Context — Agent Run Lifecycle

## Entry

Phase 7 is integrated to local `master` and R2 is complete locally. The user explicitly authorized continuing. Phase 8 starts from `master@24f1df9` on `feat/agent-run-lifecycle` and must remain isolated from master until review/authorization.

## Goal

Add the smallest real Agent Run lifecycle under the existing persistent Gateway owner: stable identity, lifecycle status, later-client visibility and cancellation. A Run must outlive the client request that starts it, but must not survive or resurrect across Gateway-owner restart.

## Locked decisions

- A Run is owner-local observed runtime state. `run_` identities and status/results are not persisted to `state.json`.
- The first concrete Run payload is exactly one asynchronous allowlisted Environment-scoped command. This gives ARUN-01 a real consumption path without introducing Phase 9 multi-step Planner/Executor/Reviewer semantics.
- Run start and cancel are writer-gated. List/status are read-only and require only a valid Environment.
- Run execution reuses `app.Service.Runtime` and `runtime.Runtime.Exec`, therefore it inherits executable allowlist, cwd containment, managed-worktree revalidation, bounded command output and OS process-tree cancellation.
- A running Run heartbeats the Environment writer while owned by the Gateway. Writer heartbeat failure cancels the Run and is observable as an error kind.
- Lifecycle states are `running`, `succeeded`, `failed`, `canceled`. A non-zero command exit is `failed`; explicit cancellation or owner shutdown is `canceled`.
- Gateway owner shutdown cancels active Runs and waits boundedly for cleanup. A restarted owner begins with an empty Run set; there is no automatic resume/reconciliation.
- Removing an Environment drops/cancels Runs owned for that Environment before metadata removal, matching existing process/session cleanup behavior.

## Non-goals

- no Planner/Executor/Reviewer workflow or multi-step orchestration (Phase 9)
- no GSD plan/phase execution or planning-state advance (Phase 10)
- no parallel Runs or automatic worktree allocation/integration (Phase 11)
- no LLM/subagent invocation
- no Run persistence/resume across Gateway restart
- no generic raw-PID/process manager
- no Desktop expansion
- no push or automatic master merge
