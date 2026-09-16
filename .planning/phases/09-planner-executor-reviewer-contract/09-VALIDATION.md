# Phase 9 Validation — Planner / Executor / Reviewer Contract

## Automated acceptance matrix

| ID | Requirement / risk | Automated evidence |
|---|---|---|
| P1 | Planner materializes immutable visible plan | focused workflow unit test checks goal, stable ordered step IDs/spec and verifier refs in `run_status` |
| P2 | Existing Runtime authority is reused | forbidden executable and escaped cwd reject before Run installation; managed/non-Git regressions remain green |
| P3 | Executor audit | successful multi-step workflow records per-step pending/running/succeeded evidence and bounded command result |
| P4 | Review accepted | passing verifier yields Run `succeeded`, review `accepted`, workflow `accepted`, verifier result retained |
| P5 | Review rejection is not orchestration failure | normal failing verifier yields Run `succeeded`, review/outcome `rejected`, no orchestration error kind |
| P6 | Executor failure is distinct | non-zero step yields Run `failed`, `executor_step_failed`, failed step visible, review `not_run` |
| P7 | Reviewer infrastructure failure is distinct | invalid/disabled/policy-failing verifier invocation yields Run `failed`, review `error`, `reviewer_error` |
| P8 | Persistent-owner lifecycle reused | later client list/status/cancel works; owner close/Environment drop cleanup and Phase 8 restart tests stay green |
| P9 | Real Agent Gateway path | real Streamable HTTP accepted + rejected workflows are observed through `run_status` from a later client |
| P10 | No persistence / no Phase 10-11 leakage | state file excludes workflow observations; diff audit contains no GSD state advance, parallel orchestration or Git integration behavior |

## Final gates

- `gofmt` on touched Go files
- focused workflow tests
- focused race detector for workflow/Run concurrent status/cancel paths
- real Streamable HTTP workflow acceptance, repeated
- `go test ./internal/gateway`
- `go test ./...`
- `go vet ./...`
- `git diff --check`

## Failure classification expected by tests

- `run.state=succeeded`, `workflow.outcome=rejected`, `review.state=rejected`: reviewer ran successfully and rejected the work.
- `run.state=failed`, `error_kind=executor_step_failed`: executor command completed non-zero; reviewer did not run.
- `run.state=failed`, `error_kind=reviewer_error`, `review.state=error`: reviewer could not be executed as configured/authorized.
- `run.state=canceled`: explicit writer cancel or owner cleanup interrupted the workflow.

No human UAT is required if real HTTP acceptance covers the full structured flow and negative distinctions.
