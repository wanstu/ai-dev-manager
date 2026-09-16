# AI Dev Manager V2 — v1.0.0-rc.1

## RC scope

`v1.0.0-rc.1` is the first local Windows release candidate for daily ADM dogfood. It is intentionally not a public installer/signing/updater release.

## Included

- Agent MCP at `/mcp` and privileged Admin MCP at `/admin/mcp` over the same Core/runtime owner.
- Desktop connection profile with explicit ADM Base URL, `/healthz` liveness, Agent/Admin MCP endpoint display, and loopback-only local bootstrap.
- Normal Desktop management through Admin MCP only; disconnected ADM does not fall back to writable local state.
- Normal CLI workspace/environment/exec/MCP/Skill/Memory management through Admin MCP; `--adm-url` and `ADM_V2_URL` select the target and do not fall back to local state.
- Desktop MCP visual management: typed HTTP/stdio add, import preview/apply, defaults, Environment selection, explicit health probe and diagnostic state.
- Desktop Skill visual management: source add/list/refresh/remove, Environment selection and availability diagnostics.
- Desktop owner-local Runtime visibility for verifier definitions/results, process state/logs/ports/stop and generic run state/output/cancel. Runtime actions use an already-active Environment writer and never acquire one automatically.
- Default local Gateway moved to `127.0.0.1:43137`; local start now performs bind preflight so reserved/occupied Windows ports fail immediately with a useful diagnostic.
- GitHub Actions RC build baseline for test/vet and Windows/Desktop plus cross-platform CLI artifacts.

## Local validation evidence

Before the RC marker:

- Final `go test -count=1 ./...` passed after the RC1 version marker; Gateway package completed in 61.226s.
- `go vet ./...` passed.
- Windows CLI smoke build passed.
- Windows Desktop smoke build passed.
- `git diff --check` passed with only Windows LF/CRLF warnings.
- Real built CLI default `gateway start --detach` succeeded on `127.0.0.1:43137`, `/healthz` returned a compatible ADM response, default CLI Admin MCP management loaded real Workspace state, and `gateway stop` shut it down cleanly.
- Real built Desktop executable remained running during launch smoke and was then explicitly stopped by the test session.
- Embedded frontend contract tests cover the MCP/Skill/Admin connection and Runtime UI bindings.

## Known limitations

- A real human click-through of the Wails window is still required during RC dogfood. The current automation environment can launch/observe the Desktop process but cannot interact with the native GUI. Any RC-blocking interaction defect found during dogfood should produce `rc.2` rather than expanding RC1 scope in place.
- Non-loopback remote Admin MCP is not enabled as a supported secure deployment mode yet. Authentication/TLS/reverse-proxy and credential-reference semantics remain 16-03E/post-RC work.
- Installer, tray, autostart, updater, signing and notifications are post-RC distribution work.
- Runtime observations are owner-local and are not durable across Gateway restart.
- Desktop intentionally does not expose arbitrary command/process/run start controls in RC1; execution remains Agent/CLI/Gateway territory.
- GitHub Actions workflow exists, but no remote GitHub CI run is claimed here because this local release process has not pushed `master`.

## Safety / architecture boundaries

- Direct writable `state.json` access is not a peer normal management mode. It is limited to explicit local bootstrap/offline/recovery paths.
- Desktop does not expose resolved secret-backed MCP values in normal views.
- Private Memory values are loaded only through explicit user actions.
- Desktop does not bypass Environment selection, exec allowlist, writer lease or runtime-owner rules.
- ADM remains a capability/control plane; planning/orchestration remains outside ADM Core.
