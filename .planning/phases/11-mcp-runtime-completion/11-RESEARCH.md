# Phase 11 Research — MCP Runtime Completion

## Current implementation audit

### Persisted model

`internal/model/types.go` currently stores MCP and Skill in the same `CatalogEntry`. MCP-specific fields are `Endpoint`, `Transport`, and `HeaderRefs`; Skill-specific fields live beside them. This makes desired MCP configuration difficult to validate as a transport-specific contract and encourages unrelated MCP/Skill changes to share one persistence type.

`internal/catalog/service.go` currently:

- defaults transport to `streamable-http`;
- accepts only HTTP(S) endpoints;
- rejects every transport other than `streamable-http`;
- stores environment-variable-like header references;
- provides global catalog/default-selection behavior.

Because compatibility is not a locked requirement, Phase 11 can introduce a typed MCP definition instead of preserving the generic transport fields indefinitely.

### Activation and transient health

`internal/app/mcp_health.go` already separates activation from persisted config in a useful way:

- Environment selection is checked first;
- missing catalog entries are reported as configured/unresolved;
- secret/header references resolve only at activation;
- on-demand probes are bounded;
- errors are classified into timeout, refused, auth and connection categories.

The current activation type is HTTP-specific (`Endpoint`, `Headers`) and needs to become transport-specific without leaking resolved secrets.

### Persistent Gateway owner

`internal/gateway/runtime_owner.go` owns MCP runtime state by `(environment_id, mcp_id)`. It already:

- reuses live sessions;
- verifies a session using `ListTools`;
- drops failed sessions;
- closes sessions on MCP disable/removal, Environment drop and Gateway shutdown;
- reconstructs desired runtime from persisted Environment selections after restart;
- keeps observed health owner-local rather than persisting it.

This is the correct owner boundary for Phase 11. Do not create a second MCP daemon.

### Missing runtime concepts

The current owner does not expose a rich observed record containing connection stage, timestamps or tool inventory. `ListTools` is both health check and inventory fetch, but there is no explicit refresh operation or auditable inventory status.

`CallTool` drops a session after failure. This is correct for future recovery, but any automatic immediate retry of the tool call would be unsafe because tool calls can mutate external systems. Phase 11 must explicitly forbid automatic replay.

## MCP protocol / SDK facts

Repository dependency: `github.com/modelcontextprotocol/go-sdk v1.7.0`.

The official Go SDK compatibility table states `v1.7.0+` supports MCP specification `2026-07-28` while retaining older supported protocol revisions. The current MCP specification deprecates legacy HTTP+SSE. The SDK also exposes `CommandTransport` for starting a local command and communicating over stdin/stdout.

Therefore Phase 11 does not need an SDK upgrade merely to speak the current protocol and should not add deprecated SSE just to claim transport breadth.

References:

- https://github.com/modelcontextprotocol/go-sdk
- https://blog.modelcontextprotocol.io/posts/2026-07-28/

## Recommended desired model

Introduce an MCP-specific persisted type rather than adding more fields to generic `CatalogEntry`.

Conceptual shape:

```text
MCPDefinition
  id
  name
  default_include_in_environment
  transport: streamable-http | stdio
  auth_mode: none | headers
  endpoint                 # streamable-http only
  header_refs              # streamable-http / headers auth only
  executable               # stdio only
  args                     # stdio only
  env_refs                 # stdio optional activation refs
```

Validation must be transport-local:

- HTTP requires valid http(s) endpoint and forbids stdio fields.
- stdio requires an executable and forbids HTTP endpoint/header fields.
- unsupported transport/auth is rejected before persistence/activation.
- secret refs store reference names/templates, not resolved values.

Stdio activation should use the selected Environment root as process cwd. The executable must pass the same executable allowlist authority as normal Runtime command execution so `mcp_add` cannot become an arbitrary command launcher.

## Recommended observed model

Owner-local runtime observation should include enough facts for diagnostics without becoming persisted desired state:

```text
MCPRuntimeObservation
  mcp_id
  environment_id
  state
  transport
  stage: activation | connect | discover | list_tools | call
  error_kind
  message
  last_checked_at
  last_success_at
  tool_inventory
  inventory_fetched_at
```

Tool inventory should contain public tool metadata already returned by MCP and remain bounded. No header/env secret values belong in this record.

## Refresh/reconnect semantics

- `environment_mcp_refresh` is explicit and safe: close/drop current observation, activate again, fetch real tool inventory.
- `environment_mcp_tools` may ensure a usable connection and fetch tools if inventory is absent/stale.
- failed safe discovery/list operations may reconnect on a subsequent request.
- failed `environment_mcp_call` drops the connection but never silently retries the same tool invocation.
- Environment disable/remove immediately invalidates the session/process and inventory.
- Gateway restart reconstructs from desired config; old observations are not treated as current truth.

## Auth boundary

Current environment-variable-backed HTTP headers are retained as the supported secret mechanism. Interactive OAuth requires a secure token/credential lifecycle that ADM does not currently have. Phase 11 must report unsupported auth explicitly rather than store refresh/access tokens insecurely or invent a half-OAuth implementation.

## Health monitoring / automatic reconnect

The official Go SDK exposes `ClientSession.Ping`, so Phase 11 can use the MCP protocol heartbeat instead of repeatedly calling `tools/list` merely to test liveness.

Recommended persisted policy per MCP definition:

```text
MCPHealthPolicy
  health_check_enabled
  check_interval_seconds
  probe_timeout_seconds
  auto_reconnect
  reconnect_interval_seconds
```

Recommended owner-local observation additions:

```text
last_check_at
last_healthy_at
consecutive_failures
next_reconnect_at
reconnect_in_flight
```

Semantics:

- health monitoring runs only for enabled MCPs under the persistent Gateway owner;
- a live session is probed with protocol Ping on the configured interval;
- probe timeout/failure marks the session unhealthy and drops stale healthy observation;
- when `auto_reconnect=true`, an unhealthy/missing session is retried at the configured reconnect interval;
- at most one probe/reconnect is in flight for one `(environment_id, mcp_id)`;
- disable/remove/config change invalidates pending reconnect generation/state;
- successful reconnect may refresh tool inventory;
- failed tool calls are never automatically replayed, even if reconnect succeeds immediately afterwards.

This is the first use of a reusable ADM health/recovery pattern. Future long-lived capabilities (for example an explicitly opt-in dev-process restart policy) may adopt a similar shape later, but Phase 11 must not prematurely create one generic retry engine for all operations.

## JSON / JSONC import format audit

Import is deliberately an edge adapter. ADM should have one canonical `MCPDefinition` regardless of source format.

### OpenCode

Current OpenCode V2 config uses `mcp.servers`. Local servers use `type: "local"` with a command array plus optional `cwd`/`environment`; remote servers use `type: "remote"`, `url`, optional headers, disabled flag and timeout. OpenCode configuration is JSON/JSONC and supports `{env:NAME}` references.

Importer mapping:

- `mcp.servers.<name>` -> one candidate;
- local `command: [exe, ...args]` -> ADM stdio executable + args;
- `environment` -> stdio env configuration/references;
- remote `url` -> Streamable HTTP endpoint;
- `headers` -> HTTP header config/references;
- source `disabled` is advisory only and does not silently rewrite ADM Environment selection.

### WorkBuddy / CodeBuddy

Current WorkBuddy/CodeBuddy MCP JSON uses an `mcpServers` object. Documented local entries use `type: "stdio"`, `command`, `args`, `env`; remote entries may use `http`/`streamableHttp` plus `url`/`headers`. WorkBuddy connector JSON can include extension fields such as `runtime`, `npmRegistry`, timeout and `x-workbuddy` metadata.

Importer mapping keeps the standard connection fields and reports unsupported WorkBuddy-only extensions as preview warnings. ADM must not silently emulate package/runtime installation behavior from those extension fields.

### Codex JSON

Codex native user MCP configuration is TOML and is not part of this JSON-import requirement. Codex plugin MCP configuration does use `.mcp.json`; current plugin parsing accepts an `mcpServers` wrapper and also a direct top-level server map in supported plugin paths. Server entries cover stdio command/args/env/cwd and Streamable HTTP URL/headers.

Importer therefore supports Codex plugin MCP JSON, not native `~/.codex/config.toml` in Phase 11.

### Claude Code

Claude Code project/plugin `.mcp.json` uses an `mcpServers` wrapper. Its user/local configuration may also contain project-scoped `projects.<path>.mcpServers` maps. HTTP accepts `http`/`streamable-http`; stdio uses command/args/env/cwd. `${VAR}` and `${VAR:-default}` expansion is documented.

Importer supports:

- direct/project `.mcp.json` `mcpServers`;
- full Claude JSON project maps with an explicit project/scope selector in preview/apply;
- recognized environment placeholders are preserved as references/templates, not resolved during import.

### MCPHub

MCPHub implementations in current use have at least two relevant JSON shapes: a standard `mcpServers` wrapper and a hub-oriented `servers` map (for example `~/.mcphub.json`) containing type/enabled/command/args/env/timeout fields. Phase 11 supports these known source shapes through an MCPHub adapter rather than assuming one universal MCPHub schema.

### Import API semantics

Recommended management/application boundary:

```text
MCPImportPreview(format, json_or_jsonc, options) -> candidates + warnings/errors
ApplyMCPImport(format, json_or_jsonc, selected_names, conflict_policy, options) -> atomic result
```

Supported format identifiers:

```text
auto
opencode
workbuddy
codex-plugin
claude-code
mcphub
```

`auto` must be deterministic. If two adapters plausibly match and normalize differently, preview returns `ambiguous_format` and requires an explicit format instead of guessing.

Single import is a selected one-server candidate. Batch import selects many/all candidates from the same payload. Selected batch apply is atomic: one invalid selected candidate means no selected definitions are persisted.

Conflict policy defaults to `error`. Optional explicit policies may include `skip` and `update_by_name`. `update_by_name` preserves the ADM MCP ID and Environment selections, then invalidates any owned runtime session/inventory so new configuration becomes authoritative.

External enabled/disabled flags never silently mutate ADM Environment selections. Import options explicitly decide `default_include` and any target-Environment enabling behavior.

Import secret handling is conservative:

- raw import payloads are not logged or persisted;
- recognized env-reference syntaxes are normalized without resolution;
- known credential-bearing literal values are reported as blocked candidates requiring an explicit secret/reference mapping before apply;
- import preview/status never echoes resolved secret material.

## Import source references

Current format references used to design the adapters:

- OpenCode MCP servers: `https://opencode.ai/v2/docs/mcp-servers`
- WorkBuddy connector `mcp.json`: `https://open.workbuddy.cn/docs/connector`
- WorkBuddy/CodeBuddy CLI MCP config: `https://www.workbuddy.cn/docs/cli/mcp`
- Claude Code MCP JSON/scopes: `https://code.claude.com/docs/en/mcp`
- Codex plugin MCP JSON implementation/docs: `https://github.com/openai/codex/blob/main/codex-rs/codex-mcp/src/agent_plugin_config.rs`
- MCPHub examples vary by implementation; adapters are fixture-based for the explicitly supported `mcpServers` and `servers` shapes rather than treating one third-party schema as universal.

These are importer-edge compatibility facts only. ADM's canonical desired/runtime models remain source-independent.

## Main risks

1. **stdio as exec bypass** — mitigate by requiring executable allowlist authority.
2. **secret leakage** — test all status/error/management output for resolved secret absence.
3. **unsafe retry** — never replay a tool call automatically.
4. **stale healthy/inventory** — drop observation on transport error, disable/remove and restart.
5. **optional-capability coupling** — one broken MCP must not block files/Skill/other MCP operations.
6. **model churn** — because this is pre-stable development, prefer a clean typed MCP model over compatibility fields.
