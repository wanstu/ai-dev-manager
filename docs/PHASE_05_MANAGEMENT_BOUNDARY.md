# Phase 05 — Management boundary

Status: complete.

## Requirement IDs

- ADM-MGMT-001
- ADM-CORE-015
- ADM-CORE-017
- ADM-DEV-001
- ADM-DEV-003

## Goal

Create one application-backed boundary for desktop/human management clients so future Wails or HTTP adapters do not read `state.json` directly and do not recreate ADM product semantics.

## Delivery slices

### Slice A — Read-only management snapshot

Status: delivered.

- aggregate registered Workspaces
- aggregate lightweight Environment summaries
- include exec allowlist
- include MCP catalog
- include Skill catalog
- include Global Memory entry count, not values
- reuse current application services and persisted store
- no Git/worktree/Gateway prerequisite

### Slice B — Explicit management commands

Status: delivered.

- expose thin named mutations for Workspace lifecycle
- expose thin named mutations for Environment lifecycle
- expose exec allowlist add/remove
- expose MCP/Skill catalog add/remove/default changes
- expose per-Environment MCP/Skill selection changes
- expose explicit-scope Global and Environment-private Memory write/delete
- delegate validation and persistence to existing application services
- return sanitized Environment summaries instead of private Memory values

### Slice C — Desktop adapter

Status: delivered.

- expose a Wails-friendly Go adapter with exported methods and JSON-friendly input structs
- bind only to the management boundary, never to `state.json`
- expose overview snapshot and explicit Workspace/Environment detail reads
- expose explicit Global and Environment-private Memory reads only when requested
- expose the already-defined management mutations from Slice B
- keep the adapter independent from the Wails runtime package so it remains testable without a desktop runtime
- do not introduce REST

## Acceptance tests

- snapshot reflects Workspace and Environment state
- Environment private Memory values are absent
- Global Memory values are absent
- catalog and allowlist state are present
- persisted changes appear in a later snapshot
- snapshot works for a non-Git Workspace
- management mutations preserve existing lifecycle and validation errors
- Environment mutation results do not expose private Memory values
- management mutations remain visible through a later snapshot
- desktop adapter exposes overview, explicit detail/Memory reads, and mutations through the management boundary
- desktop overview and Environment inspect do not expose private Memory values
- adapter remains usable and testable without importing a Wails runtime package
- no second state/cache file is introduced

## Explicit non-goals

- UI implementation in Slice A
- REST API without a client requirement
- Git worktree lifecycle
- isolation/orchestration
- direct state.json access from UI code
- V1 compatibility or migration

## New prerequisites

None.
