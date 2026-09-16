# Phase 12 Context — Skill Runtime Completion

## Goal

Turn ADM's real `SKILL.md` discovery/read foundation into a refreshable, source-aware and diagnosable Skill runtime-context capability without implementing a Skill execution engine.

## Requirements

- SKILL-COMP-01 — explicit refresh discovers current real Skill artifacts from configured sources only.
- SKILL-COMP-02 — artifact/support availability and failed authorized support reads are diagnosable per Skill without breaking unrelated Skills.
- SKILL-COMP-03 — Environment selection remains authorization and availability explains enabled/disabled/missing/broken states.
- SKILL-COMP-04 — ADM exposes real Skill/support content facts for the consuming Agent but does not interpret or orchestrate Skill instructions.

## Existing foundation

ADM already:

- discovers `SKILL.md` under an explicitly configured root;
- stores artifact path/source root/support roots;
- blocks symlink discovery and out-of-root reads;
- gives deterministic IDs using a normalized Skill name;
- allows the same global catalog entry to be selected by multiple Environments;
- exposes Environment-gated list/read through the Gateway;
- has real-host GSD Skill acceptance.

## Current gaps

1. `AddSkillRoot` discovers and upserts entries, but ADM does not persist a first-class source registration with a formal refresh lifecycle.
2. Refresh does not authoritatively remove artifacts deleted from a source.
3. Stable Skill ID is currently derived from name only. Two explicit sources containing the same Skill directory name can collide or silently replace one another.
4. There is no single Skill availability/diagnostic result explaining source missing, artifact missing/unreadable, support-root missing, disabled or unresolved selection.
5. Authorized support-file read works, but support-file/source inventory is not directly inspectable in a bounded form.
6. A broken Skill read returns an error, but the consuming Agent has to infer availability state itself.

## Locked decisions

1. Skill remains an Agent-consumed artifact, not an ADM workflow/execution engine.
2. Persist explicit `SkillSource` registrations. A source includes its discovery root, authorized support roots and default-include policy.
3. Skill identity is source/artifact based, not name-only. Same display name from two explicit sources is allowed and remains distinguishable by stable IDs/source facts.
4. Refresh is explicit and atomic per source. It scans only that registered source.
5. If a previously discovered artifact disappears on refresh, remove that catalog Skill from that source. Existing Environment selections keep the old ID as an unresolved reference; ADM does not silently bind them to another same-name Skill.
6. Source/artifact/support paths may be exposed as explicit configured facts, but reads remain containment-checked and bounded.
7. Availability states include enough detail to distinguish at least `available`, `disabled`, `unresolved`, `source_missing`, `artifact_missing`, `artifact_unreadable`, and `support_root_missing`.
8. ADM does not parse natural-language Skill instructions to infer commands, prerequisites or task steps.
9. ADM does not try to parse arbitrary markdown references and decide dependency semantics. If an Agent requests a missing authorized support file, return a structured support-read error; source/support-root existence is diagnosable independently.
10. A broken Skill affects only that Skill. Other Skills, MCPs and ordinary Environment operations remain usable.

## Non-goals

- no Skill execution engine;
- no automatic prompt injection/context composition;
- no arbitrary host Skill scan;
- no package registry/update marketplace;
- no interpretation of GSD/project `.planning` state;
- no dependency graph inferred from prose;
- no Desktop redesign in this Phase.
