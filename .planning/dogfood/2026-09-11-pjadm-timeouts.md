# Dogfood Note — pjadm Long Synchronous Command Timeouts

Date: 2026-09-11
Status: mitigation implemented; active `pjadm` Gateway pending restart/switch

## Observation

During Phase 17 follow-on validation, long synchronous `pjadm.exec` calls repeatedly timed out at the ChatGPT/tool boundary even when the ADM command `timeout_ms` was set much higher.

Observed examples:

- `go test -count=1 ./...` via synchronous `pjadm.exec` timed out before returning package-level results.
- `go test -count=1 ./internal/gateway` via synchronous `pjadm.exec` also timed out before returning results.
- Smaller package groups returned successfully, which suggests the issue is the synchronous request/response path or outer MCP/tool timeout rather than a simple test compile failure.

## Impact

This hurts daily dogfood because an Agent cannot reliably run natural long verification commands and get a clear terminal result. The workaround is to manually split commands into smaller groups or use asynchronous run lifecycle surfaces, but that workaround is easy to forget and makes failures ambiguous.

## Current workaround

- Prefer focused tests or small package groups for `pjadm.exec`.
- Prefer `run_start` + `run_status`/`run_list` for long-running commands when available.
- Keep `max_output_bytes` bounded to avoid returning huge payloads.

## Async path check

A `run_start` attempt for `go test -count=1 ./internal/gateway` returned a stable run id immediately, so async start avoids losing the ChatGPT turn to a synchronous outer timeout. However, while the run was still `running`, `run_status`/`run_list` returned only lifecycle metadata and no bounded live log/progress excerpt. The run was manually canceled to avoid leaving a long-running validation blocker active.

This means the async path is a useful workaround but not yet a complete dogfood answer for long verification, because the Agent still lacks an ergonomic running-output view and terminal result handoff inside normal chat cadence.

## Mitigation implemented

The Gateway Agent Run path now owns bounded stdout/stderr buffers directly while the command is running instead of waiting for `Runtime.Exec` to return. `run_status` can therefore expose output already produced by a still-running command, with truncation flags when `max_output_bytes` is reached. This preserves the existing writer, allowlist, cwd containment, timeout and cancellation model while making the async path useful for long verification dogfood.

Validation:

- `go test -count=1 ./internal/gateway -run TestAgentRun`
- `go test -count=1 ./internal/gateway -run TestGatewayCapabilityReport|TestGatewayResourceRetention|TestAgentRun|TestHTTPGatewaySeparatesAgentAndAdminMCPPaths|TestGatewayDevelopsPlainDirectoryWithoutGit`
- `go test -count=1 ./internal/app ./cmd/...`
- `go vet ./internal/gateway ./internal/app ./cmd/...`
- `git diff --check`

## Live connection and artifact refresh check

After `6b793d3`, the currently connected ChatGPT `pjadm` Gateway was probed with a real `run_start` command that wrote `adm-live-stream-probe` and then slept. Running `run_status` still returned only lifecycle metadata, while the terminal `run_cancel` result included stdout. That confirms the active `pjadm` process is still serving the older code path; it should be restarted or switched to a rebuilt binary before expecting running-output snapshots in this chat tool connection.

Fresh local dogfood artifacts containing the `6b793d3` fix were built with:

```text
scripts/build-rc.ps1 -Version v1.0.0-rc.local-runobs
```

Artifact checksums:

```text
8f900e80705edc8611c65f7d9ec7b0e33b16b47662f1c221dba48c8dc0987de0  adm-v1.0.0-rc.local-runobs-windows-amd64.exe
6171de994ceb9b652ef85a0236a258b5bcccc1b603f267a29586d27e27dac209  adm-desktop-v1.0.0-rc.local-runobs-windows-amd64.exe
```

## Candidate fix direction

Do not increase global timeouts blindly. Instead, make the long-command path explicit and inspectable:

1. Document or surface a clear threshold where synchronous `exec` is not appropriate.
2. Prefer an async run flow for long commands, with stable run IDs and bounded status/log excerpts.
3. Consider having synchronous `exec` fail early with an actionable diagnostic when requested timeout exceeds the likely client/tool deadline.
4. Preserve the existing safety model: writer lease required, allowlisted executable only, bounded output, no arbitrary shell escape.

## Acceptance idea

A future dogfood slice is successful when an Agent can run repository-wide verification without losing the result to outer tool timeout, either by a first-class async path or by an explicit diagnostic that tells the Agent to use that path.
