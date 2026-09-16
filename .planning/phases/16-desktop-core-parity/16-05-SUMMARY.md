# 16-05 — Saved ADM connections, modal editors and Desktop icons

Status: implementation and automated local gates complete at `41a161a`; selected executable smoke also passed. Human Windows tray/menu/autostart interaction remains open. No tag, push or publication was performed.

## Changes

- Desktop now manages multiple saved ADM connections with stable ID, display name, Base URL and active selection in `~/.config/adm/desktop-connections.json`.
- Connection writes are atomic within the config directory; invalid or secret-bearing URLs are rejected and corrupt existing configuration is not silently replaced.
- Switching connections waits for queued/in-flight Desktop adapter calls, disconnects the prior Admin MCP backend, clears stale management/runtime/health state and then connects the selected profile.
- MCP add/edit no longer expands inline or calls `scrollIntoView`; MCP import, Skill source, Workspace, Environment, executable, Global/Environment Memory and rename flows use modal dialogs as well. Errors remain visible in the active dialog and failed saves retain drafts.
- Added ADM icon sources under `assets/icons`. `ai-dev-manager-app.png` is the canonical Wails/Windows application icon source; `ai-dev-manager-tray.png` is converted to the embedded multi-size Windows tray ICO; `ai-dev-manager-window.png` is embedded as the Desktop header brand mark.
- `scripts/build-desktop.ps1` now refreshes Wails `build/appicon.png` from the committed app icon and removes the ignored generated `build/windows/icon.ico` before Wails packaging so stale icon resources cannot survive a rebuild.

## Evidence

- Targeted `go test -count=1 ./cmd/ai-dev-manager-desktop ./internal/desktop`: exit 0.
- Full `go test -count=1 ./...`: run `run_42669752b7d56bf5`, exit 0, completed 2026-09-10T12:21:01Z.
- `go vet ./...`: run `run_922e35b9c6fe0023`, exit 0.
- `git diff --check` and staged `git diff --cached --check`: exit 0; only expected autocrlf conversion notices occurred before staging.
- Final Wails build: run `run_1c1bc8da86f93595`, exit 0, using `scripts/build-desktop.ps1 -clean -trimpath -o ai-dev-manager-v2-desktop-windows-amd64.exe`.
- Artifact: `cmd/ai-dev-manager-desktop/build/bin/ai-dev-manager-v2-desktop-windows-amd64.exe`, size 17,007,104 bytes.
- App icon source SHA-256 `FEAFA40007D286C89E0A66088621EBF4EEB270AB5B614E7E3299A1FCA872FA02` exactly matched generated `build/appicon.png`.
- Generated `build/windows/icon.ico` contains 256/128/64/48/32/16 px 32-bit PNG icon entries. `Icon.ExtractAssociatedIcon` returned a 32x32 icon from the final exe, confirming a Windows icon resource is present.
- Embedded tray ICO contains 256/128/64/48/32/16 px entries and is built from `ai-dev-manager-tray.png`.
- Hidden launch/single-instance smoke: first `--autostart` process PID 25312 remained alive; second launch PID 33456 exited and only PID 25312 remained, confirming the Wails single-instance path did not create a second resident Desktop process. The smoke process was then stopped explicitly for cleanup.
- `D:\projects\ime-lock-v2` still cannot be registered through this pjadm session: `workspace_add` returns `Tool not available`. The icon implementation therefore follows the validated Wails asset/resource split and must not be described as a line-by-line port from ime-lock-v2.

## Remaining acceptance

Manually click tray Show/Hide/Quit; toggle launch-at-login from both Desktop and tray and verify HKCU Run add/remove; verify an actual Windows login starts hidden; visually inspect the modal layouts and brand icon at real display scaling; and confirm only one tray icon is visible during second-instance activation. Remote GitHub Actions is still unverified for the local post-RC3 commits. No tag, push or release is authorized by this summary.
