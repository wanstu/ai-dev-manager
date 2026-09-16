# Phase 11 Validation — MCP Runtime Completion

## Validation principle

MCP completion is accepted only through real runtime consumption. Catalog CRUD or unit-only transport validation is insufficient.

## Required proof matrix

### P1 — typed desired configuration

Prove valid `streamable-http` and `stdio` definitions persist with only their allowed fields. Invalid endpoint, missing stdio executable, mixed transport fields, unsupported transport and unsupported auth are rejected before activation.

Negative requirement: MCP configuration must not persist resolved secret values.

### P2 — Streamable HTTP remains real

A real Streamable HTTP MCP server must initialize/negotiate through the repository's current official Go SDK, list tools and execute a tool through ADM. Existing Environment enable/disable gating remains authoritative.

### P3 — real stdio MCP

Start a real helper MCP as a child process through the Gateway owner using `stdio`, list at least one tool and call it successfully from a later Agent client.

Negative requirements:

- an executable not in ADM's allowlist cannot be activated as stdio MCP;
- escaped/arbitrary cwd is not accepted; stdio process cwd is the selected Environment root;
- disabling/removing the MCP or closing the Gateway terminates the owned child process.

### P4 — secrets stay secret

Use a sentinel secret supplied through an environment reference. Prove activation succeeds while the sentinel never appears in:

- persisted ADM state;
- MCP definition/status output;
- runtime observation;
- normal error text/log output.

Missing secret ref must return a structured configuration/activation error without exposing unrelated environment variables.

### P5 — real tool inventory and refresh

Prove tool inventory is fetched from the actual server, includes a fetch timestamp/count/public tool metadata, and is invalidated/refreshed explicitly.

Change the helper MCP inventory between refreshes and prove `environment_mcp_refresh` exposes the new inventory rather than stale owner state.

Environment disable must immediately remove access and observed inventory.

### P6 — periodic health check + configurable automatic reconnect

Use a real helper MCP with protocol Ping support and machine-observable connection count.

Prove:

- `health_check_enabled=false` produces no background Ping loop while explicit inspect/refresh remains available;
- with health checks enabled, configured `check_interval_seconds` drives background Ping checks only while the MCP is enabled, and changing the interval changes subsequent scheduling without a Gateway restart;
- `probe_timeout_seconds` bounds one health probe;
- failed Ping marks the observation unhealthy and removes stale healthy state;
- omitted/new definitions default `auto_reconnect` to `false`;
- when `auto_reconnect=true`, reconnect attempts occur on the configured fixed `reconnect_interval_seconds` while unhealthy;
- reconnect timing does not silently switch to exponential/adaptive backoff;
- when `auto_reconnect=false`, ADM remains unhealthy until an explicit safe refresh/list operation reconnects;
- successful background reconnect restores healthy observation and refreshes inventory without any Agent client needing to stay connected;
- at most one reconnect is in flight per Environment/MCP;
- disabling/removing/updating the MCP cancels pending recovery so it cannot resurrect a revoked/old definition.

### P7 — lifecycle and safe reconnect / no tool replay

Prove:

- broken connection becomes structured error, never stale healthy;
- subsequent safe status/list/refresh or configured background recovery can establish a new connection after server recovery;
- a failed tool call invalidates the bad connection;
- ADM does not automatically replay that tool call before or after reconnect.

Use a helper tool that records invocation count so automatic replay would be machine-detectable.

### P8 — restart semantics

With an enabled MCP, stop/restart the real Gateway owner. Desired config/Environment selection and persisted health policy survive; old observed session/inventory/health timestamps do not. The restarted owner resumes monitoring/reconciliation from desired policy rather than serializing stale observation.

### P9 — optional failure is local

With one broken MCP and one healthy MCP in the same Environment, prove:

- healthy MCP remains callable;
- file read/search remains usable;
- Skill list/read remains usable;
- verifier/file/runtime capabilities are not globally marked failed.

### P10 — external JSON/JSONC import adapters

Use committed deterministic fixtures for each supported adapter and prove both single-candidate and batch import:

- OpenCode current `mcp.servers` local + remote forms;
- WorkBuddy/CodeBuddy `mcpServers` stdio + Streamable HTTP forms, including extension-field warnings;
- Codex plugin `.mcp.json` wrapped `mcpServers` and direct top-level server-map forms;
- Claude Code project `.mcp.json` plus a full JSON `projects.<path>.mcpServers` source selected explicitly;
- MCPHub standard `mcpServers` and hub-oriented `servers` map forms.

For every source, preview must show canonical normalized candidates before any persistence.

Negative requirements:

- `auto` detection that is ambiguous returns `ambiguous_format` instead of guessing;
- duplicate candidate names or unsupported mandatory source fields are explicit errors/warnings according to adapter policy;
- default name conflict policy is `error`; no silent overwrite/suffixing;
- explicit batch apply is atomic: if one selected candidate is invalid, none of the selected candidates persist;
- explicit `update_by_name` preserves ADM MCP identity and current Environment selections but invalidates old owned session/inventory;
- source enabled/disabled flags do not silently enable/disable ADM Environments, and Phase 11 import has no target-Environment apply mode;
- recognized `{env:VAR}`, `${VAR}` and supported template forms remain references during import and are not resolved by preview;
- literal credential-bearing values are converted into generated secret/environment-reference requirements; preview exposes only reference names + field paths, never the literal, and apply persists only those references;
- generated references must be provisioned before runtime activation can become healthy;
- raw import payloads do not appear in logs/status/errors;
- importing one bad candidate does not damage unrelated existing MCP definitions.

## Automated gates

- focused catalog/config tests;
- focused HTTP/stdio MCP runtime tests;
- fake-clock / bounded-time health-policy tests for Ping scheduling, probe timeout and reconnect interval;
- race tests for owner session/refresh/health/reconnect/drop paths;
- table-driven JSON/JSONC importer fixtures for all supported source adapters;
- real Streamable HTTP acceptance;
- real stdio child-process acceptance;
- real unhealthy→automatic-reconnect acceptance;
- Gateway restart acceptance;
- `go test ./...` or equivalent complete package-group coverage if the tool transport cannot observe one long aggregate call;
- `go vet ./...`;
- `git diff --check`.

## Review focus

Integration review must explicitly inspect:

- no secret values in persisted/returned structures;
- no automatic tool-call replay before/after background reconnect;
- health monitor cannot resurrect a disabled/removed/old MCP definition;
- health/reconnect observations are not persisted as desired state;
- no stdio exec-policy bypass;
- no legacy SSE implementation;
- importer adapters normalize into canonical ADM types rather than leaking external schemas into Core;
- no silent overwrite, partial selected-batch apply, external enable-state mutation, or literal credential persistence during import;
- no Planner/GSD orchestration reintroduction;
- no MCP failure promoted into a global Environment prerequisite.
