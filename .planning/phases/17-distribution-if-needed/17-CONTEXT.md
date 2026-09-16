# Phase 17 Context — Distribution Only If Needed

## Status

Phase 17 is conditional post-RC work. It should stay closed to implementation unless manual dogfood proves a concrete distribution blocker that cannot be solved by existing local build scripts and docs.

Phase 16 remains closed. Do not reopen broad Desktop, CI, tray, icon, naming or release automation work as part of Phase 17 unless a new daily-use blocker provides direct evidence.

Phase 15 is complete. The immediate dogfood lesson is that source fixes do not help the active `pjadm` connection until a rebuilt Gateway/CLI is used; Phase 17 may therefore refresh local dogfood artifacts, but it must not imply public release, installer, updater, signing or notification work.

## Product boundary

ADM remains a local development control plane for external Agents. Distribution work can make the validated Core easier to run, but must not add Agent orchestration, GSD state advancement, automatic Memory composition, automatic push/tag/release, or Desktop-only authorization semantics.

## Allowed work without reopening scope

- Build current local CLI/Desktop artifacts using existing scripts.
- Verify the artifacts can start and point at a local ADM Gateway.
- Record exact commands, outputs and limitations for manual dogfood.
- Update docs/checklists that describe existing behavior.

## Blocked unless concrete dogfood evidence exists

- Installer generation.
- Auto-updater implementation.
- Code signing or certificate workflow.
- Notification subsystem.
- Remote/non-loopback Admin MCP enablement.
- Release publication, tag creation or GitHub Release mutation.

## Acceptance posture

A successful Phase 17 slice may conclude that no new distribution implementation is needed. That is an acceptable outcome when current local artifacts and docs are enough for daily dogfood.
