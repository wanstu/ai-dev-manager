# Phase 27 Context — Project Toolchain & Verifier Onboarding

## Why this phase exists

Phase 26 proved that a real external Agent can use one ADM Gateway to create and operate an isolated Environment. The first coding task still had one important escape hatch: final `go test ./...` verification had to run through `@pj`, because `workspace prepare` intentionally authorized only `git` and the project had no configured verifier/tool path for Go.

This is now the first evidence-backed v0.7 capability gap.

## Product boundary

ADM must not let an Agent silently escalate its own execution policy. Workspace registration remains read-only by default. Toolchain authorization is a human/control-plane action; Environment tools merely consume the resulting EffectiveConfig.

The Agent-facing Gateway therefore does **not** get a generic `workspace_prepare` or arbitrary policy mutation tool in this phase.

## First vertical slice: Go project

The current repository is a Go project and the host Go executable is known at `D:\tools\go\go1.26.5\bin\go.exe`.

Phase 27 should make the human CLI able to prepare this project with a minimal Go toolchain profile while preserving existing project config:

```text
ai-dev-manager workspace prepare <workspace-id> --go <absolute-go-executable>
```

Expected configuration effect is intentionally split by ownership:

- user-level Workspace Registry stores a machine-local execution overlay;
- the local overlay records minimum `standard`, keeps `git` allowed for managed worktree lifecycle, and adds `go` only when explicitly requested;
- the explicit absolute `tool_paths.go` is stored only in that user-level Workspace local policy and never in Project config;
- Project config may add an absent conventional `go-test` verifier as `go test ./...` because that command definition is shareable repository intent;
- existing Project verifier/MCP/Skill/policy configuration is preserved and never silently overwritten;
- local minimum `standard` upgrades read-only/workspace-write but does not downgrade an existing Project `full` policy;
- an explicit caller Runtime Override policy remains the highest-precedence execution policy.

Repeated prepare must be idempotent.

## Capability-health finding

During continued dogfood, `search` was advertised but one routed call returned `runtime_error`. Phase 27 must not equate a capability name with verified health. The immediate requirement is to make failures diagnosable and to add tests around the real prepared-runtime surface; do not invent a generic health subsystem unless the concrete failure requires it.

## Success path

After human toolchain preparation, the existing Environment should rebuild its derived Runtime from the base Workspace EffectiveConfig on the next call. The external Agent should then be able to:

1. acquire/renew writer;
2. call `run_verifier` for `go-test`;
3. receive structured pass/fail output;
4. run `run_verifiers` without raw shell strings or raw worktree paths;
5. finish the coding verification loop without falling back to another MCP executor.

## Non-goals

- no arbitrary shell strings;
- no automatic PATH-wide executable trust;
- no automatic package-manager detection for every ecosystem;
- no Agent-facing permission escalation tool;
- no Process/dev-server/log/port manager yet;
- no remote/LAN auth work unless a real connection requires it.

## Exit criteria

Phase 27 is complete when the real `@pjadm` path can run the configured Go test verifier inside the existing Environment using a human-authorized explicit Go ToolPath, all relevant Go tests remain green, and the default unprepared Workspace behavior remains read-only.
