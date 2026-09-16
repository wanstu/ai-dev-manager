---
phase: 04-external-agent-dogfood-gate
status: passed
nyquist_compliant: true
wave_0_complete: true
---
# Phase 4 Validation

| ID | Acceptance | Evidence |
|---|---|---|
| D1 | Newest source build advertises Skill/verifier/MCP health; separate private state and port | build output, tools/list |
| D2 | Enabled real GSD Skill and support workflow read through ADM; disabled read denied; no copied installation | Skill responses and smart-entry result |
| D3 | Non-Git target read/search/write/edit; tests first fail, then pass via structured verifier | red.json, green.json, source snapshot |
| D4 | CLI consumes those real results and returns failure/pass; incomplete fields/trailing JSON rejected | focused tests and live CLI exits |
| D5 | No verifier config allows files; no Git error remains local; wrong writer rejected | negative responses |
| D6 | Private Memory and Skill/verifier selection survive task Gateway stop/start; separate Environment cannot read private value | pre/post restart responses |
| D7 | External upstream MCP conditional | N/A: none used |
| D8 | R1 review boundary; no Desktop/runtime feature expansion | diff, STATE and summary |

Regression: go test ./..., go vet ./..., git diff --check through ADM. Known Windows startup timeout: record any recurrence and focused retry; do not alter timeout runtime without stable evidence.
Target: go test ./... and go vet ./... as structured verifiers, bounded output and 120-second task-specific timeout. CLI unit tests cover real pass/fail/timeout, skipped, malformed/missing fields, contradictory pass, trailing JSON.
All project operations route through ADM; PowerShell may transport calls to ADM itself and invoke the existing CLI. No other filesystem/exec MCP, no LLM subprocess.

D9 blocker regression: TestWindowsCommandCancellationStopsChild failed before the Windows cancellation fix, then passed; unchanged TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy passed in full regression. Results retained in evidence/regression.json. No LLM gate execution is claimed.
