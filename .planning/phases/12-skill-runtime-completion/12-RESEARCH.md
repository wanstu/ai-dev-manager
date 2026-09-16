# Phase 12 Research — Skill Runtime Completion

## Current implementation audit

### Discovery

`internal/skill/service.go` performs a real explicit-root walk for files named `SKILL.md` and intentionally skips symlinks. A discovered entry records:

- deterministic Skill ID;
- display name from the Skill directory;
- artifact path;
- source root;
- authorized support roots;
- default-include setting.

This is a valid foundation and should be preserved conceptually.

### Identity problem

`StableID(name)` hashes only the lowercased Skill name. This means identity is not tied to the configured source/artifact. Two explicitly configured source roots containing the same Skill name produce the same global ID, which can cause one source to replace another in `AddSkillRoot`.

For a global multi-source Skill manager, source-aware identity is required before refresh can be authoritative.

Recommended identity input:

```text
stable ID = hash(canonical source identity + source-relative SKILL.md path)
```

Content changes must not change the ID. Moving a Skill to a different source/path is a different artifact identity and may leave old Environment selection IDs unresolved after refresh.

### Source lifecycle problem

`catalog.AddSkillRoot` currently discovers and immediately upserts discovered Skills, but the source itself is not a first-class persisted registration. Consequently:

- there is no canonical list of configured Skill sources to refresh;
- deleted artifacts cannot be authoritatively removed from one source without reconstructing ownership from entry fields;
- source-level status (missing root, broken support root, last refresh) has no proper home.

Introduce `SkillSource` as desired management state and derive discovered Skill entries from those sources.

### Read boundary

`skill.Read` already provides the important safety boundary:

- configured artifact required;
- artifact and support roots are canonicalized;
- requested file must resolve inside artifact dir or authorized support roots;
- text is bounded;
- binary/NUL content is rejected;
- missing artifact/support root produces a local error.

Do not replace this with an interpreter. Add structured status around this existing safe read boundary.

## Recommended persisted model

Conceptual model:

```text
SkillSource
  id
  root
  support_roots
  default_include_in_environment
  created_at

SkillDefinition
  id
  source_id
  name
  artifact_path
  relative_artifact_path
```

Source refresh computes the current set of definitions for that source and atomically replaces the source-owned discovered set.

Environment selections continue to store Skill IDs only.

## Refresh semantics

`skill_source_refresh(source_id)`:

1. load one persisted source;
2. canonicalize only that configured source/support roots;
3. discover current `SKILL.md` artifacts;
4. validate duplicate artifact identity within that source;
5. atomically replace discovered definitions owned by that source;
6. preserve Environment ID selections even when an old Skill disappears so the selection becomes explicitly unresolved;
7. return source refresh facts: discovered/added/updated/removed counts and per-source error.

A source-level failure should not delete the previously known catalog automatically unless the source was successfully scanned and an artifact was confirmed absent. Failed refresh and successful empty/removal refresh are different states.

## Availability model

Environment-specific Skill availability should not be a boolean. Recommended state/reason concepts:

- `available` — enabled and current artifact/read boundary valid;
- `disabled` — known Skill but not selected in Environment;
- `unresolved` — Environment selected an ID no longer present in catalog;
- `source_missing` — configured source cannot currently be resolved;
- `artifact_missing` — definition exists but artifact no longer resolves;
- `artifact_unreadable` — artifact fails regular/text/bound checks;
- `support_root_missing` — one configured support root is currently invalid;
- `unconfigured` — legacy/metadata-only record, if any remains during development.

A Skill can remain catalog-visible while reporting unavailable. Status should carry `skill_id`, `source_id`, display name, artifact/source facts and sanitized reason.

## Support-file visibility

The consuming Agent needs more than raw error strings, but ADM should not parse Skill prose.

Recommended surfaces:

- `environment_skill_inspect(skill_id)` — availability + source/artifact/support-root facts;
- `environment_skill_files(skill_id, scope, max_entries)` — bounded inventory under the Skill artifact directory or one explicitly configured support root;
- existing `environment_skill_read` — authoritative bounded content read.

If a Skill instruction references a file, the Agent follows the instruction and requests the file. ADM validates authorization/existence and returns structured read failure if missing. ADM does not crawl markdown references and invent dependency semantics.

## Refresh concurrency / safety

Refresh is management mutation, not Environment writer mutation: it changes global ADM catalog state, not project files. It must be atomic in the persisted store so Agent list/inspect sees either the old source snapshot or the new one, never a half-refreshed catalog.

Environment selection authorization remains checked at every environment-scoped list/inspect/read operation.

## Main risks

1. **same-name collision** — fix identity before introducing authoritative refresh.
2. **failed refresh deleting good catalog state** — distinguish scan failure from successful discovery result.
3. **silent selection rebinding** — never map a disappeared ID to a new same-name Skill.
4. **scope creep into interpretation** — no markdown dependency execution/planning.
5. **support-root escape/symlink problems** — keep canonical containment checks and negative tests.
6. **one broken Skill poisoning all Skills** — status/errors are per Skill/source and operation-local.
