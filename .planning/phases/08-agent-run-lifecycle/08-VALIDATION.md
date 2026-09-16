# Phase 8 Validation — Agent Run Lifecycle

## Acceptance matrix

| ID | Required truth | Deterministic evidence |
|---|---|---|
| P1 | Run start returns stable `run_` identity and immediately returns while command remains running | owner/Gateway test starts blocking helper command and observes `running` |
| P2 | Run outlives launching client and later client can list/status it | real Streamable HTTP acceptance closes first client, second client sees same `run_` |
| P3 | finite command result reaches terminal lifecycle accurately | focused tests cover exit 0 -> `succeeded`, non-zero -> `failed` with exit code/result |
| P4 | cancellation is writer-exclusive and deterministic | wrong writer rejected; matching writer cancel -> `canceled`; child process tree exits |
| P5 | existing Runtime authority remains the only command authority | forbidden executable and escaped cwd fail before Run installation; managed Runtime revalidation stays inherited |
| P6 | Environment/owner cleanup cancels active Runs | focused owner cleanup and real Gateway stop release helper process/resources |
| P7 | restart does not resurrect Run observations | real Gateway restart has new owner, empty run list, old run status fails; `state.json` contains no run identity/observed state |
| P8 | unrelated capabilities remain green | existing process/file/non-Git Gateway regression plus full `go test ./...` |

## Verification gate

- focused `go test ./internal/gateway -run AgentRun -count=1`
- real restart acceptance, repeated before integration review
- `go test ./...`
- `go vet ./...`
- `git diff --check`

## Platform / scope note

Phase 8 proves lifecycle on the current Windows target using the same Runtime cancellation policy already validated by prior phases. It does not claim a new cross-platform process implementation; it reuses the existing Runtime seam. No LLM/subagent execution is part of acceptance.
