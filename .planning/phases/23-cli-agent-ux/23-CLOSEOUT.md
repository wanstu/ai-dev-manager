# Phase 23 Closeout — CLI Agent UX + MCP/Skill Provisioning

Date: 2026-09-13
Planning baseline: `bb1fc82` (`docs: close Phase 22 temporary environment acceptance`)
Planning commit: `2e4e575` (`docs: plan Phase 23 CLI agent UX`)
23-01 implementation: `b733948` (`feat(cli): improve MCP and Skill provisioning`)
23-01 acceptance docs: `c1c3978` (`docs: record Phase 23-01 CLI acceptance`)
Final implementation head: `f85c566` (`feat(cli): expose environment agent workflows`)
Status: COMPLETE

## Outcome

Phase 23 makes the normal `adm` CLI materially easier to use for MCP/Skill provisioning and Agent-facing Environment workflows without creating a second management plane or CLI-specific product model.

The production CLI remains a thin client over the selected Admin MCP endpoint. Stable Workspace/Environment IDs remain explicit. Successful management commands emit canonical shared-model JSON; failures remain non-zero CLI errors. `gateway`, `doctor` and `state` retain their existing explicit bootstrap/offline/recovery role rather than becoming peer writable management backends.

## Closeout acceptance C01–C10

### C01 — Normal CLI remains Admin-MCP-only

PASS. New MCP/Skill/context/temporary commands use the existing Admin-MCP client path. Real acceptance points them at unavailable Admin MCP targets and proves no mutation of disposable local `ADM_V2_HOME` state. No normal-mode writable fallback was added.

### C02 — MCP file/stdin import

PASS at `b733948`. MCP preview/apply accept exactly one of inline JSON/JSONC, local `--file`, or explicit redirected `--stdin` through one shared resolver. Zero/multiple-source errors occur before mutation. Literal credential-bearing values remain converted to references rather than persisted or returned as raw values.

### C03 — MCP probe/status/inspect/refresh remain distinct

PASS at `b733948`. Global `mcp probe`, Environment one-shot `mcp status`, passive owner-runtime `mcp inspect`, and explicit owner-runtime `mcp refresh` are separately exposed and documented. Passive inspect adds zero upstream traffic; refresh may reconnect/Ping/inventory-refresh but calls zero business tools and does not change desired definitions or Environment selection.

### C04 — Skill source provisioning remains explicit

PASS at `b733948`. Source add/update accept repeated support roots, source update does not implicitly refresh, and refresh remains an explicit operation. The CLI projects the existing canonical `[]string` model rather than inventing comma-encoded configuration.

### C05 — Skill availability scopes are clear

PASS at `b733948`. `skill availability` reports global structural availability independently of Environment enablement. `environment skill list/inspect` report Environment-specific enabled/disabled/broken availability without reading full Skill instructions or mutating source/catalog state.

### C06 — Canonical Environment context bundle

PASS at `f85c566`. `environment context` calls the existing Phase-20 `environment_context_bundle` through Admin MCP with explicit stable Environment ID and bounded request budgets. Acceptance proves no added MCP requests during the context call, no writer lease change, no private/Global Memory values, no hidden current Environment, and existing path/identity authority remains enforced.

### C07 — Phase-22 temporary lifecycle CLI parity

PASS at `f85c566`. CLI exposes temporary create/status/promote/cleanup using the existing Phase-22 models and Admin-MCP tools. Create requires explicit owner + positive TTL, optional session/run are provenance only, status is read-only, promote is retention-only, and cleanup defaults to preview. Execution requires explicit `--execute`; there is no force option.

### C08 — Non-Git existing-root safety / optional managed worktree

PASS. Real CLI acceptance creates and cleans an ordinary temporary Environment in a non-Git Workspace and proves a real project marker remains. Requesting managed-worktree mode on that non-Git Workspace fails only that optional operation. Phase-22 fixed/full regression continues to prove successful managed-worktree creation, dirty/unpublished/tamper blockers, retained branch and targeted cleanup semantics.

### C09 — Automation behavior

PASS. New commands use existing canonical JSON stdout on success and existing non-zero error behavior on failure. No universal formatter, output envelope, query language, template subsystem or CLI-only DTO layer was introduced. Help remains human-readable text.

### C10 — Fixed-head regression and artifact

PASS on `f85c566`:

- focused: `run_e6dd66630f674d97` — PASS;
- new Environment CLI repeat x3: `run_73ace1e85bf573f4` — PASS;
- full repository: `run_b7593afcb1f20f56` — PASS, Gateway 205.344s;
- vet: `run_88f9c56ff957c7a4` — PASS;
- CLI build: `run_56be6a0b03f2bee9` — PASS;
- `git diff --check` / staged implementation check — PASS.

Artifact:

- `dist/ai-dev-manager-phase23-final-f85c566.exe`
- 17,175,552 bytes
- SHA-256 `6E0E54F0EBDC3A7DEF20316C24B15E188692548E3D2AEA92DD54ADB561CF690A`

No Wails/Desktop build was required because Phase 23 changed no Desktop feature code.

## Delivered command surface

Phase 23 adds or completes normal CLI access to:

- MCP import preview/apply from inline/file/stdin;
- global MCP probe, Environment status, passive inspect and explicit refresh with documented side-effect distinctions;
- Skill source add/update with repeated support roots and explicit refresh;
- global and Environment-specific Skill availability diagnostics;
- Phase-20 bounded Environment context bundle;
- Phase-22 temporary Environment create/status/promote/preview/execute lifecycle.

All remain projections of shared Core/Gateway/Admin-MCP contracts.

## Preserved boundaries

Phase 23 did not add task records, Planner/Executor/Reviewer policy, GSD `.planning` interpretation, hidden current project state, cwd-based authorization, direct writable state fallback, a second Admin API, automatic Memory/Skill context injection, MCP business-tool calls during diagnostics, automatic source refresh, force temporary cleanup, automatic GC, automatic merge/rebase/push, branch deletion, Desktop feature work, or the Phase-24 package split.

Git remains optional except when the caller explicitly requests managed-worktree behavior. Optional MCP/Skill/Git/verifier capabilities remain operation-local.

## Next phase

Phase 24 — Desktop/CLI Surface Boundary Split — remains PLANNED / NOT STARTED. It must not begin automatically from this closeout.

No push, tag, release, or subagents were used.
