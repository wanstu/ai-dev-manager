# Phase 19 — Workspace Discovery + Project Navigation

Date: 2026-09-12
Inspected baseline: 5c30979, master, clean.
Status: 19-01 Core complete; see 19-01-SUMMARY.md. 19-02 surfaces are ready and not started.

## Goal and authority

Given an explicitly registered Workspace containing projects/p1, projects/p2 and projects/p3, return a bounded directory summary and evidence-backed project candidates so the caller can choose a root. This refines ADM-GOAL-001 and ADM-CORE-001/002/003/004/005/006, with ADM-MGMT-001 and ADM-DESKTOP-001 surface parity. No new product prerequisite.

The user handoff requested opening/confirming the detailed Phase 19 plan before execution. This checkpoint prepares a reviewable plan; no feature implementation is claimed. Recent dogfood fixes e180cb9, be591d4, 5ca528f and 5c30979 are complete committed nodes. Their native visual handoffs remain distinct from Phase 19 acceptance.

## Scope

- Explicit bounded metadata scan, project-marker evidence and relative candidate roots.
- Compact tree digest: a directory/marker summary, not a cryptographic hash, code index, dependency graph or Agent context bundle.
- Workspace discovery before Environment creation; Environment tree digest through existing Runtime-root validation.
- Shared Core, Agent/Admin MCP, normal CLI through Admin MCP, Desktop explicit scan and existing Environment creation flow.

## Boundaries

1. Workspace discovery requires a stable registered Workspace ID and optional contained relative path. Environment digest requires a stable Environment ID and its validated Runtime root. No arbitrary host path parameter or fallback from a narrow Environment to its parent Workspace.
2. Read-only discovery requires no writer, Git executable, shell, verifier, MCP or Skill. It creates no Workspace/Environment, edits no file and starts no process.
3. Resolve and validate roots and selected subpaths. Do not follow symlinks/junctions/reparse directories during traversal; report skipped links. Explicit link targets outside the authorized root are refused. Windows short/long paths use existing pathutil behavior. Missing/replaced roots fail locally.
4. Inspect names/types only. Marker content, README, source, .env values, .git/config, Memory and external MCPs are not read. Markers suggest a project; they do not prove buildability, health or authorization.
5. Depth, visited entries, returned candidates, digest entries and output bytes have defaults and hard caps. Limits and skipped/error observations are included in the report. A partial result is never described as complete.
6. Directory reads must be bounded themselves: use incremental ReadDir batches, not an unbounded directory listing before applying a result limit. Context cancellation is checked between batches/entries. A slow OS filesystem operation is not advertised as strictly preemptible.
7. Default traversal skips .git contents, node_modules, vendor, dist, build, target, bin, obj, coverage, .next, .venv, venv, __pycache__, .tmp. Count .git marker before pruning. Ordinary non-marker directories remain navigable. The policy is disclosed; no hidden ignore-file parsing.
8. A bounded literal query ranks/filters discovered relative paths/names only. No semantic search or natural-language interpretation. Exact basename, exact relative path and substring evidence are distinguishable. Ambiguous matches remain multiple candidates.
9. No automatic selection, project opening, context injection or Environment creation. Creation uses the existing service's fresh root validation; scan results do not grant authority.
10. Global MCP health and Skill structural availability remain independent of Environment selection. This phase must not reintroduce that coupling.

## Delivery order

19-01: shared bounded scanner, candidate/digest DTOs and app authority entry points with behavioral tests.
19-02: Gateway/Admin client/CLI/Desktop views, explicit creation handoff and integrated acceptance.

No persistence or new frontend dependency; no index daemon, cache lifecycle, watcher, background scan, remote provider, worktree policy, Phase 20 bundle or Phase 21 async resource model.

## Source calibration

internal/workspace/service.go stores registered paths; internal/app/service.go Runtime validates Environment/isolation before file operations; internal/pathutil handles canonical comparison; internal/runtime/runtime.go has the existing tree/file authority behavior. Reuse authority helpers where appropriate, but do not copy an unbounded traversal into the new scanner. Existing state is not migrated. New code placement is documented in each plan.

On resume: inspect Git/AGENTS/STATE, read this context and 19-01-PLAN, obtain writer only for implementation, and preserve the existing master/local-commit/no-push/no-subagents rules.
