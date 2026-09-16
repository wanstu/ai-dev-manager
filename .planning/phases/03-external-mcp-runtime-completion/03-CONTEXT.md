# Phase 03 Context — External MCP Runtime Completion

**Gathered:** 2026-09-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the current partial MCP HTTP proxy (catalog entry + connect-per-call + no health) into a real Environment-scoped external MCP runtime with proper definition lifecycle, health semantics, and connection management. This completes the R1 external MCP slice so that Phase 4 can prove a real Agent development loop using Skills + verifier + MCP + files/exec as one coherent capability set.

</domain>

<decisions>
## Implementation Decisions

### Transport Scope

- **D-01:** Phase 3 transport is Streamable HTTP only. ADM-CORE-012 explicitly says "the first runtime transport is a Streamable HTTP endpoint." Do not add stdio/command-based MCP transport in Phase 3. Broader transports remain deferred unless later dogfood proves concrete need. — **Reversibility:** reversible — stdio can be added later without changing the Streamable HTTP path; the definition model should keep transport as a discriminated field so a future stdio variant slots in cleanly.

### Connection & Session Model

- **D-02:** Do not add persistent connection pooling, session ownership, or background process ownership in Phase 3. Persistent Runtime/MCP/process ownership is explicitly Phase 5 (LIFE-01..03). Phase 3 may use bounded on-demand initialization/health probes and per-operation Streamable HTTP sessions (connect → operation → close), while still defining correct configured/disabled/healthy/error semantics. The plan should choose the smallest semantics satisfying MCP-RUN-01..05 without preempting Phase 5. — **Reversibility:** reversible — connection pooling/ownership can be layered on top of the on-demand model in Phase 5 without reworking the health or status API.

### Secret Handling

- **D-03:** MCP-RUN-05 is locked: secret values are resolved only at activation boundaries and are not exposed in normal status/log output. Plan a minimal secret-reference mechanism appropriate to Streamable HTTP (e.g., environment variable interpolation in endpoint URLs); do not persist resolved secrets. Status/inspect output must redact or omit resolved secret values. — **Reversibility:** costly — changing how secrets flow through the definition model after real MCPs are configured would require a migration of existing catalog entries.

### Health & Activation Lifecycle

- **D-04:** Health must distinguish four states: configured, disabled, healthy, error. Health is based on actual runtime initialization/probe evidence, not configuration existence. Avoid periodic/background health ownership (that belongs later in Phase 5+). Phase 3 health is derived from on-demand probe results: a successful connect-and-list-tools proves healthy; a failed connect or list-tools proves error; disabled means not enabled for the Environment; configured means the catalog entry exists but has not yet been probed or the probe result has been discarded. — **Reversibility:** reversible — health semantics can be enriched with background probes later without breaking the four-state model.

### Definition Management Surface

- **D-05:** The existing human management surface (CLI + MCP management) already supports add/remove/default for MCP catalog entries and enable/disable per Environment. Phase 3 does not need a new management surface; it enriches the existing `CatalogEntry` with transport-specific fields and adds structured health/status reporting through the existing inspection and Gateway tools. — **Reversibility:** reversible — adding richer management later does not conflict with the existing surface.

### Agent-Facing Gateway Tools

- **D-06:** The existing Gateway tools (`environment_mcp_set`, `environment_mcp_tools`, `environment_mcp_call`) remain the Agent-facing surface. `environment_mcp_tools` and `environment_mcp_call` gain proper health checks and structured error responses. A new or enriched inspection path exposes MCP health status to the Agent/management layer. Do not add new Agent-facing tool proliferation; enrich existing tools with better lifecycle semantics. — **Reversibility:** reversible — tool behavior enrichment is backward-compatible.

### Error Handling

- **D-07:** Bad configuration (invalid URL, unreachable endpoint, auth failure) reports a structured error without secret leakage. Errors carry enough identity (MCP ID, error kind) for the Agent or human to diagnose without exposing token values in error messages. — **Reversibility:** reversible — error shape can be extended with more detail later.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Product Contract
- `docs/PRODUCT_CONTRACT.md` §ADM-CORE-012 — Defines MCP catalog + Environment selection model; explicitly scopes first transport to Streamable HTTP
- `docs/PRODUCT_CONTRACT.md` §ADM-GOAL-001 — Primary product goal: let an Agent develop software through MCP
- `docs/PRODUCT_CONTRACT.md` §ADM-CORE-001 — Environment as development context; optional capability model

### Requirements (from ROADMAP.md / PROJECT.md)
- `MCP-RUN-01` — Configured MCP definitions represent actual connection information and transport
- `MCP-RUN-02` — Environment selection gates activation/access at runtime
- `MCP-RUN-03` — Health/status distinguishes configured/disabled/healthy/error; config existence ≠ health
- `MCP-RUN-04` — Real external MCP discovery/call works through ADM without bypassing Environment policy
- `MCP-RUN-05` — Secret values resolved only at activation boundaries; not exposed in normal status/log

### Phase Lifecycle Constraints (deferred scope)
- `LIFE-01..03` — Persistent Runtime/MCP/process ownership — explicitly Phase 5, not Phase 3

### Prior Phase Patterns
- `.planning/phases/02-structured-verifier-runtime/02-CONTEXT.md` — Structured typed results, bounded output, writer-gated execution pattern
- `.planning/phases/01-skill-runtime/01-CONTEXT.md` — Environment-gated capability, explicit discovery roots, real artifact model

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/catalog/service.go` — Catalog CRUD for MCP entries (AddMCP, List, Remove, SetDefault); endpoint URL validation already present
- `internal/gateway/server.go:553-578` — `enabledMCPEndpoint()` resolves endpoint from Environment+catalog; `connectExternalMCP()` creates Streamable HTTP client session
- `internal/gateway/server.go:358-400` — `environment_mcp_tools` and `environment_mcp_call` handlers; existing connect-per-call pattern to enrich
- `internal/app/service.go:111-127` — `InspectEnvironment` resolves enabled/unresolved MCPs from catalog; existing resolution pattern
- `internal/model/types.go:46-55` — `CatalogEntry` struct with `Endpoint` field; needs transport-specific extension
- `internal/model/types.go:30-44` — `Environment` struct with `EnabledMCPIDs`; unchanged in Phase 3
- `internal/verifier/` package (Phase 2) — Precedent for Environment-scoped definition CRUD + typed result pattern

### Established Patterns
- Environment-scoped definitions persist on `Environment` or in global catalog; selections are IDs only
- Gateway tools are thin handlers over app service; validation and policy live in app layer
- Structured results carry identity + status + bounded output; timeout is status + flag, not error string
- Writer-gated execution reuses `Runtime.Exec`; no new execution primitive added
- Tests use real Streamable HTTP Gateway client for acceptance (Phase 1/2 precedent)

### Integration Points
- `internal/catalog/service.go` — Enrich `CatalogEntry` with transport type and optional secret references
- `internal/gateway/server.go` — Add or enrich MCP health/status tool; improve error responses in tools/call
- `internal/app/service.go` — Add health probe logic at MCP activation boundary (on first tools/call or explicit status query)
- `internal/management/service.go` — Ensure CLI/management surfaces expose health status correctly
- `internal/desktop/adapter.go` — Ensure Desktop inspection surfaces MCP health (minimal, no new UI)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for health probing and secret-reference mechanisms. The plan should prove health semantics with a real local Streamable HTTP MCP (e.g., a test fixture or the actual `@modelcontextprotocol/server-everything` if available locally).

</specifics>

<deferred>
## Deferred Ideas

- stdio/command-based MCP transport (ADM-CORE-012 says first transport is HTTP; broader transports deferred)
- Persistent connection pooling or session ownership (Phase 5, LIFE-01..03)
- Periodic/background health monitoring (Phase 5+)
- Desktop MCP health UI beyond basic inspection surface
- Migration/compatibility burden for existing catalog entries

</deferred>

---

*Phase: 03-External MCP Runtime Completion*
*Context gathered: 2026-09-07*
