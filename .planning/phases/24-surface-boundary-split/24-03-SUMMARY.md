# 24-03 Summary — Desktop adapter separation + integrated closeout

Date: 2026-09-15
Predecessor: 24-02 complete at `810d6bc`
Implementation commit: `9a4fea0` (`refactor(desktop): separate management surface boundaries`)
Status: complete and validated; Phase 24 ready to close.

## Delivered

Phase 24-03 separates Desktop presentation responsibilities into focused source files while preserving the existing Wails/frontend method contract, shared Core/application authority, Admin MCP production path and release names.

### Desktop responsibility split

`internal/desktop/adapter.go` now owns only the `Adapter` shape, constructors and readiness guards. Existing behavior was moved without semantic redesign into:

- `dto.go` — Wails-facing input/status DTOs and their JSON tags;
- `connection.go` — ADM connection inspection/connect/disconnect, local Gateway bootstrap/lifecycle and shared target projection;
- `management.go` — Workspace/Environment/exec/MCP/Skill/Memory management facade methods;
- `runtime.go` — verifier/process/run runtime facade methods;
- existing `profiles.go` — connection profile persistence/preferences remains independently owned.

Exported Wails method names, DTO JSON fields, frontend calls and management/runtime backend contracts remain unchanged.

### Production Admin-MCP-only behavior preserved

`NewClientAdapter()` remains the production constructor. A successful compatible ADM connection installs one `*adminmcp.Client` as both management and runtime backend. Disconnect, incompatible endpoint and connection failure clear production backends rather than falling back to writable local state.

`NewAdapter(*management.Service)` remains the explicit test/offline/recovery-oriented constructor only. The frontend still talks to Wails bindings and did not gain direct `/admin/mcp` networking.

### Cross-surface dependency assertions

`internal/desktop/surface_boundary_test.go` adds source/build assertions covering the Phase-24 structural contract:

- Desktop does not import `internal/cli` and CLI does not import `internal/desktop`;
- Core/application/Gateway/Admin-MCP packages do not import presentation packages;
- shared Gateway target semantics remain the source for Desktop connection/lifecycle decisions;
- production Desktop uses the shared Admin MCP client;
- CI/build scripts retain the `adm` and `adm-desktop` release naming contract.

The existing 24-02 CLI boundary tests continue to guard the opposite dependency direction.

### Acceptance fixture budget correction

During full-repository acceptance, `TestVerifierRealHTTPAcceptanceNonGitRepositoryCopy` timed out because its test-only verifier definition hard-coded a 180-second budget for an inner `go test ./...`. After the Phase-24 CLI package extraction increased the repository test surface, the inner suite reached 181.185 seconds and was cut off by that fixture budget.

The fixture timeout was raised from 180 to 300 seconds. This changes no production Runtime/verifier default or product timeout semantics; it only restores sufficient acceptance-test budget. The isolated real HTTP verifier acceptance then passed in 148.313 seconds, and the final full repository suite passed with Gateway at 219.887 seconds.

A Windows process-cancellation timing test also failed once during the heavily contended full-suite attempt; the exact test passed three consecutive isolated runs and the final non-contended full suite passed, so no product code change was made for that transient timing failure.

## S21–S26 acceptance

| Gate | Result |
|---|---|
| S21 | PASS — Desktop DTO, connection/bootstrap, management, runtime and profile responsibilities are visibly separated while Wails/frontend contracts stay stable. |
| S22 | PASS — production Desktop and normal CLI both remain clients of the same Admin MCP/Core/state authority with no writable local fallback. |
| S23 | PASS — source-level boundary assertions prevent CLI↔Desktop imports and Core/Admin-MCP/Gateway→presentation imports. |
| S24 | PASS — focused Go, Phase-23 CLI repeat, Desktop helpers/browser smoke, full repository, vet and builds are green on the implementation tree committed as `9a4fea0`. |
| S25 | PASS — standard release/build names remain `adm` and `adm-desktop`; no repository/module split occurred. |
| S26 | PASS — Phase 24 remains refactor-only from the product perspective; no business, persistence, auth/TLS, orchestration or distribution semantics changed. |

## Validation evidence

- Focused surface/Gateway run `run_5ce0d3ef6e16b33a` — PASS; `internal/gateway` 199.827s.
- Phase-23 high-risk CLI acceptance repeat x3 — PASS.
- Desktop JS helper regression — 20/20 PASS.
- Production browser smoke `run_b41fb7165ab5bf09` — PASS at 1120x760@100%, 820x560@100%, 1120x760@125%; 198 checks each.
- Isolated Gateway verifier acceptance `run_efe702b18143f636` after fixture-budget correction — PASS, 148.313s.
- Windows cancellation focused repeat x3 — PASS.
- Final non-contended full repository `run_4d6ad47c00baa399`: `go test -count=1 ./...` — PASS; `internal/gateway` 219.887s and all packages green.
- `go vet ./...` — PASS.
- `git diff --check` and staged `git diff --cached --check` — PASS.
- Fixed-head CLI release build: `go build -trimpath -o dist/adm-windows-amd64.exe ./cmd/ai-dev-manager` — PASS.
- Fixed-head Wails production build: `scripts/build-desktop.ps1 -clean -trimpath` — PASS.

Artifacts built from clean implementation commit `9a4fea0`:

- `dist/adm-windows-amd64.exe` — 17,154,048 bytes — SHA-256 `2BA627A588A41C337AA63A0B3E0182290F55EE609941E8D056D7DA7810401D40`.
- `dist/adm-desktop-windows-amd64.exe` — 17,916,416 bytes — SHA-256 `32653763CFC156B80AEE30A477635E86389505BFECC218C89A25D8A54AFCD42B`.

Native GUI click-through was not newly performed in this node. Existing Phase-19/22 native-evidence availability notes remain unchanged; browser/Wails build evidence is not misrepresented as native GUI-control evidence.

## Boundary notes

No Desktop-only state/cache/protocol, CLI/Desktop cross-import, direct frontend Admin-MCP fetch path, writable local fallback, model duplication, auth/TLS change, installer/updater/signing work, task/GSD orchestration, push/tag/release or subagents were introduced.

Phase 24 can now be closed. Phase 25 remains conditional/Standby and is not opened by this closeout.
