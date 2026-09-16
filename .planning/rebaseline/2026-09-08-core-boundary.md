# 2026-09-08 Core Boundary Rebaseline

## Why this rebaseline exists

The roadmap drifted from ADM's product role into Agent/GSD orchestration. Phase 8 added a generic asynchronous Runtime resource, which still fits ADM. Phase 9 then added Planner / Executor / Reviewer workflow semantics, and the abandoned Phase 10 branch started implementing GSD `.planning` interpretation and STATE advancement. Those are orchestration concerns and duplicate responsibilities that belong to the Agent/GSD layer.

The user explicitly rejected that direction on 2026-09-08 and asked to refocus ADM on its actual core: MCP, Skill and safe local development capabilities.

## Product boundary

ADM is a local AI development control plane.

ADM owns:

- Workspace and Environment identity and lifecycle;
- Environment-scoped file/search/edit/delete capabilities;
- allowlisted command execution and long-lived development processes;
- writer/concurrency safety;
- external MCP configuration, activation, session lifecycle, tool discovery/call and diagnostics;
- Skill discovery, source/support-file access, Environment selection and availability diagnostics;
- structured verifier execution;
- optional Git/worktree capabilities;
- stable Runtime resource identities when a resource must outlive one client request;
- capability/status/diagnostic evidence that an external Agent can inspect.

ADM does not own:

- task decomposition or planning;
- Planner / Executor / Reviewer role orchestration;
- interpreting natural-language plans into commands;
- GSD phase selection or `.planning/STATE.md` advancement;
- deciding the next task/phase;
- automatic Git integration decisions;
- multi-Agent reasoning policy.

Those belong to the Agent, GSD, or another orchestrator consuming ADM capabilities.

## Existing implementation audit

### Keep as ADM core

- Phases 1-7: core Skill/MCP/verifier/runtime/process/isolation foundations, subject to continued completion work.
- Phase 8 single-command `run_`: keep only as a generic asynchronous Runtime lifecycle (`start/list/status/cancel`) because it solves client-independent command ownership without prescribing task semantics.

### Mis-scoped and scheduled for cleanup

- Phase 9 `run_workflow_start`, workflow plan/step/reviewer domain model and FLOW-01 product requirement.
- Phase 10 GSD Phase Executor branch (`feat/gsd-phase-executor`): abandoned and must never be merged into master.
- Former Phase 11 parallel Agent orchestration: removed from the ADM roadmap. Worktree remains an optional isolation primitive; an external orchestrator may use it.

The Phase 9 history remains in Git/evidence for auditability, but its orchestration surface is not authoritative product behavior after this rebaseline.

## Corrected priority

1. Remove mis-scoped orchestration from ADM Core while retaining generic Runtime primitives.
2. Complete external MCP Runtime as a first-class core capability.
3. Complete Skill Runtime as a first-class core capability.
4. Make Environment capability routing and diagnostics explicit: what is available, what is not, and why.
5. Add evidence-first development investigation helpers only where they materially improve Agent accuracy/speed.
6. Expose validated Core through human management surfaces.
7. Distribution polish only when daily use proves a need.

## MCP direction

The existing Streamable HTTP tracer/session lifecycle is a foundation, not MCP completion. Future MCP work should cover a coherent definition/runtime model, supported transports, secret/auth handling, session/process lifecycle, tool inventory refresh, Environment gating and actionable diagnostics. A missing transport/auth mode must block only that MCP, never ordinary development.

## Skill direction

The existing real `SKILL.md` discovery/read path is a foundation, not Skill completion. Future Skill work should cover explicit refresh, source/support-file facts, broken-artifact isolation, Environment selection, availability diagnostics and clear dependency/capability facts without inventing an ADM-specific Skill execution engine. The Agent consumes the Skill; ADM makes the Skill safely available and explainable.

## Planning consequence

The old Phase 10-13 roadmap is superseded. The next implementation phase is orchestration-boundary cleanup, followed immediately by MCP Runtime completion and Skill Runtime completion. Phase 10 GSD implementation remains isolated and abandoned.
