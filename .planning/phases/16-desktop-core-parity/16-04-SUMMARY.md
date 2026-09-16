# 16-04 — Local verification, 2026-09-10

Status: code and automated local gates complete; interactive Windows acceptance remains open. No tag, push or publication is authorized by this session. Historical RC3 release-ready text in STATE is not evidence for these new changes.

## Changes

- Path repair committed separately as `11c49ee`: canonicalize existing path prefixes with Windows GetLongPathName, retain missing descendants, and use canonical comparisons across Workspace/Environment/Isolation/Runtime/Skill. Real GetShortPathName regression passed without skipping on this host.
- MCP/Skill UI wording, edit/delete health invalidation, reference visibility, Escape handling, source refresh labels.
- Windows tray show/hide/autostart/quit and Wails single-instance handling. Desktop login startup is opt-in HKCU Run and never starts/stops Gateway implicitly.
- Fixed defects found in this review: embedded icon moved from ignored build output to tracked assets/tray.ico; startup command preserves literal Windows path separators instead of Go %q escaping; unsupported platforms retain normal close behavior; tray/Desktop startup checkboxes synchronize through Wails events and write failures surface in a native dialog.
- Actual executable smoke caught runtime.EventsOn without a Wails context. External-loop tray initialization now happens in OnStartup after context setup and single-instance selection, instead of during constructor setup.

## Evidence

- Initial full test Run `run_e098f8855941ea57` failed because the non-Git copy excludes build/windows/icon.ico. Fixed the production asset location; did not weaken the copy acceptance test.
- Final `go test -count=1 ./...`: `run_34c65f6597cbf9a1`, exit 0, completed 2026-09-10T11:26:59Z, including non-Git HTTP verifier acceptance.
- Final `go vet ./...`: exit 0 after the lifecycle correction.
- `git diff --check`: exit 0; only autocrlf conversion notices were observed.
- Frontend app.js/index.html/styles.css: strict UTF-8 decoding succeeded, no U+FFFD replacement characters found; Chinese source strings read normally.
- Final Wails build: `run_e27613291209e15d`, exit 0, using scripts/build-desktop.ps1 -clean -trimpath -o ai-dev-manager-v2-desktop-windows-amd64.exe.
- Artifact: cmd/ai-dev-manager-desktop/build/bin/ai-dev-manager-v2-desktop-windows-amd64.exe.
- Artifact SHA-256: 62adceff2e1079b3b87bfab46d24f3e1239351b0d0e76bfe1bdb33aa44f625f7.
- Real final executable created WebView2 successfully. Smoke process 16832 displayed the expected Desktop title and responded. CloseMainWindow returned true, then the process remained alive with MainWindowHandle 0. The smoke process was subsequently stopped explicitly for cleanup; this does not prove tray Quit.
- A combined autostart/second-launch tool call did not return its detailed result. Do not count that as complete evidence for hidden initial startup or duplicate-icon prevention.

## Remaining acceptance

Use the exact artifact to click tray show/hide/quit, toggle login startup from both Desktop and tray and verify HKCU Run writes/removal, verify hidden --autostart and second-instance activation with one tray icon, and check actual Windows login behavior. UI visual polish still needs human click-through. No remote GitHub Actions result exists for these local commits. No direct comparison with ime-lock-v2 was performed because that reference workspace is not available through the current Environment.

Wails API references used for the event synchronization and native errors: https://wails.io/docs/reference/runtime/events/ and https://wails.io/docs/reference/runtime/dialog/ .
