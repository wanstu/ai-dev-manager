# Phase 04 — Environment management UX

Status: complete.

## Requirement IDs

This phase implements or refines:

- ADM-CORE-002
- ADM-CORE-016
- ADM-CORE-017
- ADM-DEV-001
- ADM-DEV-003

## Goal

Make Environment lifecycle management as explicit and safe as Workspace lifecycle management without changing the Environment model or introducing isolation semantics.

## Delivery slices

### Slice A — Environment rename

Status: delivered.

- rename an Environment by stable ID
- rename changes only the human-readable name
- preserve Workspace ID, root, selections, private memory, writer state, and creation time
- reject an empty replacement name
- do not move, rename, or delete project files
- expose the operation through CLI and MCP

### Slice B — Environment inspection ergonomics

Status: delivered.

- `environment list` returns lightweight summaries instead of dumping private Memory values
- summaries expose `private_memory_count`
- `environment inspect` exposes the referenced Workspace and current capabilities
- inspect resolves selected MCP/Skill IDs to catalog entries when they still exist
- removed catalog selections remain explicit as unresolved IDs
- inspect exposes private Memory count without private Memory values
- CLI and MCP reuse one application-level management view
- no second persistence or product model is introduced

## Acceptance tests

- rename preserves stable identity and root
- rename preserves MCP/Skill selections and private memory
- rename preserves an active writer lease
- empty names fail clearly
- project files are untouched
- CLI help exposes rename
- MCP discovery exposes `environment_rename`
- list/inspect never dump private Memory values
- list/inspect expose private Memory entry count
- inspect exposes Workspace metadata and current capabilities
- inspect resolves existing MCP/Skill selections and preserves removed selections as unresolved IDs
- CLI and MCP return the same application-level management view
- existing non-Git development tests continue to pass

## Explicit non-goals

- Git worktree lifecycle
- branch management
- isolation
- task/session attribution
- orchestration
- desktop UI
- V1 compatibility or migration

## New prerequisites

None.
