# Dogfood 01 — Non-Git development loop

Date: 2026-09-06

Status: passed. The non-Git development loop and Gateway process restart/reconnect persistence check are both verified.

## Goal

Exercise ADM V2 through its Agent Gateway against a real local project directory that is not a Git repository. This is the first concrete check of the V2 milestone gate in `docs/PRODUCT_CONTRACT.md`.

## Project

- path: `D:\projects\adm-v2-dogfood-nongit`
- Git repository: no
- language/toolchain: Go
- task: extend `NormalizeName` so it collapses internal whitespace, add tests, use the failing test result to make a follow-up implementation change, and exercise file deletion.

## Observed capabilities

`environment_inspect` advertised:

- `files.tree`
- `files.read`
- `search.text`
- `files.write`
- `files.edit`
- `files.delete`
- `shell.exec`

No `git.*` capability was advertised.

## Development loop exercised

1. Registered the ordinary non-Git directory as a Workspace.
2. Created an Environment rooted directly at that Workspace.
3. Listed the project tree.
4. Read `normalize.go`.
5. Searched for `NormalizeName`.
6. Acquired the Environment writer lease.
7. Wrote `normalize_test.go` with a requirement that repeated spaces/tabs collapse to one space.
8. Ran `go test ./...` through ADM. The first run failed because `NormalizeName("Alice   Smith")` returned `"alice   smith"` instead of `"alice smith"`.
9. Used exact edit to change the implementation from `TrimSpace` to `strings.Fields` + `strings.Join`.
10. Ran `go test ./...` again. It passed.
11. Created and deleted a temporary file through ADM to exercise file deletion.
12. Invoked Git status. It failed locally with `git.status is unsupported for this environment root`.
13. Immediately read `normalize.go` successfully after the Git failure, proving the missing Git capability did not make unrelated tools unavailable.
14. Released the writer and re-inspected the Environment successfully.

## Result

Passed:

- Workspace admission without Git
- Environment creation without Git
- tree/read/search
- write/edit/delete
- writer lifecycle
- allowlisted command execution
- test failure output returned to the Agent
- follow-up implementation change based on test output
- successful test rerun
- operation-local Git capability failure
- continued file access after Git failure
- persisted Environment state across separate Gateway calls
- actual HTTP Gateway process restart/reconnect persistence via `TestHTTPGatewayPersistsContextAcrossProcessRestart`

The restart test uses two real temporary Gateway child processes on a private port. The first process creates persisted context through MCP, is terminated, and a second process starts from the same state file. A fresh MCP client then successfully inspects the same Environment and reads its Environment-private Memory.

## Concrete blockers found

No core non-Git development blocker was found in this pass.
