# Phase 13 Context — Environment Capability Diagnostics

## Goal

Give Agents and humans one authoritative, resilient view of what can actually be used in one Environment, what is unavailable, and why, without turning optional capability failures into global Environment failure.

## Requirements

- CAP-01 — one Environment capability inspection surface reports usable/unavailable capabilities with structured reasons and evidence.
- CAP-02 — optional capability failures remain operation-local and never silently become global Environment prerequisites.

## Existing foundation

`app.InspectEnvironment` currently returns:

- Environment summary;
- Workspace;
- Runtime capability string list;
- resolved enabled MCP/Skill catalog entries;
- unresolved MCP/Skill IDs.

This is useful management information, but it is not yet an authoritative usability report.

## Current gaps

1. `InspectEnvironment` calls `Capabilities`, which first constructs the Environment Runtime. If Runtime validation fails, the entire inspection fails instead of reporting component-level degradation.
2. Runtime capabilities are bare strings such as file/exec/Git support; there is no structured reason/evidence.
3. Verifier configured/disabled/invalid-executable state is not summarized with Runtime facts.
4. MCP selection is listed, but actual desired/observed MCP availability from Phase 11 is not integrated.
5. Skill selection is listed, but source/artifact availability from Phase 12 is not integrated.
6. Managed-worktree validity and generic owner capabilities are separate probes.
7. One broken optional component can force callers to probe many separate tools to understand the Environment.

## Locked decisions

1. Capability diagnostics are read-only observation. They do not acquire writer leases, start processes, call MCP tools, execute verifiers or mutate project state.
2. One `CapabilityFact` model is authoritative across application/Gateway/CLI surfaces.
3. Capability facts distinguish at least `available`, `disabled`, `unconfigured`, `unavailable`, and `degraded`/`error` where needed.
4. Every unavailable/degraded fact has a stable `reason_code`, human-readable sanitized message and evidence referencing ADM IDs/configuration/runtime facts.
5. Evidence never contains secret values or Environment-private Memory values.
6. Base capabilities are granular. Example facts include read/search, write, exec, verifier definitions, process/run support, Git, isolation, each selected MCP, and each selected Skill rather than one global boolean.
7. Writer-gated capabilities report that writer authority is required and expose current lease facts, but capability inspection itself never requires the writer.
8. Static desired/configuration facts are produced by application services. Owner-local runtime observation (especially MCP/process/run) may enrich the same model when the Gateway owner is available.
9. Default inspection does not force a network reconnect or call external MCP tools. Explicit MCP refresh/probe remains Phase 11's operation. Diagnostics consume current desired + observed facts.
10. A failed/invalid Environment root may make file/exec/Git/stdio-MCP capabilities unavailable while still allowing global/HTTP MCP or Skill diagnostics to be returned where possible.
11. Diagnostics must reuse real validators/authority checks. Do not create a second permissive model that says a capability is available when the actual operation would reject it.

## Non-goals

- no automatic remediation;
- no tool/task selection policy;
- no Agent planning;
- no broad code investigation helpers yet;
- no secret/Memory dump;
- no Desktop redesign in this Phase.
