# Phase 03 — Management CLI parity

Status: complete.

## Requirement IDs

This phase implements the human management CLI slice of:

- ADM-CORE-012
- ADM-CORE-013
- ADM-CORE-015
- ADM-DEV-001
- ADM-DEV-003

## Goal

Expose already-persisted ADM development-context management capabilities through the human-facing CLI instead of leaving them discoverable only through MCP.

The CLI must reuse the same application services and state as MCP. This phase does not introduce a second management model or persistence path.

## Delivery slices

### Slice A — Global MCP / Skill catalogs

Status: delivered.

- `mcp list`
- `mcp add`
- `mcp remove`
- `mcp set-default`
- `skill list`
- `skill add`
- `skill remove`
- `skill set-default`
- top-level and subcommand help expose both catalogs

### Slice B — Global Memory

Status: delivered.

- list
- read
- write
- delete

### Slice C — Environment MCP / Skill selections

Status: delivered.

- enable / disable global MCP IDs for one Environment
- enable / disable global Skill IDs for one Environment
- changes remain scoped to the selected Environment

### Slice D — Environment-private Memory

Status: delivered.

- list
- read
- write
- delete by explicit Environment ID

## Acceptance tests

The phase is complete when automated tests prove that:

- MCP and Skill catalog CRUD/default changes made through CLI persist through the same services used by MCP
- catalog help is discoverable from top-level CLI help
- invalid required arguments fail clearly
- Global Memory CLI writes do not become Environment-private writes
- Environment-private Memory cannot be read through another Environment ID by default
- changing Environment MCP/Skill selections does not change another Environment or global catalog defaults
- existing non-Git development tests continue to pass

## Explicit non-goals

This phase does not implement:

- a new REST management API
- desktop UI
- Git worktree lifecycle
- isolation/orchestration
- verifier configuration
- process/log/port management
- V1 compatibility or migration

## New prerequisites

None.

The CLI reuses existing local application services and persisted state. MCP, Git, shell execution, verifier, worktree, and UI are not prerequisites for management CLI operations that do not use them.
