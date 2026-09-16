# Phase 12 Validation — Skill Runtime Completion

## Validation principle

Skill completion means ADM can maintain trustworthy source/artifact identity and availability for real Skills. It is not accepted by catalog CRUD alone and does not require ADM to execute Skill instructions.

## Required proof matrix

### P1 — source-aware stable identity

Configure two explicit Skill sources that each contain a same-name Skill directory.

Prove:

- both Skills coexist with distinct stable IDs;
- each ID resolves to the correct source/artifact;
- editing SKILL.md content does not change the ID;
- moving/removing the artifact changes availability/identity semantics explicitly rather than silently rebinding by name.

### P2 — atomic explicit refresh

For one registered source:

1. refresh with Skill A;
2. add Skill B and refresh;
3. edit Skill A and refresh;
4. delete Skill A and refresh.

Prove returned added/updated/removed counts and resulting catalog are correct.

Negative requirement: make the source temporarily unreadable/missing and prove failed refresh preserves the last known catalog snapshot rather than deleting all source-owned Skills.

### P3 — unresolved Environment selection is preserved

Enable Skill A in an Environment, then successfully refresh after deleting A.

Prove:

- A is removed from current discovered catalog;
- Environment selection still contains A's old ID as unresolved;
- a same-name Skill from another source is not silently substituted;
- unrelated Skill selections remain unchanged.

### P4 — availability states are explicit

Machine-prove at least:

- available;
- disabled;
- unresolved selection;
- source missing;
- artifact missing;
- artifact unreadable/non-text/oversize;
- configured support root missing.

Each result identifies the Skill/source and a structured reason without exposing unrelated filesystem content.

### P5 — authorized support inventory/read remains contained

Prove bounded `environment_skill_files`/equivalent inventory and `environment_skill_read` can access:

- SKILL.md;
- sibling authorized artifact files;
- files in explicitly configured support roots.

Negative tests reject:

- absolute/outside paths not under configured roots;
- `..` escape;
- symlink escape;
- binary/oversize content;
- missing support file with structured local error.

### P6 — broken Skill is operation-local

With one broken enabled Skill and one healthy enabled Skill in the same Environment, prove:

- healthy Skill remains listable/readable;
- broken Skill status is local to that Skill;
- MCP/file/verifier operations still work.

### P7 — real installed Skill acceptance

After source/identity refactor, configure the real installed GSD Skill source used by existing dogfood and prove an enabled Environment can still inspect/read `SKILL.md` plus an authorized support workflow through ADM. This is artifact-runtime acceptance only; ADM does not execute GSD planning.

### P8 — no interpreter/orchestrator surface

Source/code/Gateway audit must prove Phase 12 adds no:

- Skill command generation;
- Planner/Executor/Reviewer semantics;
- `.planning` interpretation;
- automatic task/phase advancement.

## Automated gates

- source/identity/refresh focused tests;
- availability/support containment tests;
- real-host Skill HTTP acceptance;
- multi-Environment selection isolation regression;
- MCP/file/verifier regression;
- `go test ./...` or equivalent complete package grouping;
- `go vet ./...`;
- `git diff --check`.
