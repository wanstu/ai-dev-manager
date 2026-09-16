# Temporary Resource Lifecycle Design Note

Date: 2026-09-09
Status: captured for later design; not part of Phase 13-01 implementation

## Problem

Agent-facing MCP usage can create many development Environments while working through isolated worktrees. ADM currently has explicit create/remove/destroy operations, but no retention or cleanup model for temporary resources that were created by automation and then abandoned.

This can leave stale Environments, managed worktrees, MCP definitions, Skill sources/Skills, and related selections in local state even after the development task is no longer active.

## Initial product boundary

- CLI/UI-created Workspace, Environment, MCP and Skill resources are durable by default.
- MCP/Gateway-created resources may be temporary by default, but the exact contract needs design before implementation.
- Cleanup is ADM lifecycle/diagnostics/authorization scope, not Agent planning/orchestration scope.
- Cleanup must not revive Planner/Executor/Reviewer, `.planning` state advancement, `gsd_phase_*`, or task orchestration semantics.

## Candidate model to discuss

Temporary resources may need explicit metadata such as:

- persistence class: `durable` or `temporary`;
- creator surface: CLI, UI, Gateway/MCP, import, or internal migration;
- owner/session/run identity where available;
- attached Environment or temporary Environment scope;
- created_at, last_used_at and optional expires_at;
- retention policy such as ttl-after-last-use;
- cleanup eligibility reason and last cleanup decision.

## Candidate retention semantics

Potential defaults to evaluate:

1. CLI/UI resources stay durable unless explicitly marked temporary.
2. Gateway/MCP-created Environments, MCPs, Skill sources or Skills may default to temporary.
3. Temporary MCPs/Skills can be attached to an Environment; if the Environment is cleaned up, attached temporary resources are cleaned up too.
4. Detached temporary MCPs/Skills can be cleaned when `last_used_at + ttl` has elapsed, for example after 72 hours of no use.
5. Cleanup should skip or block resources with an active writer, active Gateway-owned process/run, dirty/unpublished managed worktree, or unresolved safety condition.
6. Managed worktree cleanup must reuse the existing destroy safety gate; no raw directory deletion path should bypass it.
7. Cleanup should have a dry-run/preview inspection surface before destructive execution.
8. Automatic cleanup, if added, should be configurable and conservative; explicit cleanup remains the safer first implementation.

## Candidate surfaces

Possible future surfaces:

- `retention_policy_inspect` — show effective policy and temporary/durable interpretation.
- `temporary_resource_list` or `resource_retention_inspect` — list cleanup candidates and blockers without mutation.
- `temporary_resource_cleanup` or `resource_retention_cleanup` — perform cleanup only for eligible resources, ideally with dry-run by default.
- CLI/UI controls to promote temporary resources to durable or mark durable resources as temporary.

## Open questions

1. Should MCP/Gateway-created resources always be temporary, or should the caller choose `temporary=true` explicitly?
2. Should temporary MCP/Skill definitions be global with ownership metadata, or Environment-scoped attachments that are invisible outside that Environment?
3. Should temporary Skill sources and discovered Skills have independent TTLs, or inherit TTL only from their owning Environment?
4. What counts as `last_used_at` for MCP, Skill, Workspace and Environment resources?
5. Should automatic cleanup run inside the Gateway owner process, a CLI command, Desktop, or a future local daemon?
6. What is the default TTL, and should 72h be the default or just a configurable example?
7. How should stale resources created before this metadata exists be classified during migration?

## Placement

This is a later lifecycle/retention design item. It should be considered after Phase 13 capability diagnostics stabilizes, or as a dedicated follow-up plan before broader Desktop/distribution work if stale Agent-created resources become a dogfood blocker.
