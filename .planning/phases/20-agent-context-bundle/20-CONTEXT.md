# Phase 20 — Agent Context Bundle + Capability Injection

Date: 2026-09-12
Inspected baseline: `89be773`, master, clean.
Status: 20-01 Core complete and validated; 20-02 ready, not started.

## Goal and contract mapping

Give an Agent one compact, bounded, read-only Environment context snapshot so it does not need to probe tree, capability, MCP, Skill and verifier surfaces separately before beginning ordinary development.

This refines ADM-GOAL-001/002, ADM-CORE-002/003/004/005/012/017/019/020, ADM-GW-001/003 and ADM-NONGOAL-001. No new prerequisite is introduced.

Phase 19 supplies the bounded Environment directory digest. Phase 13 supplies static and Gateway-owner capability facts. Phase 11/12 supply MCP/Skill desired/runtime facts. Phase 20 composes those existing authorities; it does not create a second capability model.

## Injection semantics

ADM has no implicit per-session "current Environment". Agent tools intentionally route by stable `environment_id`, and MCP initialization does not provide an authorized Environment identity. Phase 20 therefore must not add hidden session selection state just to push dynamic project data during initialize.

"Capability injection" in this phase means two things:

1. the Agent Gateway advertises static, non-project-specific instructions that tell clients to use stable IDs and obtain one Environment context bundle before work when useful; and
2. an explicit read-only `environment_context_bundle(environment_id, ...)` returns the dynamic Environment-specific snapshot.

The static instructions never contain project paths, catalog values, Memory, secret references or owner observations. Dynamic context is returned only after an explicit stable Environment ID is supplied.

## Bundle contents

The compact bundle should contain:

- Environment and Workspace stable IDs/names plus the authorized Environment root identity;
- a bounded Phase-19 Environment tree digest and its coverage/truncation facts;
- compact capability availability and unavailable/degraded reason summaries, preserving `requires_writer` semantics;
- Environment-enabled MCP identity/state summaries, with Gateway-owner observed tool-name inventory only when such an observation already exists;
- Environment-enabled Skill availability/support summaries, without reading or embedding `SKILL.md` contents;
- verifier identity/kind/availability summaries without executing them;
- generic file/mutation/verifier/run/MCP/Skill usage guidance expressed as ADM tool/capability facts, not task steps;
- generated/observed timestamps, effective budgets, omissions and uncertainties sufficient to distinguish complete, partial and not-observed information.

## Explicit boundaries

1. **Stable identity remains mandatory.** No implicit current Environment, path-only authorization or fallback from Environment to parent Workspace.
2. **Read-only and side-effect-free.** No writer lease, state mutation, MCP connect/probe/reconnect, MCP business-tool call, Skill refresh or explicit Skill content-read operation, verifier execution, process/run start, Git command, Environment creation or Memory mutation. Existing Skill availability checks may inspect configured artifact readability, but bundle composition never returns Skill contents.
3. **No automatic background scan.** The bounded directory digest occurs only when the explicit bundle operation is called. No cache/index/watcher is introduced.
4. **No secret or Memory value injection.** MCP endpoint/header/env secret values are never returned. Global Memory values and Environment-private Memory values are not included in the default bundle. Existing explicit Memory tools remain authoritative. The existing private-memory count may remain visible through Environment summary semantics, but Phase 20 does not silently compose Memory text into Agent context.
5. **No full Skill instruction injection.** Report enabled Skill identity, availability and bounded support facts; the consuming Agent still uses the existing explicit Skill read tool to consume instructions/content when needed.
6. **No live probe disguised as context.** MCP tool names/health may be included only from existing Gateway-owner observations. Missing observation is reported as `not_observed`; building a bundle must not connect merely to improve the snapshot.
7. **Optional failure stays local.** Missing Git/verifier/MCP/Skill/executable is represented as a capability state/reason and does not make the Environment bundle unavailable when root authority is otherwise valid.
8. **Managed-root validation remains authoritative.** A missing/tampered managed worktree or invalid Environment root fails before a tree digest is returned.
9. **Bound output.** Tree traversal and serialized bundle size have defaults/hard caps. Section lists have deterministic caps and explicit omitted counts; no arbitrarily large catalog/tool inventory is emitted.
10. **No orchestration.** Guidance may explain which ADM operation is available and whether writer authority is required; it must not choose a task, sequence a plan, select an MCP tool for a business goal, interpret Skill instructions or decide Git integration.
11. **No Desktop/CLI scope expansion in Phase 20.** The primary consumer is the Agent Gateway. Script-friendly CLI context output remains a Phase 23 direction; Desktop does not need a new context-bundle page.

## Proposed request and budgets

Request fields: `path` (optional Environment-relative digest path), `max_depth`, `max_entries`, `max_digest_entries`, `max_output_bytes`. Zero means default; negative/above-hard-cap values fail clearly.

| Budget | Default | Hard cap |
|---|---:|---:|
| tree depth | 3 | 8 |
| tree entries visited | 1000 | 10000 |
| digest entries returned | 80 | 500 |
| serialized bundle bytes | 65536 | 262144 |

Minimum output budget: 4096 bytes. Non-tree sections use deterministic internal caps initially: 64 capability issues, 64 MCP summaries, 128 observed MCP tool names total, 64 Skill summaries and 32 verifier summaries. Any omitted items are counted in the response. These constants are implementation details unless dogfood proves callers need knobs; avoid turning one-call context into a large configuration surface.

## Delivery order

20-01: shared model/application bundle composition over existing Environment inspection, capability and Phase-19 digest authority, with deterministic compaction and negative acceptance.

20-02: Gateway-owner enrichment, Agent/Admin MCP read-only tool, static MCP server instructions and integrated acceptance. No CLI/Desktop feature work in this phase.

## Source calibration

- `internal/app/service.go` already exposes sanitized `EnvironmentInspection`, resolved selections and private Memory count.
- `internal/app/capability_report.go` provides operation-local static capability facts without executing optional tools.
- `internal/gateway/capability_report.go` already enriches MCP/process/run/provider facts from passive Gateway-owner observations.
- `internal/model/workspace_discovery.go` and `EnvironmentTreeDigest` provide bounded metadata-only directory evidence.
- MCP SDK `ServerOptions.Instructions` is static and therefore suitable only for generic Agent usage guidance, not Environment-specific dynamic data.

On implementation resume: inspect Git/AGENTS/STATE, read this context and 20-01-PLAN, acquire a fresh writer, and preserve master/local-commit/no-push/no-subagents rules. Do not start 20-02 before 20-01 acceptance is recorded.
