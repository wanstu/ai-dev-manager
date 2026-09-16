# Phase 02 — Development Context

## Requirement IDs

This phase implements the first usable development-context slice of:

- ADM-GOAL-001
- ADM-CORE-002
- ADM-CORE-003
- ADM-CORE-004
- ADM-CORE-012
- ADM-CORE-013
- ADM-GW-001
- ADM-GW-002
- ADM-GW-003
- ADM-DEV-001
- ADM-DEV-003
- ADM-DEV-004

## Delivered behavior

- Global MCP catalog supports add, list, remove, and default-include changes.
- Global Skill catalog supports add, list, remove, and default-include changes.
- New Environments copy the catalog defaults that exist at creation time.
- Later global default changes do not rewrite existing Environment selections.
- Each Environment can independently enable or disable global MCP and Skill IDs.
- Removing a global catalog entry does not silently recreate, copy, or erase an existing Environment ID reference; the unresolved reference remains explicit.
- Global Memory supports list/read/write/delete.
- Environment-private Memory supports list/read/write/delete and is addressed by explicit `environment_id`.
- Memory writes always target an explicit global or Environment scope; there is no implicit synchronization or promotion.
- `environment_inspect` exposes the Environment model, including enabled MCP IDs, enabled Skill IDs, private-memory state, writer state, activity fields, root, and current Runtime capabilities.
- Agent Gateway exposes management tools for the MCP catalog, Skill catalog, Environment selections, and both Memory scopes.
- Agent Gateway additionally exposes `workspace_add`, `exec_allow`, and `exec_allow_list` so a fresh V2 Gateway can bootstrap a real local development loop without an out-of-band management API.

## Negative acceptance tests

Automated tests prove that:

- a project with no MCP catalog entries can still be developed
- a project with no Skill catalog entries can still be developed
- an Environment with no Memory data can still be developed
- changing global MCP defaults does not rewrite an existing Environment selection
- changing global Skill defaults does not rewrite an existing Environment selection
- enabling a selection for one Environment does not change another Environment
- Environment-private Memory is not readable through another Environment ID by default
- Global Memory and Environment-private Memory survive service reconstruction
- Environment MCP/Skill selections survive service reconstruction
- removing a global catalog entry leaves existing Environment references explicit instead of silently rewriting them
- the MCP Gateway advertises the new development-context management tools
- the Phase 01 non-Git development loop continues to pass

## Explicit non-goals

This phase does not implement:

- Git worktree lifecycle
- directory-copy isolation
- clone/container/VM isolation
- task/session attribution
- parallel Agent orchestration
- GSD workflow automation
- process/log/port management
- V1 state migration or compatibility
- UI

The next development step after switching real Agent traffic to V2 is dogfood. Optional Git-worktree isolation may then be implemented from concrete self-development needs without becoming an Environment prerequisite.

## New prerequisites

None for Workspace or Environment admission.

MCP/Skill catalogs and Memory are persisted context features, not development prerequisites. Git remains required only for Git-specific operations. Shell execution remains available only for explicitly allowlisted executables.
