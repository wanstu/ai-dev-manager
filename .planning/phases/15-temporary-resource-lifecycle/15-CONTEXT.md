# Phase 15 Context — Temporary Resource Lifecycle

## Why this phase exists

After MCP/Skill/runtime/capability diagnostics and the first evidence-first investigation helpers, ADM can be used by external Agents for real local development. Agent-facing MCP/Gateway usage can also create many Environments, MCP selections, Skill selections, runs, processes or provider resources that are useful for one investigation session but should not live forever.

The 2026-09-09 temporary resource note captured the problem. This phase promotes that design from deferred note into an explicit future Core lifecycle phase.

## Product boundary

ADM owns local resource lifecycle, authorization, cleanup eligibility and diagnostics. ADM still does not own task planning, GSD phase advancement, automatic merge policy, or Agent orchestration.

## Durable vs temporary rule

- CLI/UI-created Workspaces, Environments, MCP definitions and Skill definitions remain durable by default.
- Gateway/MCP-created resources may be temporary only when the creation surface or explicit flag marks them temporary.
- Temporary resources must carry ownership, attachment, last-used and retention metadata.
- Cleanup must be explainable and conservative.

## Safety principle

Cleanup must never silently delete project work. Destructive cleanup needs eligibility evidence and should provide a dry-run/preview path before actual deletion.

## Relationship to Desktop

Desktop should not invent its own cleanup rules. Temporary lifecycle should be implemented first so Desktop can expose the same Core retention state and cleanup preview/execute actions.
