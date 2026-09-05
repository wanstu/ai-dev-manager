# Phase 26 Context — External Agent Dogfood

## Why this phase exists

v0.6 has crossed the functional threshold for external Agent use: one stable Agent MCP Gateway can discover Workspaces/Environments, create/destroy Environments, acquire/release a writer, and route read/write/edit/git/exec/verify by `environment_id`.

The next product risk is no longer "can ADM expose coding tools?". It is "can a real external Agent configure ADM once and comfortably use it for daily development?"

Before adding Process/logs/ports speculatively, v0.7 starts by dogfooding the v0.6 Gateway through a real external Agent connection such as ChatGPT/Codex/Claude/OpenCode.

## Product boundary

- External Agent remains the brain.
- ADM remains Environment facts, policy, execution, isolation and verification.
- Do not add built-in LLM planning.
- Do not add per-Environment MCP configuration.
- Existing Direct MCP remains available for compatibility/debugging.

## `@adm` readiness

From the ADM implementation side, v0.6 is already sufficient to expose an `@adm`-style MCP connector.

The remaining integration requirement depends on the caller:

1. If the external Agent/connector runtime can reach the development machine's loopback Gateway endpoint, register the stable Gateway URL directly.
2. If the connector executes remotely and cannot reach `127.0.0.1`, this is an MCP exposure/auth problem, not a Remote Runtime problem. Add only the minimum safe exposure required by the actual caller.

Do not introduce SSH Runtime merely to solve remote Agent -> local ADM connectivity.

## Dogfood target

A real Agent should be able to:

1. connect once to ADM Gateway;
2. list Workspaces and existing Environments;
3. create a new Environment for a real task;
4. acquire one writer lease;
5. inspect/read/search/git status;
6. edit/write/exec/verify inside that Environment;
7. never need the worktree path or a separate MCP endpoint;
8. leave the branch/Environment for human review rather than auto-merge;
9. resume after daemon restart using the same Gateway endpoint and Environment ID.

## First dogfood finding — Docker reachability

The first real `@adm` registration attempt proved the connector reaches the host through `host.docker.internal`, not host loopback. The v0.6 loopback-only Gateway therefore blocked the real caller even though the MCP server itself was complete.

Phase 26 responds with explicit `gateway up --docker`: it preserves the stable Gateway port, binds for local Docker reachability, reports both `127.0.0.1` and `host.docker.internal` endpoints, persists the mode across daemon restart, and applies Host/Origin restrictions. This is local Docker dogfood only; it is not authenticated remote/LAN exposure.

## Second dogfood finding — MCPHub tool-schema compatibility

The real MCPHub 1.0.33 connector could initialize ADM successfully but displayed zero tools. MCPHub logs showed its `@modelcontextprotocol/sdk@1.29.0` + Zod validator rejected routed tool schemas at `outputSchema.properties.result`: Go MCP SDK had represented `Result any` as the valid JSON Schema boolean `true`, while MCPHub's validator accepts only object-form schemas there.

ADM now explicitly publishes routed read/mutation `result` as the equivalent permissive object-form schema `{}`. Runtime values remain unconstrained JSON, but `tools/list` no longer contains `"result": true`. Verification used MCPHub's own container and exact TypeScript MCP SDK/Zod stack to connect to `http://host.docker.internal:41137/mcp/`; `listTools()` returned all 19 tools successfully.

## What we want to learn

Record concrete friction rather than speculative features:

- Is Workspace discovery understandable enough for an Agent?
- Does Environment creation need better semantic naming/description metadata?
- Is explicit `environment_id` on every call too verbose in practice?
- Is `writer_owner` ergonomics sufficient for real Agent sessions?
- Are facts/warnings/hints actionable without becoming directives?
- Which missing development capability blocks real work first: process lifecycle, logs, ports, HTTP verification, or something else?
- Does the target external Agent require non-loopback Gateway exposure/auth?

## Exit criteria

Phase 26 is complete when at least one real external Agent connection has used one stable ADM Gateway to complete a small real coding task in an isolated Environment, and any blocking integration gaps have either been fixed or converted into evidence-backed v0.7 requirements.
