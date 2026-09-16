---
phase: 06-dev-process-logs-ports
status: passed
nyquist_compliant: true
wave_0_complete: true
---
# Phase 6 Validation

| ID | Acceptance | Evidence target |
|---|---|---|
| P1 | start returns a stable `proc_` identity while server keeps running | owner unit test + real Gateway dogfood |
| P2 | later independent client list/status sees same process | real Streamable HTTP acceptance |
| P3 | stdout/stderr logs are bounded and queryable after client exit/process exit | bounded-log tests + Gateway acceptance |
| P4 | owned server listening port is reported as a fact | Windows owned-PID port test + real dev server acceptance |
| P5 | wrong writer, forbidden executable and escaped cwd are rejected; child start immediately renews the writer lease | negative Gateway/app/runtime tests; `TestProcessStartRenewsWriterLeaseBeforeChildLifetime` + RequireWriter source review |
| P6 | stop and owner Close terminate process tree and release port | focused Windows integration + real Gateway stop |
| P7 | process/log/port observation is not persisted or resurrected after restart | private state/restart acceptance |
| P8 | file/non-Git development remains independent; no generic OS process manager appears | full regression + API/diff review |

Final gate: all packages in `go list ./...` covered by Go tests (package-by-package execution is used when the connector interrupts an aggregate call), `go vet ./...`, `git diff --check`, repeated real process lifecycle acceptance. Latest completed results: `06-VERIFICATION.md` and `evidence/regression.json`.

No LLM verifier/subagent. Deterministic local tests and task-owned Gateway only.
