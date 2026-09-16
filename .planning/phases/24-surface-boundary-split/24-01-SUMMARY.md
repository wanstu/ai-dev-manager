# 24-01 Summary — Shared ADM target and management boundary cleanup

Date: 2026-09-15
Planning baseline: `2b14a57` (`docs: add comprehensive v1.1 user guides`)
Implementation commit: `a96558c` (`refactor(surfaces): unify ADM connection target boundaries`)
Status: complete and validated; 24-02 ready, not started by this execution node.

## Delivered

Phase 24-01 removes duplicated ADM connection-target semantics from the CLI and Desktop while preserving the existing Admin-MCP-only normal management boundary and all product behavior.

### Shared Gateway target semantics

`internal/gateway` now exposes one canonical `HTTPTarget` resolution path for:

- normalized HTTP/HTTPS ADM Base URL;
- `/healthz`, `/mcp` and `/admin/mcp` endpoint derivation;
- loopback HTTP root lifecycle eligibility;
- conversion of an eligible Base URL to local `host:port` listen form.

Remote HTTPS and base-path targets remain valid for inspection/management connection semantics where supported, but are not eligible for local bootstrap/stop. Local lifecycle still requires an explicit loopback HTTP root with a port. Userinfo, query strings (including an empty forced query) and fragments are rejected centrally.

### CLI convergence

`cmd/ai-dev-manager/admin_connection.go` retains only CLI presentation/configuration concerns such as `--adm-url`, `ADM_V2_URL` and duplicate/missing flag errors. URL normalization, endpoint derivation and local lifecycle checks now delegate to `internal/gateway`.

Production normal CLI management still constructs `*adminmcp.Client` against the shared canonical Admin MCP endpoint. Existing tests continue to prove that connection failure never falls back to writable local `state.json` management.

### Desktop convergence

Desktop connection inspection, connection-profile validation and local bootstrap eligibility now consume the same Gateway target helpers as the CLI. Profile persistence format is unchanged.

`NewClientAdapter` still installs `*adminmcp.Client` only after a running compatible ADM connection. Stopped/disconnected state clears both management and runtime backends. The explicit `NewAdapter(management.Service)` constructor remains test/offline-oriented and is not an automatic production fallback.

### Boundary assertions

Focused acceptance now asserts:

- CLI and Desktop target behavior agrees with the shared Gateway target implementation for loopback IPv4, localhost, IPv6, HTTPS and remote base-path examples;
- custom local ports remain valid;
- `*adminmcp.Client` satisfies the surface-local production interfaces;
- production Desktop installs `*adminmcp.Client` and clears it after disconnect;
- Desktop frontend JavaScript contains no direct `fetch`, `XMLHttpRequest` or `WebSocket` management path and continues to use Wails bindings;
- CLI base-path Admin MCP management remains functional through the canonical derived endpoint.

## S01–S06 acceptance

| Gate | Result |
|---|---|
| S01 | PASS — CLI/Desktop share canonical Base URL, health, Agent MCP and Admin MCP derivation. |
| S02 | PASS — only loopback HTTP root targets are local-lifecycle eligible; HTTPS, remote and base-path targets are inspection/connection-only. |
| S03 | PASS — localhost/IPv4/IPv6 custom-port lifecycle conversion remains valid. |
| S04 | PASS — production Desktop clears Admin MCP backends on disconnect; normal CLI no-fallback acceptance remains green. |
| S05 | PASS — diff is limited to connection/lifecycle surface boundary code/tests; no Workspace/Environment/MCP/Skill/Memory/retention/Runtime product model changes. |
| S06 | PASS — no user-facing command names, flags or release names changed. |

## Validation

- Focused Gateway target tests — PASS.
- `go test -count=1 ./cmd/ai-dev-manager ./internal/desktop ./internal/adminmcp` — PASS.
- Full `internal/gateway` package — PASS, `254.041s` before final private-normalizer tightening.
- Final shared-target focused Gateway rerun after tightening — PASS.
- Full repository `run_260c45509794c0a8`: `go test -count=1 ./...` — PASS; Gateway `257.913s`, all packages green.
- `go vet ./...` — PASS.
- `git diff --check` — PASS.
- `gofmt` applied to all changed Go files.

## Boundary notes

No second management protocol, writable fallback, persisted connection-target model, remote auth/TLS behavior, product model change, UI redesign, source/module/repository rename, task/GSD orchestration, Phase-25 work, push/tag/release or subagents were introduced.

24-02 may now extract CLI implementation into a dedicated internal surface package behind a thin executable bootstrap while preserving all existing command grammar, output/error contracts and explicit local recovery exceptions.
