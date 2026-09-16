# Phase 03: External MCP Runtime Completion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-07
**Phase:** 03-external-mcp-runtime-completion
**Areas discussed:** Transport Scope, Connection & Session Model, Secret Handling, Health & Activation Lifecycle

---

## Transport Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Streamable HTTP only | First runtime transport as ADM-CORE-012 specifies | ✓ |
| Streamable HTTP + stdio | Add both HTTP and local child-process transport | |
| Transport-agnostic | Abstract transport layer from day one | |

**User's choice:** Streamable HTTP only — ADM-CORE-012 explicitly says "the first runtime transport is a Streamable HTTP endpoint." Do not add stdio in Phase 3; broader transports deferred unless later dogfood proves need.
**Notes:** Transport field should remain extensible (discriminated) for future variants, but implementation scope is HTTP only.

---

## Connection & Session Model

| Option | Description | Selected |
|--------|-------------|----------|
| On-demand per-operation | Connect → operation → close; no persistent sessions | ✓ |
| Connection pooling | Maintain warm connections across operations | |
| Persistent session ownership | Long-lived sessions with lifecycle management | |

**User's choice:** On-demand per-operation. Do not add persistent connection pooling, session ownership, or background process ownership. These belong to Phase 5 (LIFE-01..03). Phase 3 uses bounded on-demand initialization/health probes and per-operation Streamable HTTP sessions while defining correct health semantics.
**Notes:** The on-demand model is the smallest model satisfying MCP-RUN-01..05 without preempting Phase 5.

---

## Secret Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Env-var interpolation in endpoint | `$ENV_VAR` syntax resolved at activation boundary only | ✓ |
| Plain-text endpoint only | Current model, no secret handling | |
| Vault/external secret store | External secret resolution | |

**User's choice:** MCP-RUN-05 is locked. Secret values resolved only at activation boundaries, not exposed in normal status/log output. Minimal env-var interpolation mechanism for Streamable HTTP endpoints; resolved secrets not persisted.
**Notes:** Status/inspect output must redact or omit resolved secret values.

---

## Health & Activation Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| On-demand probe (configured/disabled/healthy/error) | Derived from connect-and-list-tools evidence | ✓ |
| Periodic background health | Background goroutine monitoring health | |
| Lazy on first call only | Probe once on first call, cache forever | |

**User's choice:** Health must distinguish configured/disabled/healthy/error based on actual runtime initialization/probe evidence, not configuration existence. Avoid periodic/background health ownership (Phase 5+). Phase 3 health is on-demand: successful connect+list = healthy; failed = error; not enabled = disabled; no probe yet = configured.
**Notes:** The four-state model is the smallest satisfying MCP-RUN-03 without preempting background monitoring.

---

## Deferred Ideas

- stdio/command-based MCP transport (ADM-CORE-012 first transport = HTTP)
- Persistent connection pooling / session ownership (Phase 5, LIFE-01..03)
- Periodic/background health monitoring (Phase 5+)
- Desktop MCP health UI beyond basic inspection
