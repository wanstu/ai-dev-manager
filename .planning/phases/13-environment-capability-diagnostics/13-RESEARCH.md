# Phase 13 Research — Environment Capability Diagnostics

## Current inspection path

`internal/app/service.go` exposes `InspectEnvironment`. It resolves:

- the Environment;
- its Workspace;
- `Capabilities(ctx, env.ID)` from a concrete Runtime;
- enabled/unresolved MCP selections;
- enabled/unresolved Skill selections.

The important weakness is ordering: Runtime construction/validation happens before the MCP/Skill catalog resolution. A missing/tampered managed worktree or invalid root can therefore make the whole Environment inspection fail even though some global capabilities could still be meaningfully diagnosed.

## Required architectural shift

Capability inspection should aggregate independent facts instead of constructing one all-or-nothing Environment truth.

Recommended core types:

```text
CapabilityState
  available
  disabled
  unconfigured
  unavailable
  degraded

CapabilityFact
  key
  kind
  state
  reason_code
  message
  requires_writer
  evidence[]

CapabilityEvidence
  type
  id
  name
  path?        # only explicit registered/configured path facts
  detail?      # sanitized
```

The exact JSON may differ, but one type must be reused by Gateway/CLI/Desktop later.

## Capability groups

### Base file/runtime

Evaluate granular facts independently where possible:

- files.read/tree/search;
- files.write/edit/delete;
- exec;
- generic async run/process support.

If Environment root/runtime validation fails, return those facts as unavailable with the real validator reason rather than failing the entire report.

Writer-gated mutation facts carry `requires_writer=true` plus current lease metadata. Inspection does not acquire/steal the lease.

### Verifiers

For each configured verifier report at least:

- enabled/disabled;
- executable/cwd configuration facts;
- whether required executable authority is currently configured;
- writer requirement.

Do not execute the verifier during capability inspection.

### Git/isolation

Use the same Git support and managed-worktree validation logic as real Runtime/isolation operations. A non-Git root should be normal `unavailable/not_repository`, not Environment error.

### MCP

Consume Phase 11 desired and owner-local observed status:

- selected/disabled/unresolved;
- transport/config readiness;
- current observation if any;
- latest structured runtime error;
- tool inventory count/fetch time if observed.

Default capability inspection does not reconnect/probe. Explicit Phase 11 refresh remains the operation for that.

### Skill

Consume Phase 12 source-aware Environment availability:

- enabled/disabled/unresolved;
- source/artifact/support availability;
- structured reason/evidence.

Do not parse Skill prose.

## Application vs Gateway observation

Desired/configuration facts belong in application services and can be used by CLI/management without a live Gateway owner.

Some observations are owner-local by design, especially MCP session/tool inventory and running proc/run facts. The Gateway should enrich the same `CapabilityFact` model with owner observation rather than inventing a separate response shape.

The product truth is therefore one schema with two evidence origins:

- desired/static application facts;
- current owner observation when available.

Absence of owner observation must be reported as `not_observed`/similar, not incorrectly converted to unhealthy.

## Failure isolation

The aggregator should collect component errors and convert them to facts. The top-level operation should fail only when the requested Environment itself cannot be resolved or the underlying ADM state cannot be read at all.

Examples:

- broken HTTP MCP → only that MCP fact degraded/unavailable;
- missing Skill source → only affected Skill/source facts unavailable;
- no Git → Git unavailable, files still available;
- invalid managed worktree → root-dependent capabilities unavailable, but catalog/global diagnostics still returned;
- no writer → read capabilities remain available and mutation facts state writer requirement/current lease.

## Evidence safety

Permitted evidence examples:

- Workspace/Environment/MCP/Skill/verifier/managed-worktree IDs;
- registered root or explicitly configured artifact/source path;
- executable name/path already present in allowlist/config;
- transport kind;
- reason code/timestamp/tool count.

Forbidden evidence:

- resolved MCP header/env secret values;
- OAuth/token values;
- Environment-private Memory values;
- arbitrary host filesystem scan results.

## Main risks

1. **second truth model** — reuse operation validators instead of duplicating loose checks.
2. **inspection side effects** — no command/process/verifier/tool call during default report.
3. **global failure** — aggregate per-capability errors; top-level should remain useful.
4. **stale observation presented as fact** — timestamps/origin are explicit.
5. **secret leakage through evidence** — response model and tests must sanitize by construction.
