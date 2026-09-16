# Phase 00 Context — Rebaseline and Planning Authority

## Why This Phase Exists

V2 accumulated working code but lost a trustworthy execution queue. The immediate problem is not one more missing feature; it is that metadata/CRUD slices were sometimes treated as completed capabilities and reactive dogfood fixes began replacing planned development.

This Phase fixes planning truth only.

## Inputs

- current V2 source/tests;
- `docs/PRODUCT_CONTRACT.md` and historical `docs/PHASE_*`/`STATUS.md`;
- earlier ADM GSD planning artifacts under `D:\projects\.ai-dev-manager-worktrees\...\.planning`, especially its validated sequencing and GSD rules.

## Locked Decisions

- Product contract semantics still win over a Phase plan.
- `.planning/` becomes the active implementation queue.
- Existing `docs/PHASE_*` files are historical evidence, not automatically current truth.
- The earlier ADM implementation is reference/evidence; V2 does not silently import its architecture or code.
- No feature code changes in Phase 00.
- No Desktop polish/package work.

## Exit Criteria

1. PROJECT records real capability state and locked rules.
2. ROADMAP defines dependency-ordered milestones through Core, lifecycle, Agent/GSD and Human Manager.
3. STATE identifies one current Phase/Plan and backlog rules.
4. Phase 01 has Context + executable Plan.
5. Misleading project-status claims are reconciled without rewriting history.
6. Repository diff for Phase 00 is planning/docs only.