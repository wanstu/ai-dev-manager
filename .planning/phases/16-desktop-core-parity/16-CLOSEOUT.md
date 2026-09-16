# Phase 16 Closeout — Desktop Core Parity + 1.0 RC Readiness

Status: closed on 2026-09-10 by user acceptance after RC5 release automation work.

## Result

Phase 16 moved ADM from Core-only capability validation into a usable Desktop RC track. Desktop now manages the validated Core surfaces without adding Desktop-only product state or authorization semantics.

## Landed implementation

- GitHub Actions CI and Desktop RC build baseline.
- Desktop MCP/Skill visual management UI.
- Agent/Admin MCP surface split.
- Desktop and normal CLI convergence on Admin MCP for management operations.
- Configurable ADM connection profiles.
- Gateway Base URL / selected ADM target handling.
- Windows path canonicalization for short/long paths.
- Desktop Runtime/process/run/verifier visibility.
- Tray, hidden start and HKCU launch-at-login support.
- Saved connections modal editors and Desktop icon pipeline.
- ime-lock-v2-informed tray lifecycle and fitted tray icon sizing.
- User-facing names simplified to `adm` and `adm-desktop`.
- Unified `dist/` packaging and tag-aware artifact names.
- Windows CI path-equivalence test fixes.
- Tray event loop fixed by keeping the systray Win32 window/message loop on one OS thread.
- Tag-triggered GitHub Release automation with checksums.

## Key commits

- `6d51e6d` — CI/RC baseline.
- `5a96fe4` — RC baseline closeout.
- `374fa12` / `74be256` — Desktop adapter APIs and MCP/Skill visual UI.
- `2c6ab1c` — Agent/Admin MCP split.
- `cd191ff` / `0dfe01d` / `ecf0516` — Desktop/CLI Admin MCP convergence.
- `11c49ee` — Windows path canonicalization.
- `53300d8` — Desktop tray/autostart/UI polish.
- `41a161a` — saved connections, modal editors and icons.
- `abafcc0` — `adm` / `adm-desktop` packaging/docs/tray sizing polish.
- `73c4481` — Windows canonical path CI test compatibility.
- `5c8574e` — tray event loop OS-thread fix.
- `559aa3d` — tag-triggered Release automation.

## Validation

- Full local `go test -count=1 ./...` passed after canonical-path CI fixes.
- Full local `go vet ./...` passed.
- `git diff --check` / staged diff checks passed for submitted changes.
- Wails Desktop build produced `dist\adm-desktop-windows-amd64.exe` after the tray event fix.
- User accepted the fixed tray behavior well enough to close Phase 16.
- User reported GitHub Actions green before Phase 16 closure.

## Remaining post-Phase-16 work

- Continue Phase 14 from `14-02`: optional code intelligence provider + GitNexus integration boundary.
- Keep Phase 15 temporary resource lifecycle for post-RC unless it becomes a daily-use blocker.
- Treat Phase 17 as conditional distribution polish; Release automation exists, but installer/updater/signing/notifications remain separate future work.
- Do not reopen Phase 16 for broad Desktop expansion; only fix regressions against the RC behavior if they appear.
