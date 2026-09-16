# ADM V2 Core Roadmap Plan — 2026-09-08

## Product boundary

ADM is the local development control plane used by external Agents. ADM owns capability configuration, Environment authorization, safe local Runtime execution, persistent resource lifecycle and diagnostics. The consuming Agent/GSD owns task planning, sequencing, review policy and project/phase completion.

## Next delivery sequence

### Phase 10 — Orchestration Boundary Cleanup

Remove the mis-scoped Phase 9 workflow surface and retain only generic single-command asynchronous `run_` lifecycle. The abandoned GSD executor branch never merges.

### Phase 11 — MCP Runtime Completion

Make MCP a complete first-class runtime capability rather than a Streamable HTTP vertical slice.

User outcome: configure one MCP definition, enable it in one or more Environments, and reliably use/inspect/refresh it through ADM with clear transport/auth/runtime diagnostics.

Locked scope:

- supported transports: `streamable-http` and local `stdio`/command;
- legacy HTTP+SSE is not added because it is deprecated in the current MCP specification;
- stdio executable remains subject to ADM's executable allowlist so MCP configuration cannot become an exec-policy bypass;
- configured secret references resolve only during activation and are never returned as values;
- explicit refresh may reconnect and refresh tool inventory;
- enabled MCPs have configurable protocol health checks and optional automatic reconnect with configurable intervals/timeouts under the persistent Gateway owner;
- failed tool calls may invalidate the connection, but ADM does not automatically replay a tool call because the call may have side effects;
- desired configuration and health policy remain persisted; observed connection/session/health timestamps/failure counters/tool inventory remain runtime observation;
- single/batch JSON/JSONC import uses preview + atomic apply into canonical ADM definitions for OpenCode, WorkBuddy/CodeBuddy, Codex plugin MCP JSON, Claude Code and supported MCPHub shapes; external schemas remain importer-only.

### Phase 12 — Skill Runtime Completion

Make real `SKILL.md` sources refreshable, inspectable and diagnosable without creating a Skill execution engine.

User outcome: know which real Skills are installed, where they came from, whether they are enabled and readable in an Environment, and why a broken Skill is unavailable.

Locked scope:

- explicit persisted Skill sources, never arbitrary disk scan;
- source refresh is atomic per source;
- stable Skill identity is source/artifact based rather than name-only;
- deleted Skills become unresolved selections instead of silently rebinding to another Skill with the same name;
- support roots and bounded authorized support-file reads remain explicit;
- ADM does not interpret Skill instructions or decide how to execute them.

### Phase 13 — Environment Capability Diagnostics

Provide one resilient capability report for an Environment.

User outcome: ask one surface what files/exec/verifier/process/Git/isolation/MCP/Skill capabilities are usable and get structured reasons/evidence for unavailable ones.

Locked scope:

- one broken optional capability does not fail the entire report;
- statuses distinguish usable, disabled, unconfigured, unavailable and degraded/error states;
- evidence is sanitized and references ADM IDs/configuration facts, never secret values or private Memory values;
- diagnostics reuse the same authority/runtime checks used by real operations rather than maintaining a second truth model.

## Deferred planning

Phase 14 evidence-first investigation helpers are intentionally not decomposed into implementation plans yet. Endpoint resolution, symbol/reference/write tracing, data lineage and similar helpers must be chosen from real dogfood evidence after Phases 11-13 prove the Core. This prevents another speculative feature branch from displacing MCP/Skill priorities.

## External protocol note

The repository currently uses `github.com/modelcontextprotocol/go-sdk v1.7.0`. The official Go SDK compatibility table states v1.7.0+ supports MCP spec `2026-07-28`. That specification deprecates legacy HTTP+SSE, so Phase 11 targets Streamable HTTP and stdio instead of expanding the deprecated transport surface.
