# Phase 16 RC Gate — Local 1.0 Release Candidate

## RC definition

A 1.0 RC is a local release candidate suitable for daily dogfood. It is not a public packaged release and does not require installer, tray, autostart, updater, signing or notifications.

## Required automated gate

Before declaring a local 1.0 RC:

1. GitHub Actions CI must exist and run on `master`, pull requests and manual dispatch.
2. CI must run full Windows `go test -count=1 ./...`.
3. CI must run `go vet ./...`.
4. CI must build the Windows CLI binary.
5. CI must build a runnable Windows Desktop artifact through Wails (`scripts/build-desktop.ps1` / `wails build`); raw `go build ./cmd/ai-dev-manager-desktop` is not a valid Desktop release artifact.
6. CI should build CLI artifacts for Windows/Linux/macOS when supported by normal `go build`.
7. CI must upload short-retention artifacts for smoke review.
8. CI must not publish releases, push commits, require secrets or deploy anything.

For a **local RC marker**, the workflow definition plus passing local test/vet/build gates is sufficient when no push has been requested. A real GitHub Actions pass is required before promoting the same candidate as a public/remote-validated RC.

## Required local gate

Before declaring RC locally:

1. `go test -count=1 ./...` passes.
2. `go vet ./...` passes.
3. `git diff --check` passes, allowing only platform line-ending warnings if there are no whitespace errors.
4. CLI smoke build passes.
5. Desktop Wails build passes and the resulting executable launches without the Wails build-tags error.
6. `git status` is clean after the RC commit.
7. No push unless explicitly requested.

## Desktop smoke checklist

Before RC:

1. Desktop launches.
2. It can connect to the selected ADM Base URL, validate `/healthz`, and load management data through `/admin/mcp`.
3. Stopped/unreachable ADM is shown as disconnected and does not expose stale/local-fallback management data.
4. Workspace and Environment list/inspect work.
5. Capability report is visible, including unavailable/degraded reasons.
6. MCP definitions, import/config basics, Environment selection and health/status are visually manageable in Desktop.
7. Skill sources, refresh, Environment selection and availability are visually manageable in Desktop.
8. Process/run/verifier state is visible enough for daily use.
9. Private Memory values and secret-backed MCP values are not displayed in normal views.
10. Known limitations are documented.

## RC blockers

A gap blocks RC only if it prevents safe daily use of validated Core capabilities through Desktop or makes the CI/artifact path unreliable.

Default RC blockers:

- Desktop cannot launch.
- Desktop cannot connect to a running local ADM and manage it through Admin MCP, or silently falls back to writable local state when disconnected.
- Normal CLI management silently bypasses Admin MCP or falls back to writable local state when the selected ADM is unavailable.
- Desktop leaks private Memory values or secret-backed MCP values.
- Desktop bypasses writer lease, executable allowlist or Environment selection rules.
- Capability diagnostics are absent from Desktop with no clear CLI/Gateway fallback.
- Desktop cannot manage MCP/Skill basics well enough for daily use: add/import/list, defaults, selected-Environment enablement and visible health/availability reasons.
- CI cannot run test/vet/build or cannot produce any usable binary artifact.

## Non-blocking by default

The following are useful but not RC blockers unless dogfood proves otherwise:

- GitNexus/provider integration.
- Additional Phase 14 investigation helpers.
- Temporary resource lifecycle cleanup.
- Installer/tray/autostart/updater/signing/notifications.
- Automatic release publishing.
- Authenticated/TLS-safe non-loopback Admin MCP remote exposure (16-03E).

## Known limitations policy

A known limitation is acceptable for RC when:

1. the Core capability remains available through CLI/Gateway;
2. Desktop clearly avoids unsafe or misleading behavior;
3. the limitation is documented in the RC notes;
4. there is a post-RC follow-up path.
