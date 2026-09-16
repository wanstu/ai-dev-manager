# AI Dev Manager V2 — v1.0.0-rc.2

## Why RC2 exists

RC1 dogfood found two release-blocking defects that automated `go build` smoke checks did not catch:

1. The RC1 Desktop executable was built with raw `go build`. Wails applications require Wails build tags, so that artifact launched an error dialog instead of the real Desktop UI.
2. CLI management already supported `--adm-url` / `ADM_V2_URL`, but `gateway status/start/stop/restart` still defaulted independently to `127.0.0.1:43137`. A Gateway intentionally running on another port such as `8001` therefore looked invisible unless `--listen` was repeated.

RC2 fixes both defects and tightens the release gate so the same failures cannot be reintroduced silently.

## Desktop build fix

- Runnable Desktop artifacts are built only through Wails v2.15.0 (`scripts/build-desktop.ps1` / `wails build`).
- GitHub Actions now uses the Wails build path for the Windows Desktop artifact.
- `scripts/build-rc.ps1 -Version v1.0.0-rc.2` builds the versioned CLI, builds the Desktop through Wails, copies the real Wails executable into `dist/`, and writes SHA-256 checksums using .NET APIs that also work on older Windows PowerShell.
- README and the Phase 16 RC gate explicitly state that raw `go build ./cmd/ai-dev-manager-desktop` is only a compile check and is not a runnable/releasable Desktop build.

Observed local smoke before RC2 cut: a Wails-built Desktop process remained running after 3 seconds and did not show the RC1 build-tags error.

## Gateway target fix

Gateway lifecycle now participates in the same ADM connection target used by normal CLI management:

- `--adm-url URL` can appear before or after the `gateway` command.
- `ADM_V2_URL` is used when `--adm-url` is absent.
- explicit `--listen HOST:PORT` remains supported and has highest precedence.
- without `--listen`, local `gateway start/stop/restart` derive the listen address from the selected loopback HTTP ADM Base URL.
- `gateway status` can inspect the selected custom-port or remote Base URL.
- start/stop/restart refuse non-loopback/HTTPS/base-path targets; remote lifecycle mutation is not introduced by RC2.

A live local Gateway already running on `http://127.0.0.1:8001` was successfully observed with:

`ai-dev-manager-v2 gateway status --adm-url http://127.0.0.1:8001`

The command reported the `8001` MCP URL and the real process/runtime-owner information rather than falling back to `43137`.

### Important target behavior

ADM does not scan arbitrary local ports to guess which Gateway the user previously started. If no target is supplied, the default remains `http://127.0.0.1:43137`. For another port, use one of:

- `--adm-url http://127.0.0.1:8001`
- `ADM_V2_URL=http://127.0.0.1:8001`
- `gateway status --listen 127.0.0.1:8001`

This avoids ambiguous discovery when several ADM instances are intentionally running.

## MCP / Skill UI dogfood polish

A real RC2 Wails session exposed a frontend presentation defect that static Go checks could not reveal: the `details > summary` marker CSS contained a damaged quoted character, causing the browser to discard the following MCP/Skill management rules. That made forms look like mostly default HTML and collapsed status badges into unreadable text.

RC2 now also includes the screenshot-driven UI correction at `a3478f2`:

- repaired the broken CSS marker using an ASCII-safe CSS unicode escape;
- MCP add/import and Skill source forms start collapsed so the normal management list stays primary;
- successful add/import/source flows collapse again after completion;
- MCP and Skill lists have search, state/scope filters and visible/total counts;
- long MCP/Skill lists use a bounded scroll region instead of stretching the whole page;
- resource rows, badges, check controls and action groups have explicit spacing/density styling;
- `Sources = 0` with legacy Skills is explained explicitly rather than implying contradictory discovery state;
- Windows path placeholders display normal single backslashes.

## RC boundaries unchanged

- Agent MCP remains `/mcp`; administrator/Desktop/CLI management remains `/admin/mcp`.
- Desktop and normal CLI management do not silently fall back to writable local state.
- authenticated non-loopback Admin MCP mutation remains deferred to 16-03E.
- no Planner/Executor/Reviewer or GSD orchestration semantics are reintroduced.
- installer, signing, updater, tray/autostart and broader distribution remain post-RC unless dogfood proves blocking.

## Final RC2 validation

- `go test -count=1 ./...` passed after the UI dogfood fixes; Gateway suite completed in 75.794s.
- `go vet ./...` passed.
- `git diff --check` passed with only Windows line-ending warnings.
- `scripts/build-rc.ps1 -Version v1.0.0-rc.2` successfully built both versioned artifacts; Desktop was built by Wails v2.15.0.
- The exact `dist/ai-dev-manager-v2-desktop-v1.0.0-rc.2-windows-amd64.exe` stayed running after a 3-second launch smoke and did not show the Wails build-tags error.
- The exact RC2 CLI successfully inspected the pre-existing `http://127.0.0.1:8001` Gateway.
- The exact RC2 CLI completed a separate `start --detach` → `status` → `stop` lifecycle on `http://127.0.0.1:48002`, reporting Gateway version `v1.0.0-rc.2`.

SHA-256:

- `9d1fe8aa914c4ae6f9ac96f45fbbafcb2c9aae8d75b8e102dd4c585c40f41b6b` — `ai-dev-manager-v2-v1.0.0-rc.2-windows-amd64.exe`
- `4d112f6f2c16ffaa00d905b76017f9b6b44ff992c2704a239d0fe83e82d7c975` — `ai-dev-manager-v2-desktop-v1.0.0-rc.2-windows-amd64.exe`
