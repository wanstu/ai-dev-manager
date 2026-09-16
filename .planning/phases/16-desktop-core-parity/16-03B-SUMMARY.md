# Phase 16-03B Summary — Desktop ADM Connection Profiles

## Result

Completed configurable Desktop ADM connection profiles and health/liveness inspection.

Implementation commit: `cd191ff` — `feat(16-03b): add Desktop ADM connection profiles`

## Delivered

- Desktop accepts an explicit ADM Base URL instead of assuming only `127.0.0.1:43137`.
- Health inspection accepts normalized `http`/`https` Base URLs with optional base path.
- Connection status derives and displays:
  - `/healthz`
  - Agent `/mcp`
  - Admin `/admin/mcp`
- Desktop stores the selected Base URL as a local UI preference rather than ADM Core state.
- Local process bootstrap is restricted to loopback `http` root URLs with an explicit port.
- Remote/domain/HTTPS/base-path profiles cannot accidentally start or stop a local ADM process.
- Existing listen-based Gateway lifecycle remains compatible.
- Fixed the 16-02 frontend MCP badge reference to the previously undefined `config` variable.

## Safety boundary

16-03B adds configurable connection/liveness only. It does **not** enable unauthenticated non-loopback ADM serving. The Gateway remains loopback-only until 16-03E defines authentication/TLS/Host semantics.

## Validation

Passed before commit:

- focused Gateway/Desktop connection tests;
- `go test -count=1 ./...`;
- `go vet ./...`;
- CLI/Desktop Go build;
- `git diff --check` (Windows LF→CRLF warnings only).

`node` was not in the Environment executable allowlist, so no new allowlist permission was added merely for JavaScript syntax checking; Desktop frontend contract tests and Desktop build remained the available automated UI checks.
