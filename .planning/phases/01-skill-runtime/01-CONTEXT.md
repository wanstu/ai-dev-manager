# Phase 01 Context — Real Skill Runtime + GSD Bootstrap

## Goal

Turn Skill from a persisted text record into a real, Environment-scoped capability that an external Agent can consume through ADM. Use the actual globally installed GSD Skill as the primary acceptance case.

## Problem With Current V2

Current V2 stores `CatalogEntry{Name, Instructions}` and Environment `EnabledSkillIDs`, then exposes selected instructions through `environment_skill_context`.

That proves persistence and selection, but it does not model or consume an actual Skill installation. It cannot truthfully satisfy “use the GSD Skill through ADM”.

## Reference From Earlier ADM

The earlier implementation validated these useful semantics:

- Skill discovery scans only explicit configured roots.
- A Skill is represented by a real path/artifact, initially centered on `SKILL.md`.
- One global GSD Skill root can serve multiple Workspaces without copying the Skill into each project.
- Project `.planning/` is project state and remains separate from the global Skill installation.

V2 should reuse these semantics where they fit, not copy the old implementation wholesale.

## Real Host Discovery — 2026-09-06

The actual GSD installation on this machine is an OpenCode Skill suite under:

- discovery root: `C:\Users\wanstu\.config\opencode\skills`
- shared GSD support root: `C:\Users\wanstu\.config\opencode\gsd-core`

There is no single `gsd/SKILL.md`. The installation contains many real Skills such as `gsd-next`, `gsd-plan-phase`, `gsd-execute-phase`, and `gsd-manager`. `gsd-next/SKILL.md` is the first acceptance artifact because it is the state-aware front door. Its `SKILL.md` directly references workflow/support files under the sibling `gsd-core` root.

Therefore the Phase 01 source model must distinguish an explicitly configured **discovery root** from explicitly configured **support roots**. Agent Skill reads may reach only the selected Skill artifact or files contained by those configured support roots. An arbitrary absolute path, including another host config path, is not authorized merely because it appeared in Agent input.

## Locked V2 Constraints

- Environment remains Git-independent.
- Environment selection remains the access gate.
- No arbitrary filesystem-wide Skill scan.
- No compatibility shim for the current development-only instructions records is required; the model may be corrected directly.
- Do not build Agent Run/GSD executor here. This Phase only makes the Skill real and consumable.
- Desktop gets only the minimum management adjustment required by the new Skill model; no redesign.

## Design Questions The Plan Must Resolve In Code

1. Exact persisted Skill definition: explicit artifact path vs configured roots + discovered metadata.
2. Stable Skill ID derivation/collision rules.
3. Agent-facing protocol surface for list/read/use while preserving Environment gating.
4. How global Skill roots are configured without turning arbitrary host paths into Agent file access.
5. How supporting Skill files are exposed, if the real GSD Skill requires more than `SKILL.md`.

These questions are resolved by the smallest real GSD vertical slice, not by speculative plugin abstractions.

## Exit Criteria

1. ADM discovers the real globally installed GSD Skill from an explicitly configured location.
2. Environment A enables GSD and can list/read its real Skill artifact through the Gateway.
3. Environment B does not enable GSD and cannot read/use it.
4. A second enabled Environment can consume the same global Skill without a copied project-local installation.
5. The Agent uses the Skill through ADM to read the project `.planning` state and prepare the next Phase transition.
6. No unrelated capability becomes a prerequisite.
7. Focused tests + `go test ./...` + `go vet ./...` + real Gateway acceptance pass.