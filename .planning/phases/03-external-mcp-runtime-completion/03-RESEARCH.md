# Phase 03 Research — External MCP Runtime Completion

**Researcher:** gsd-phase-researcher
**Date:** 2026-09-07
**Status:** Complete

---

## 1. Current State Analysis

### 1.1 What Exists

The current MCP proxy is a functional vertical slice that proves Environment-scoped external MCP access works, but it treats configuration-as-health and has no transport lifecycle, health semantics, or secret handling.

**Data model (`internal/model/types.go:46-55`):**

```go
type CatalogEntry struct {
    ID                  string   `json:"id"`
    Name                string   `json:"name"`
    DefaultIncludeInEnv bool     `json:"default_include_in_environment"`
    Endpoint            string   `json:"endpoint,omitempty"`
    // ... Skill-only fields follow
}
```

- `Endpoint` is the only MCP connection field — a plain URL string.
- No transport type, no auth references, no health state.

**Catalog CRUD (`internal/catalog/service.go:35-45`):**

- `AddMCP(name, endpoint, defaultInclude)` validates `http(s)` URL format only.
- `List`, `Get`, `Remove`, `SetDefault` — standard CRUD, unchanged since Phase 1.

**Gateway proxy (`internal/gateway/server.go:358-400, 553-578`):**

- `enabledMCPEndpoint(ctx, service, envID, mcpID)` resolves the endpoint via `InspectEnvironment` → checks `info.EnabledMCPs` → returns the `Endpoint` string or error.
- `connectExternalMCP(ctx, endpoint)` creates a per-call `mcp.Client` + `StreamableClientTransport` + `Connect` → returns `*mcp.ClientSession`.
- `environment_mcp_tools` handler: connect → ListTools (paginated) → close → return `{"mcp_id": ..., "tools": [...]}`.
- `environment_mcp_call` handler: connect → `session.CallTool` → close → return result.
- Every call creates a new connection and tears it down after. No caching, no reuse (by design — D-02).

**Environment inspection (`internal/app/service.go:98-130`):**

- `InspectEnvironment` resolves enabled MCP IDs to `[]model.CatalogEntry` + unresolved IDs.
- No health information is returned.
- MCP entries appear as raw `CatalogEntry` objects with `Endpoint` visible in inspect output.

**Management surface (`internal/management/service.go`):**

- `MCPAdd`, `MCPRemove`, `MCPSetDefault`, `EnvironmentMCPSet` — thin delegates.
- `Snapshot` returns `[]model.CatalogEntry` for MCPs — raw endpoint visible.

**CLI (`cmd/ai-dev-manager/main.go:505-612`):**

- `mcp add --name --endpoint [--default]`
- `mcp list`, `mcp remove --id`, `mcp set-default --id --enabled`
- `environment mcp enable/disable --environment-id --mcp-id`

**Tests (`internal/gateway/server_test.go:572-750`):**

- `TestGatewayUsesOnlyEnabledConfiguredExternalMCPAndSkillRuntime` — creates a real `httptest.NewServer` with a mock external MCP (exposes `external_ping`), proves Environment enable/disable gates access, proves disabled Env cannot call, proves disabling revokes immediately.
- This is the strongest existing test and directly validates MCP-RUN-02 and MCP-RUN-04.

### 1.2 What Works

| Capability | Status | Evidence |
|---|---|---|
| Store HTTP endpoint in catalog | Working | `AddMCP` validates URL, persists |
| Environment enable/disable gate | Working | `enabledMCPEndpoint` check, test proves |
| Connect-per-call proxy | Working | `connectExternalMCP`, paginated ListTools |
| Real external MCP list-tools | Working | Test with `httptest.NewServer` mock |
| Real external MCP call-tool | Working | Test with `external_ping` → `external-pong` |
| CLI management | Working | `mcp add/list/remove/set-default` |
| CLI environment selection | Working | `environment mcp enable/disable` |

### 1.3 What's Missing (Phase 3 Scope)

| Gap | Requirement | Severity |
|---|---|---|
| No transport type field on `CatalogEntry` | MCP-RUN-01 | Medium — only HTTP exists now, but model should declare it |
| No health concept at all | MCP-RUN-03 | High — 4-state health is a core deliverable |
| Health conflated with config existence | MCP-RUN-03 | High — current inspect shows "enabled" MCPs as if healthy |
| No secret handling | MCP-RUN-05 | High — endpoint URLs may contain tokens; inspect/status/logs expose them |
| No structured error responses | MCP-RUN-07 | Medium — current errors are raw `fmt.Errorf` strings |
| No health probe mechanism | MCP-RUN-03 | High — health must come from actual connect/list-tools evidence |
| `environment_mcp_tools`/`_call` lack health-aware errors | MCP-RUN-03/04 | Medium — callers can't distinguish "not configured" from "broken" |
| No MCP health inspection tool | MCP-RUN-03 | Medium — Agent/CLI needs explicit health query |
| Snapshot exposes raw endpoint URLs | MCP-RUN-05 | Medium — management surface leaks potential secrets |

---

## 2. Technical Approach

### 2.1 MCP-RUN-01: Transport-Specific Definition

**What to add to `CatalogEntry`:**

```go
type CatalogEntry struct {
    ID                  string            `json:"id"`
    Name                string            `json:"name"`
    DefaultIncludeInEnv bool              `json:"default_include_in_environment"`
    Endpoint            string            `json:"endpoint,omitempty"`
    Transport           string            `json:"transport,omitempty"`           // NEW: "streamable-http" (default), future: "stdio"
    HeaderRefs          map[string]string `json:"header_refs,omitempty"`         // NEW: env-var-referenced auth headers
    // ... Skill-only fields unchanged
}
```

**Design choices:**
- `Transport` defaults to `"streamable-http"` when empty (backward-compatible with existing catalog entries).
- Phase 3 only implements Streamable HTTP. The field exists to keep the model extensible without a Phase 5 migration.
- `HeaderRefs` stores key→env-var-reference mappings, e.g. `{"Authorization": "Bearer ${MCP_API_TOKEN}"}`. The actual secret values are resolved at activation time, never stored.
- The `AddMCP` signature gains optional transport config. The simplest approach: add a new `AddMCPConfig` method or a config struct to avoid breaking the existing `AddMCP` call sites.

**Impact on existing catalog entries:** None. Empty `Transport` defaults to `"streamable-http"`. No migration needed (ADM-CORE-011: no compatibility burden).

### 2.2 MCP-RUN-02: Environment Selection Gates Activation

Already works via `enabledMCPEndpoint()`. No changes needed to the gating mechanism. The health enrichment (MCP-RUN-03) will be layered on top.

### 2.3 MCP-RUN-03: Health Semantics (4 States)

This is the largest new capability in Phase 3.

**Health states:**
1. **`configured`** — Catalog entry exists, endpoint is set, but has not been probed or probe result is stale/discarded. This is the initial state after `mcp add` or for an MCP that was just enabled.
2. **`disabled`** — MCP is not enabled for this Environment. Deterministic from `EnabledMCPIDs` — no probe needed.
3. **`healthy`** — On-demand probe succeeded: `connect → list-tools` completed without error.
4. **`error`** — On-demand probe failed: connect refused, timeout, auth failure, or list-tools failed.

**Health is derived, not persisted (D-02, D-04):**

- No health field on `CatalogEntry` or `Environment`.
- Health is computed per-call by attempting a probe.
- Probe results are NOT cached across Gateway restarts.
- Within a single Gateway lifetime, successful probe results may be cached briefly (one tool call duration) to avoid redundant probes when `tools` + `call` are invoked in sequence.

**Probe implementation (new `internal/app/mcp_health.go`):**

```go
type MCPHealthStatus struct {
    MCPID   string `json:"mcp_id"`
    State   string `json:"state"`   // "configured" | "disabled" | "healthy" | "error"
    Message string `json:"message,omitempty"`  // error details for "error" state
}
```

The probe function:
1. Check if MCP is enabled for the Environment → if not, return `disabled`.
2. Check if catalog entry exists and has an endpoint → if not, return `configured` with message about missing endpoint.
3. Resolve endpoint (apply `HeaderRefs` env-var interpolation → D-03/D-05).
4. Attempt `connectExternalMCP(ctx, resolvedEndpoint)` with a bounded timeout (e.g., 10s).
5. On success: attempt `session.ListTools(ctx, nil)` with a bounded timeout (e.g., 10s).
6. On full success: return `healthy`.
7. On any failure: return `error` with structured message (error kind + MCP identity, no secrets).

**Gateway tool enrichment:**

- `environment_mcp_tools` — before connecting, probe health. If `error`, return structured error. If `healthy` or first-time, proceed with tool listing.
- `environment_mcp_call` — same pattern.
- New optional input field or separate tool `environment_mcp_status` — explicitly probe and return health without listing tools.

**Alternatively, the simpler approach (recommended):**

The existing `environment_mcp_tools` and `environment_mcp_call` already do a connect-per-call. Instead of adding a separate probe step, enrich their error handling:
- If connect succeeds → the MCP is `healthy` for this call.
- If connect fails → return a structured error with health context: `{"mcp_id": "...", "error_kind": "connection_refused|timeout|auth_failure|tool_list_failed", "message": "..."}`.
- A dedicated `environment_mcp_status` tool can do the probe without listing tools, returning the 4-state health.

**This is the recommended approach** because it avoids redundant probes, respects D-02 (no persistent connections), and the connect-per-call model already IS the health evidence.

### 2.4 MCP-RUN-04: Real External MCP Discovery/Call

Already works. The test at `server_test.go:572-750` proves end-to-end. Phase 3 enriches this with:
- Structured error responses when connection fails (MCP-RUN-03/07).
- Secret resolution at the connect boundary (MCP-RUN-05).

### 2.5 MCP-RUN-05: Secret Values Resolved at Activation Boundaries

**Env-var interpolation model:**

- `CatalogEntry.HeaderRefs` stores `{"Authorization": "Bearer ${MCP_TOKEN}"}`.
- At activation time (inside `connectExternalMCP`), resolve each value via `os.ExpandEnv()`.
- The resolved endpoint (if it contains secrets in URL params) is also resolved at this boundary.
- The resolved values are used ONLY for the HTTP request and are never returned in any status, inspect, log, or error message.

**Endpoint URL secrets:**

Some MCPs put tokens in the URL: `https://example.com/mcp?token=${MCP_SECRET}`. The endpoint itself may contain secret references. Resolution happens at connect time only.

**Status/inspect redaction:**

- `CatalogEntry.Endpoint` in `mcp list`, `environment inspect`, and `Snapshot` may contain env-var references like `${MCP_TOKEN}` — these are safe to display (they're templates, not values).
- The resolved endpoint with actual token values is NEVER returned.
- Error messages from connection failures strip any resolved endpoint before reporting.

**Implementation location:** A new `resolveEndpoint(endpoint string, headerRefs map[string]string)` function in `internal/app/mcp_health.go` or `internal/catalog/resolve.go`. This function:
1. Calls `os.ExpandEnv()` on the endpoint string.
2. Calls `os.ExpandEnv()` on each header ref value.
3. Returns the resolved endpoint + headers.
4. Is called ONLY at the connect boundary.

### 2.6 Structured Error Handling (D-07)

**Error type:**

```go
type MCPError struct {
    MCPID    string `json:"mcp_id"`
    ErrorKind string `json:"error_kind"`  // "not_enabled" | "missing_endpoint" | "connection_refused" | "timeout" | "auth_failure" | "tool_list_failed" | "tool_not_found"
    Message   string `json:"message"`     // Human-readable, no secrets
}
```

- The existing `toolResult(nil, err)` pattern in gateway handlers already propagates errors as MCP tool errors.
- Enrich the error messages to include the MCP ID and error kind.
- Ensure `connectExternalMCP` error messages do NOT include the resolved endpoint URL (which may contain secrets).

---

## 3. File-Level Change Map

### 3.1 Model Changes

**`internal/model/types.go`:**
- Add `Transport string` field to `CatalogEntry` (line ~50, after `Endpoint`).
- Add `HeaderRefs map[string]string` field to `CatalogEntry` (line ~51).
- No changes to `Environment` struct (health is not persisted).

### 3.2 Catalog Changes

**`internal/catalog/service.go`:**
- Modify `AddMCP(name, endpoint string, defaultInclude bool)` signature OR add a parallel method. Recommend adding:
  ```go
  type MCPConfig struct {
      Endpoint      string
      Transport     string            // defaults to "streamable-http"
      HeaderRefs    map[string]string // env-var-referenced headers
      DefaultInclude bool
  }
  func (s *Service) AddMCPConfig(name string, config MCPConfig) (model.CatalogEntry, error)
  ```
- Update `add()` to populate `Transport` and `HeaderRefs`.
- Backward-compat: existing `AddMCP` wraps `AddMCPConfig` with defaults.

### 3.3 New MCP Health Package

**`internal/app/mcp_health.go` (NEW FILE):**

```go
package app

type MCPHealthState string

const (
    MCPHealthConfigured MCPHealthState = "configured"
    MCPHealthDisabled   MCPHealthState = "disabled"
    MCPHealthHealthy    MCPHealthState = "healthy"
    MCPHealthError      MCPHealthState = "error"
)

type MCPHealthStatus struct {
    MCPID     string         `json:"mcp_id"`
    State     MCPHealthState `json:"state"`
    ErrorKind string         `json:"error_kind,omitempty"`
    Message   string         `json:"message,omitempty"`
}

func (s *Service) ProbeMCPHealth(ctx context.Context, environmentID, mcpID string) (MCPHealthStatus, error)
func resolveEndpoint(endpoint string, headerRefs map[string]string) string
func resolveHeaders(headerRefs map[string]string) map[string]string
```

- `ProbeMCPHealth` checks enablement → catalog existence → endpoint → connect → list-tools.
- `resolveEndpoint`/`resolveHeaders` apply `os.ExpandEnv` at activation boundary only.

**`internal/app/mcp_health_test.go` (NEW FILE):**
- Test configured state for unprobed MCP.
- Test disabled state for non-enabled MCP.
- Test healthy state with mock HTTP server.
- Test error states: connection refused, timeout, auth failure, tool-list failure.
- Test secret resolution: env-var references resolved, resolved values not in error messages.

### 3.4 Gateway Enrichment

**`internal/gateway/server.go`:**

1. **New `EnvironmentMCPStatusInput` struct** (alongside existing `EnvironmentMCPRuntimeInput`):
   ```go
   type EnvironmentMCPStatusInput struct {
       EnvironmentID string `json:"environment_id"`
       MCPID         string `json:"mcp_id"`
   }
   ```

2. **New `environment_mcp_status` tool** (or enrich `environment_inspect`):
   - Calls `service.ProbeMCPHealth(ctx, envID, mcpID)`.
   - Returns structured `MCPHealthStatus`.
   - No writer required (read-only probe).

3. **Enrich `environment_mcp_tools` handler** (line 358-383):
   - After `enabledMCPEndpoint`, if connect fails → return structured error with `error_kind`.
   - On success → tools listing proceeds as before.

4. **Enrich `environment_mcp_call` handler** (line 384-400):
   - Same error enrichment pattern.
   - If tool not found on external MCP → structured error with `error_kind: "tool_not_found"`.

5. **Update `connectExternalMCP`** (line 571-578):
   - Accept resolved headers as parameter (or resolve inside).
   - Set resolved headers on `StreamableClientTransport` if the SDK supports it.
   - Ensure error messages do NOT include the endpoint URL.

6. **Update `enabledMCPEndpoint`** (line 553-568):
   - Return structured info including `HeaderRefs` so the caller can resolve secrets.

### 3.5 Management Surface

**`internal/management/service.go`:**

- Add `MCPHealth(ctx, environmentID, mcpID) (app.MCPHealthStatus, error)` method.
- Optionally enrich `Snapshot` to include per-MCP health summary for enabled MCPs (or keep snapshot lightweight and require explicit probe).

**`internal/desktop/adapter.go`:**

- Add `ProbeMCPHealth(environmentID, mcpID) (app.MCPHealthStatus, error)` adapter method.
- Pass through to management service.

### 3.6 CLI Enrichment

**`cmd/ai-dev-manager/main.go`:**

- Add `mcp status --id MCP_ID --environment-id ENV_ID` command.
  - Calls `ProbeMCPHealth` and prints JSON.
  - No writer required.
- Optionally: add `mcp list` output enrichment to show transport type.

### 3.7 Tests

**`internal/gateway/mcp_acceptance_test.go` (NEW FILE):**

Real Streamable HTTP acceptance tests:
1. **Happy path:** Add real external MCP → enable for Environment → `environment_mcp_status` returns `healthy` → `environment_mcp_tools` lists tools → `environment_mcp_call` succeeds.
2. **Health states:** Start with unprobed MCP → status shows `configured` → probe → `healthy` → disable → `disabled`.
3. **Error health:** Point MCP at unreachable endpoint → `environment_mcp_status` returns `error` with `connection_refused`.
4. **Secret non-exposure:** Configure MCP with env-var endpoint → verify inspect/status/logs do not contain resolved secret.
5. **Structured errors:** Bad config → structured error with `error_kind` and no secret leakage.
6. **Negative tests:** `environment_mcp_tools` on disabled MCP → clear error (existing test covers this). `environment_mcp_call` on broken MCP → structured error. Unrelated Gateway tools still work after MCP error.

**`internal/catalog/service_test.go` (extend):**
- Test `AddMCPConfig` with transport + header refs.
- Test backward-compat: existing `AddMCP` defaults transport to `streamable-http`.

**`internal/app/service_test.go` (extend):**
- Test `ProbeMCPHealth` for all 4 states.
- Test `resolveEndpoint` with env-var references.
- Test secret redaction in error messages.

---

## 4. Risk Assessment

### 4.1 Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| MCP Go SDK `StreamableClientTransport` may not support custom headers | Medium | High | Check SDK source; if not, use `http.Client` wrapper or raw HTTP headers on the transport. The SDK v1.7.0 likely supports this since it's HTTP-based. |
| Probe timeout may be too aggressive for slow MCPs | Low | Medium | Make probe timeout configurable or use generous defaults (10s connect + 10s list-tools). This is on-demand, not background. |
| `os.ExpandEnv` may not cover all secret patterns (e.g., Vault, cloud metadata) | Low | Low | D-03 says "e.g., environment variable interpolation" — env vars are sufficient for Phase 3. Richer secret sources are Phase 5+ scope. |
| Existing catalog entries have no `Transport` field | None | None | Empty defaults to `streamable-http`. No migration needed. |

### 4.2 Design Unknowns

1. **Does `mcp.StreamableClientTransport` support custom HTTP headers?** Need to verify against `github.com/modelcontextprotocol/go-sdk v1.7.0`. If not, the plan may need a thin `http.Transport` wrapper or a custom transport type.

2. **Should `environment_mcp_status` be a separate tool or part of `environment_inspect`?** Recommendation: separate tool. `inspect` is already large; a focused health probe is cleaner. But the plan should keep this open for the planner to decide.

3. **Should health be cached within a Gateway process lifetime?** Recommendation: minimal caching. The connect-per-call model is simple and correct. A 30-second cache on probe results could avoid redundant probes within a single Agent interaction, but adds complexity. The plan should favor simplicity (no cache) unless performance requires it.

### 4.3 Dependencies

- **No new Go dependencies.** The MCP Go SDK v1.7.0 already supports Streamable HTTP. No additional libraries needed.
- **No new external services.** Health probing uses the same HTTP client already in use.
- **No new prerequisites.** Phase 3 has no new prerequisites beyond Phase 2.

---

## 5. Validation Architecture

### 5.1 Unit Tests

| Test | Location | What it proves |
|---|---|---|
| `TestAddMCPConfigSetsTransportAndHeaders` | `catalog/service_test.go` | Transport + HeaderRefs persist correctly |
| `TestAddMCPDefaultsTransportToStreamableHTTP` | `catalog/service_test.go` | Backward-compat: empty transport → "streamable-http" |
| `TestProbeMCPHealthConfigured` | `app/mcp_health_test.go` | Unprobed MCP returns "configured" |
| `TestProbeMCPHealthDisabled` | `app/mcp_health_test.go` | Non-enabled MCP returns "disabled" |
| `TestProbeMCPHealthHealthy` | `app/mcp_health_test.go` | Successful connect+list-tools → "healthy" |
| `TestProbeMCPHealthErrorConnectionRefused` | `app/mcp_health_test.go` | Unreachable endpoint → "error" |
| `TestProbeMCPHealthErrorTimeout` | `app/mcp_health_test.go` | Slow/dead endpoint → "error" with timeout kind |
| `TestResolveEndpointExpandsEnvVars` | `app/mcp_health_test.go` | `${VAR}` → resolved value |
| `TestSecretsNotExposedInHealthStatus` | `app/mcp_health_test.go` | Resolved secret not in status JSON |
| `TestSecretsNotExposedInErrorMessages` | `app/mcp_health_test.go` | Resolved secret not in error text |

### 5.2 Integration Tests (Real Streamable HTTP)

| Test | Location | What it proves |
|---|---|---|
| `TestMCPHealthStatusEndToEnd` | `gateway/mcp_acceptance_test.go` | Full probe cycle through Gateway tool |
| `TestMCPToolsWithHealthGating` | `gateway/mcp_acceptance_test.go` | `environment_mcp_tools` returns structured error on bad endpoint |
| `TestMCPCallWithHealthGating` | `gateway/mcp_acceptance_test.go` | `environment_mcp_call` returns structured error on bad endpoint |
| `TestMCPHealthAfterEnableDisable` | `gateway/mcp_acceptance_test.go` | Enable → probe → healthy → disable → disabled |
| `TestUnrelatedGatewayToolsSurviveMCPError` | `gateway/mcp_acceptance_test.go` | Broken MCP does not break `gateway_info`, `read`, etc. |
| `TestMCPSecretResolutionThroughGateway` | `gateway/mcp_acceptance_test.go` | Env-var endpoint resolved at call time, not in inspect |

### 5.3 Acceptance Criteria Mapping

| ROADMAP Success Criterion | Validation |
|---|---|
| 1. Real local external MCP initializes and lists tools | `TestMCPHealthStatusEndToEnd` + existing `TestGatewayUsesOnlyEnabledConfiguredExternalMCPAndSkillRuntime` |
| 2. Environment enable/disable gates activation | Existing test proves this; Phase 3 adds health-aware gating |
| 3. Health distinguishes configured/disabled/healthy/error | `TestProbeMCPHealth*` unit tests + `TestMCPHealthStatusEndToEnd` |
| 4. Missing/bad config reports structured error without secret leakage | `TestSecretsNotExposed*` + `TestMCPToolsWithHealthGating` + `TestMCPCallWithHealthGating` |
| 5. Real external MCP call succeeds end-to-end | Existing test proves this; Phase 3 adds health context |

### 5.4 Exit Criteria

1. `go test ./...` passes.
2. `go vet ./...` passes.
3. `git diff --check` is clean.
4. All `MCP-RUN-01` through `MCP-RUN-05` acceptance tests pass.
5. Real Streamable HTTP acceptance proves a configured MCP returns structured health, lists tools, and calls tools through the ADM Gateway.
6. Error health states return structured errors with no secret leakage.
7. PROJECT/ROADMAP/STATE/verification/UAT/SUMMARY are updated only after implementation evidence exists.

---

## RESEARCH COMPLETE
