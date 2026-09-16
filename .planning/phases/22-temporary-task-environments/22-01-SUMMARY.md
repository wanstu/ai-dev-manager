# 22-01 Summary — Core temporary Environment lifecycle

Date: 2026-09-13
Planning commit: `84cc9c7` (`docs: plan Phase 22 temporary environment lifecycle`) on Phase-21 closeout `b6c96cd`.
Status: complete and validated; 22-02 ready, not started.

## Delivered

- Added explicit temporary Environment create/status models plus `run_id` provenance in `ResourceRetention`; run/session provenance remains retention metadata only and has no execution/task semantics.
- Added `app.Service.CreateTemporaryEnvironment` with required Workspace/name/owner/positive TTL, trusted creator surface, default `existing_root` mode, optional Workspace-contained existing root, optional `managed_worktree` mode and explicit expiry persisted atomically with the Environment.
- Added `Environment.CreateNewWithRetention` so temporary creation refuses the ordinary idempotent duplicate shortcut and can never reuse an existing durable Environment as temporary success.
- Added retention-aware managed Environment/worktree persistence through `CreateManagedWithRetention` / `Isolation.CreateWithRetention`; the existing Git rollback path remains authoritative and is now directly tested with an injected post-worktree persistence failure.
- Added owner-scoped temporary promotion that changes retention only and preserves stable Environment identity/root/selections/private Memory/managed worktree metadata.
- Added Gateway-owner targeted temporary status/preview/execute helpers. Runtime-report execution is intersected with the supplied report, so a one-Environment cleanup cannot sweep another eligible Environment/MCP/Skill.
- Ordinary targeted cleanup removes only the Environment state/private context and preserves Workspace/project files. Managed targeted cleanup reuses `Isolation.Safety` plus non-force `Isolation.Destroy`, retaining the managed branch.
- Added a fresh runtime check before targeted execution and extended Environment retention blockers to active Phase-21 `vfrun_` resources. Only running verifier observations block; terminal verifier observations do not.
- No MCP tool registration, Desktop UI, CLI lifecycle UX, automatic GC, task record/orchestration, force cleanup, merge/push policy or second persistence model was added.

## Acceptance T01–T18

| Gate | Result | Evidence |
|---|---|---|
| T01 | PASS | Non-Git `existing_root` temporary creation persists temporary/owner/expiry plus session/run provenance. |
| T02 | PASS | Existing contained subdirectory succeeds; missing/outside root fails without installing an Environment. |
| T03 | PASS | Missing owner, non-positive TTL and invalid mode fail with no Environment record. |
| T04 | PASS | Existing durable same name/root collision returns a clear conflict and remains durable. |
| T05 | PASS | Temporary managed worktree persists managed + Environment identity/retention while source branch/HEAD remain unchanged. |
| T06 | PASS | Injected Environment persistence failure after Git worktree creation rolls back ADM Environment/managed metadata, worktree registration and generated `adm/*` branch. |
| T07 | PASS | Not-due temporary status reports `not_due` / `retention_not_expired` without mutation. |
| T08 | PASS | Wrong lifecycle owner cannot promote or execute targeted cleanup. |
| T09 | PASS | Matching-owner promotion becomes durable, clears temporary owner/session/run/expiry and later temporary cleanup refuses it. |
| T10 | PASS | Eligible ordinary targeted cleanup removes only Environment state; Workspace/root/file marker remain. |
| T11 | PASS | Eligible managed targeted cleanup removes the ADM-owned worktree and keeps its generated branch. |
| T12 | PASS | Dirty, unpublished and tampered managed worktrees all block targeted cleanup through existing safety/identity evidence. |
| T13 | PASS | Targeted cleanup of one expired Environment leaves another simultaneously eligible temporary Environment untouched. |
| T14 | PASS | Active writer is an explicit cleanup blocker. |
| T15 | PASS | Active dev process is an explicit cleanup blocker. |
| T16 | PASS | Active generic `run_` is an explicit cleanup blocker. |
| T17 | PASS | Active `vfrun_` adds `active_verifier_run`; terminal verifier observation no longer blocks cleanup. |
| T18 | PASS | Owner-local verifier/runtime blocker evidence is not serialized into `state.json`. |

## Validation

- Final focused owner/Core regression: `run_0e7fb13de9ea93e3` PASS.
  - `internal/app` 5.647s
  - `internal/environment` 0.031s (no matching tests)
  - `internal/isolation` 6.273s
  - `internal/gateway` 14.535s
- Timing/runtime repeat: `go test -count=3 ./internal/app ./internal/gateway -run "^TestTemporary"` PASS (`internal/app` 0.142s; `internal/gateway` 26.110s).
- Final full repository: `run_061053596b8e414a` PASS; Gateway 124.718s.
- Final `go vet ./...`: `run_1e123fe4c00bcbf7` PASS.
- `git diff --check` PASS; only existing Windows LF→CRLF working-copy warnings were emitted.

## Next

22-02 may now expose the shared Agent/Admin temporary Environment workflow tools and real Streamable HTTP acceptance. 22-02 has not started in this node.

No push, tag, release or subagents were used.
