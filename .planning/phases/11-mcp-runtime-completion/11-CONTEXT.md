# Phase 11 Context — MCP Runtime Completion

## Goal

Turn ADM's current external-MCP foundation into a complete, reliable and diagnosable runtime capability that external Agents can use without bypass tools.

## Requirements

- MCP-COMP-01 — explicit desired MCP definition model separated from observed runtime state.
- MCP-COMP-02 — every advertised transport has a real implementation and lifecycle.
- MCP-COMP-03 — auth/secret configuration is explicit and secret values never leak through normal status/errors/logs/import previews.
- MCP-COMP-04 — real tool inventory is inspectable/refreshable and Environment disable revokes access immediately.
- MCP-COMP-05 — connection/discovery/call/reconnect failures have actionable structured diagnostics and stale healthy state is impossible.
- MCP-COMP-06 — enabled MCPs support configurable periodic protocol health checks and optional fixed-interval automatic reconnect under the persistent Gateway owner.
- MCP-COMP-07 — single/batch JSON or JSONC import adapters normalize supported OpenCode, WorkBuddy/CodeBuddy, Codex plugin MCP JSON, Claude Code and MCPHub data into canonical ADM MCP definitions through preview + atomic apply.

## Existing foundation

ADM already has:

- global MCP catalog entries;
- Environment enable/disable selection;
- Streamable HTTP activation;
- environment-variable-backed header references;
- four-state configured/disabled/healthy/error health;
- real `tools/list` and `tools/call` through the Gateway;
- Gateway-owned external MCP sessions with restart reconciliation and cleanup.

These are a strong vertical slice, but the persisted MCP definition still shares a generic `CatalogEntry` with Skill fields, transport support is restricted to Streamable HTTP, observed health is relatively coarse, and there is no explicit tool-inventory refresh/status contract.

## Locked decisions

1. MCP is a Core ADM capability. This Phase does not add task planning or tool-selection policy.
2. Supported transports for this Phase are:
   - `streamable-http` for remote/local HTTP MCP;
   - `stdio` for local command MCP.
3. Legacy HTTP+SSE is not added. The current MCP specification deprecates the legacy HTTP+SSE transport.
4. The repository's current official Go MCP SDK `v1.7.0` already supports protocol revision `2026-07-28`; dependency replacement is not required merely for protocol-version support.
5. Persisted desired MCP configuration and Gateway-owner observed runtime status are different models. Session IDs, health observations and tool inventories are never persisted as desired config.
6. Stdio MCP executable launch must obey ADM's executable allowlist. Adding an MCP definition must not create a second arbitrary-process authority path.
7. A stdio MCP is activated per Environment selection under the Gateway owner. Its process working directory is the Environment root; the definition remains global/shareable.
8. Secret values are references in persisted config and resolve only at activation. Status can expose that a reference exists/missing, but never its resolved value.
9. Phase 11 supported HTTP auth modes are `none` and explicit secret-backed headers. Interactive OAuth is not silently approximated; it remains unsupported until ADM has an approved secure credential/token lifecycle.
10. `environment_mcp_refresh` explicitly drops/re-establishes runtime observation and refreshes the actual tool inventory.
11. On connection/list failure, ADM may reconnect for a subsequent safe inspection/list operation. On tool-call failure, ADM drops the bad runtime session but does not automatically replay the tool call because a failed response does not prove the remote side had no side effect.
12. Disabling/removing an MCP or dropping an Environment immediately closes its owned session/process and invalidates observed inventory.
13. One broken MCP affects only that MCP. Files, Skills, verifiers and other MCPs remain usable.
14. MCP health uses the protocol `ping` operation when a live session exists. Tool listing is inventory/discovery, not the normal heartbeat mechanism.
15. Persisted `health_policy` is explicit and configurable per MCP: `health_check_enabled`, `check_interval_seconds`, `probe_timeout_seconds`, `auto_reconnect`, and `reconnect_interval_seconds`. Health-check timing is user-configurable; Phase 11 does not lock one universal interval. `auto_reconnect` defaults to `false`. Reconnect uses the configured fixed interval only; no exponential backoff policy is introduced. The Gateway owner records transient timestamps/failure counters/next reconnect time; those observations are never persisted as desired state.
16. Only one health probe or reconnect may be in flight per `(environment_id, mcp_id)`. Disable/remove/config change invalidates pending recovery work so a stale reconnect cannot resurrect revoked capability.
17. Automatic reconnect may re-establish a session and refresh public tool inventory after success. It must never replay a failed tool invocation.
18. Import is an adapter boundary, not a compatibility model. External JSON/JSONC is parsed into canonical `MCPDefinition`; source-specific fields do not leak into runtime/domain types.
19. Import always supports a preview before apply. Batch apply is atomic for the selected candidates. Name conflicts default to error; explicit `skip` or explicit update-by-name may be offered, and update preserves ADM identity/Environment selections while dropping affected live sessions.
20. Phase 11 import writes only the global MCP catalog. External enabled/disabled flags are preview facts only; import never enables an MCP in an existing Environment and exposes no target-Environment option in this Phase. Imported definitions use the global catalog's normal default-include field only when explicitly provided by the ADM import request; otherwise they default to not being auto-enabled for newly created Environments.
21. Recognized environment-reference syntaxes are normalized without resolving them during preview. Literal credential-bearing values are converted into generated secret/environment-reference requirements instead of being persisted as literals. Preview reports the generated reference names and affected field paths without echoing the credential values; apply persists only the references. The caller must provision those references before activation. Raw import payloads and resolved secrets never appear in normal logs/status.

## Non-goals

- no Planner/Executor/Reviewer logic;
- no automatic choice of which MCP tool an Agent should call;
- no legacy SSE transport implementation;
- no insecure OAuth token persistence;
- no generic arbitrary-process launch outside the existing executable authority;
- no Desktop redesign in this Phase.
