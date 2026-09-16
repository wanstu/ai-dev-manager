# 24-02 Summary — CLI package extraction

Date: 2026-09-15
Predecessor: 24-01 complete at `a96558c`
Implementation commit: `810d6bc` (`refactor(cli): extract command surface package`)
Status: complete and validated; 24-03 ready, not started by this execution node.

## Delivered

Phase 24-02 moves the CLI presentation surface out of `cmd/ai-dev-manager` into `internal/cli` without changing command behavior, management authority or release naming.

### Internal CLI surface package

The existing CLI implementation now lives under `internal/cli` and owns:

- top-level command routing and help;
- command/flag parsing and JSON stdout behavior;
- MCP file/stdin import ergonomics;
- Admin MCP client construction and Phase-24 shared ADM target semantics;
- explicit local `gateway`, `doctor` and `state` recovery/bootstrap behavior;
- Windows/non-Windows detached Gateway process helpers;
- CLI-local narrow backend interfaces and the existing CLI behavior/acceptance tests.

The extraction is intentionally mechanical. Command names, flags, help text, JSON result shapes, error semantics and local recovery exceptions were retained rather than reimplemented.

### Thin executable bootstrap

`cmd/ai-dev-manager/main.go` is now process bootstrap only:

1. load executable-adjacent `.env`;
2. pass `os.Args[1:]` to `cli.Run`;
3. print the final `错误：` error line;
4. choose process exit status through `os.Exit(1)`.

An executable-level smoke test proves that the bootstrap delegates to the internal CLI surface and exposes the existing help contract.

### Dependency direction guard

`internal/cli/boundary_test.go` parses Go imports and enforces the Phase-24 direction:

- no other `internal/*` package imports `internal/cli`;
- Desktop executable/source does not import `internal/cli`;
- `internal/cli` does not import `internal/desktop`.

This keeps Admin MCP/Core/Gateway independent of the CLI presentation package and prevents CLI↔Desktop coupling.

## S11–S16 acceptance

| Gate | Result |
|---|---|
| S11 | PASS — `cmd/ai-dev-manager` contains only the thin bootstrap/smoke while command implementation/tests live in `internal/cli`. |
| S12 | PASS — the moved full CLI test suite and Phase-23 acceptance retain command/flag/help/JSON behavior. |
| S13 | PASS — normal management still uses the Admin MCP client and no-fallback tests remain green. |
| S14 | PASS — `gateway`/`doctor`/`state` implementation and OS-specific detached lifecycle moved intact with the CLI surface. |
| S15 | PASS — source-level dependency guard prevents Core/Desktop→CLI and CLI→Desktop imports. |
| S16 | PASS — no product model, authorization, persistence, MCP/Skill/Memory/Environment/Runtime semantics or release artifact names changed. |

## Validation

- `gofmt` — applied to moved/new Go files.
- `go test -count=1 ./internal/cli ./cmd/ai-dev-manager ./internal/adminmcp` — PASS.
- Phase-23 high-risk acceptance repeat: `go test -count=3 ./internal/cli -run ^TestPhase23` — PASS.
- Focused Gateway lifecycle/HTTP tests — PASS.
- CLI release build: `go build -o dist/adm-phase24-02.exe ./cmd/ai-dev-manager` — PASS; temporary artifact removed after the build check.
- Full repository `run_d3c74399dc98550f`: `go test -count=1 ./...` — PASS; `internal/gateway` 234.161s and all packages green.
- `go vet ./...` — PASS.
- `git diff --cached --check` — PASS before implementation commit.

## Boundary notes

No second CLI protocol, state/cache, writable fallback, hidden current Environment, MCP/Skill/Memory/Runtime semantic change, Desktop behavior change, module/repository split, distribution change, task/GSD orchestration, push/tag/release or subagents were introduced.

24-03 may now separate Desktop DTO/connection/management/runtime responsibilities into focused files while preserving the Wails/frontend contract, then run Phase-24 integrated fixed-head acceptance and closeout.
