# 16-06 — ime-lock-v2 alignment, tray sizing, packaging and docs polish

Status: implementation and automated local gates complete at `abafcc0`. No tag, push or publication was performed.

## Changes

- Compared ADM Desktop against `https://github.com/wanstu/ime-lock-v2` reference clone `.tmp/ime-lock-v2-reference` at commit `43f9a19d0256d47acd561244b866f44d360633ed` (`v0.1.4-rc1`).
- Replaced the prior tray integration with `github.com/gogpu/systray` and an independent tray object/message-loop model, started after Wails `OnDomReady`.
- Kept Wails v2 `SingleInstanceLock` instead of adopting ime-lock-v2's lock-file/wake-file path, because the official lock is already implemented and has real-exe smoke coverage.
- Fixed the visibly-small tray icon: the original `assets/icons/ai-dev-manager-tray.png` visible alpha bounds filled only 65.4% of the canvas; `scripts/prepare-desktop-icons.ps1` now creates `cmd/ai-dev-manager-desktop/assets/tray.png` with 94% visible fill for Go embedding.
- Simplified user-facing names to `adm` and `adm-desktop` without changing the Go module path `ai-dev-manager-v2`.
- Updated CLI help, Gateway health/server naming, Admin MCP client implementation name, Wails app name/output filename, Desktop title, tray tooltip and Windows launch-at-login value to the simplified names.
- Kept `ai-dev-manager-v2.exe` as a recognized legacy sibling executable during Windows Gateway stop/restart migration.
- Standardized package outputs in `dist/`: default Desktop build copies `dist/adm-desktop-windows-amd64.exe`; RC builds write `adm-$Version-windows-amd64.exe`, `adm-desktop-$Version-windows-amd64.exe` and `SHA256SUMS-$Version.txt`.
- Updated GitHub Actions to build on `v*` tags and include `${{ github.ref_name }}` in CI artifact names and filenames for tag builds.
- Slimmed README to quick-start/index content with clickable table of contents, and added `docs/index.md` plus topic docs for quickstart, Desktop, CLI, catalog/memory and packaging.

## Evidence

- Focused tests after rename compatibility fix: `go test -count=1 ./cmd/ai-dev-manager ./internal/gateway`, run `run_b601539dc3610a22`, exit 0.
- Focused Desktop tests: `go test -count=1 ./cmd/ai-dev-manager-desktop ./internal/desktop`, exit 0.
- Full repository tests: `go test -count=1 ./...`, run `run_ce76b7e08468ed7f`, exit 0.
- `go vet ./...`, run `run_12b298a2818ffd2b`, exit 0.
- `git diff --check`, run `run_00c323d99f51baeb`, exit 0; only expected autocrlf notices were emitted.
- Default Wails build succeeded with `scripts/build-desktop.ps1 -clean -trimpath`; output was `dist\adm-desktop-windows-amd64.exe` and Wails built `cmd\ai-dev-manager-desktop\build\bin\adm-desktop-windows-amd64.exe`.
- RC packaging smoke succeeded with `scripts/build-rc.ps1 -Version v1.0.0-rc.4`; outputs were `dist\adm-v1.0.0-rc.4-windows-amd64.exe` (16,104,448 bytes), `dist\adm-desktop-v1.0.0-rc.4-windows-amd64.exe` (16,925,184 bytes), and `dist\SHA256SUMS-v1.0.0-rc.4.txt`.
- SHA256SUMS for RC4 smoke: `6e3f280afbbd5a87b66c85b4eb77a113e101814e9004904b6a168bb543ceb85d  adm-v1.0.0-rc.4-windows-amd64.exe`; `373af5046a9f25346a5876072700bdd32b4d3a76e8499b913a70fe0420516be7  adm-desktop-v1.0.0-rc.4-windows-amd64.exe`.
- The accidental first commit attempt `70b287c` was reverted before publication because it contained line-ending noise across 221 files. The final functional commit is the cleaned `abafcc0` with 26 files changed.

## Remaining acceptance

Manual Windows acceptance is still required for tray Show/Hide/Quit clicks, launch-at-login registry toggle, real login hidden startup, final tray icon visual size at real DPI, modal layout review and visible single-tray-icon behavior. Remote GitHub Actions evidence is still absent until push is explicitly authorized and observed. Do not tag or publish from local evidence alone.
