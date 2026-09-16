# 22-03 Summary — Desktop visibility and Phase 22 integrated closeout

Date: 2026-09-13
Implementation baseline: `ac29983` (`docs: record Phase 22-02 gateway acceptance`)
Final implementation head: `56c79e3` (`feat(desktop): surface temporary environment lifecycle`)
Status: complete and validated; Phase 22 ready for docs closeout only.

## Delivered

- Added Admin-MCP client/Desktop adapter wrappers for `environment_temporary_status`, `environment_temporary_promote`, and `environment_temporary_cleanup`. Production Desktop continues to use the connected Admin MCP; there is no writable local-state fallback for temporary lifecycle actions.
- Environment list/detail now render retention explicitly:
  - `Durable`, `Temporary`, `Temporary · 已过期`, or `Retention 未知` when retention is absent;
  - lifecycle owner and optional session/run provenance for temporary Environments;
  - expiry and, after explicit lifecycle refresh/preview, managed-worktree vs existing-root context and cleanup blockers.
- Provenance remains informational lifecycle metadata only. It is not rendered as a task hierarchy, task status, execution parentage, or GSD state.
- Environment-private Memory values remain absent from list/detail lifecycle rendering. Existing private Memory continues to require explicit Memory-page loading.
- Added bounded temporary lifecycle controls to existing Environment management:
  - refresh lifecycle/status;
  - promote to durable using the persisted lifecycle owner;
  - cleanup preview;
  - cleanup execute only after the latest preview reports eligible and the user confirms a second destructive prompt.
- Cleanup UI exposes no force path. Blockers remain visible instead of becoming generic success/failure. Ordinary cleanup messaging states that ADM Environment state is removed without deleting project/root files; managed-worktree messaging states that the managed root is removed while its branch is retained.
- Lifecycle observation caches are presentation-only maps and are cleared on connection-scope reset. Calls capture `connectionGeneration`; late results from a prior ADM connection are discarded. No new Desktop state file, cache, authorization model, or persistence path was added.

## Desktop/Admin-MCP evidence

`internal/desktop/connection_test.go` proves the production-style Desktop client reaches temporary status and owner-scoped promotion through Admin MCP. Wrong-owner promotion is rejected and matching-owner promotion preserves stable Environment identity. The lightweight `gateway.NewHTTPHandler` used by that adapter test intentionally has no persistent Runtime owner, so its cleanup call correctly reports `persistent runtime owner is unavailable`; owner-aware cleanup semantics are covered by the Phase-22-02 real Gateway-owner Streamable HTTP suite rather than being faked in the Desktop test.

Production browser smoke covers:

- D01 — durable, temporary/expired, and unknown retention are distinguishable; missing retention is not rendered as durable certainty.
- D02 — expiry plus owner/session/run provenance is visible without exposing Environment-private Memory values.
- D03 — cleanup preview is non-destructive; an active writer is shown as `active_writer`, and no execute action appears while blocked.
- D04 — promote requires explicit confirmation, calls the Admin-MCP-backed Desktop method with the persisted lifecycle owner, preserves `env-a`, and refreshes it as durable.
- D05 — cleanup execute is unavailable until an eligible preview, requires explicit confirmation, removes only the selected temporary Environment, and leaves the unrelated Environment present.
- D06 — blocker text is rendered before destructive success; broader dirty/unpublished/runtime blocker semantics remain covered by the real Gateway acceptance from 22-02.
- D07 — there is no force cleanup control/action.
- D08 — lifecycle presentation state is cleared with the existing connection scope; stale lifecycle responses use `connectionGeneration` guards and there is no local writable fallback.
- D09 — production Chromium matrix passed at 1120x760, 820x560, and 1120x760 @ 125% scale.
- D10 — uniquely named Wails production artifact built successfully. Native Wails/WebView2 click-through is **PENDING by availability**, because this session has no native GUI-control tool; no PASS is claimed for native click-through.

## Fixed-head validation on `56c79e3`

- Focused lifecycle/regression gate: `run_97667f3e145341a7` — PASS.
  - `go test -count=1 ./internal/app ./internal/gateway ./internal/desktop ./cmd/ai-dev-manager-desktop -run "Temporary|ResourceRetention|Managed|VerifierRun|AgentRun|DevProcess|ClientAdapter"`
  - app 6.071s; Gateway 57.062s; Desktop 0.169s; Desktop cmd 0.043s.
- Full repository: `run_873a9a8b33dac10e` — PASS.
  - `go test -count=1 ./...`
  - Gateway 203.904s; all packages green.
- Vet: `run_05c2a8553a06917c` — PASS (`go vet ./...`).
- Production Chromium smoke: `run_a41c0cce2b26efe3` — PASS, 194 checks in each matrix entry:
  - 1120x760 @ 100%
  - 820x560 @ 100%
  - 1120x760 @ 125%.
- Desktop helper regression: `run_a9afd9071cae231a` — PASS, 20/20 (`node --test tests/desktop-ui/*.test.cjs`).
- CLI unique build: `run_45d7bd0b2d0f3a95` — PASS.
- Wails v2.15.0 production build: `run_0a9fa491ea9ab808` — PASS using the existing allowlisted Go executable:
  - `go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -clean -o adm-desktop-phase22-final-56c79e3.exe`
- `git diff --check` PASS on the clean fixed implementation head.

## Fixed-head artifacts

- CLI: `dist/ai-dev-manager-phase22-final-56c79e3.exe`
  - size: 17,032,192 bytes
  - SHA-256: `E53168E6D9614409DC6862C6AB7AAC98D15BD922F88D1CB3E212641FE9013F4A`
- Wails Desktop: `cmd/ai-dev-manager-desktop/build/bin/adm-desktop-phase22-final-56c79e3.exe`
  - size: 17,905,152 bytes
  - SHA-256: `FA2644F46FF7FE9901AED958488C8B122D931253A4B05E526538C01A7225C55A`

## Boundary notes

Phase 22 remains lifecycle infrastructure. Desktop did not gain task records, task planning, GSD automation, automatic cleanup/GC, automatic merge/rebase/push, force cleanup, branch deletion, a second persistence model, or direct state-file mutation. Existing-root mode remains non-Git-capable; Git remains operation-local to managed-worktree mode. CLI lifecycle UX remains Phase 23 scope.

No push, tag, release, or subagents were used.
