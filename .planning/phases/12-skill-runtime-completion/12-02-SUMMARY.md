---
phase: 12-skill-runtime-completion
plan: "02"
subsystem: skill-runtime
tags: [skill, availability, diagnostics, support-inventory, gateway]
requires:
  - phase: 12-skill-runtime-completion
    plan: "01"
    provides: Persisted SkillSource model, source/artifact stable identity and atomic per-source refresh
provides:
  - Environment-specific Skill availability states and structured reasons
  - environment_skill_inspect and environment_skill_files Gateway surfaces
  - Structured Skill read error kinds for artifact/support failures
  - Bounded artifact/support-root inventory without interpreting Skill instructions
  - Broken-Skill isolation across list/read/MCP/file/verifier operations
implementation_commit: 8f46f6e019afc8722708e3a4478de06faa4b241a
tech-stack:
  added: []
  patterns: [application-level availability facts, bounded inventory, structured local errors, operation-local failures]
key-files:
  created: [internal/app/skill_availability.go, internal/app/skill_availability_test.go, internal/gateway/skill_availability_test.go, internal/management/skill_availability_test.go]
  modified: [internal/app/service.go, internal/gateway/server.go, internal/gateway/server_test.go, internal/gateway/skill_source_test.go, internal/management/service.go]
key-decisions:
  - "Skill availability is Environment-specific and includes enabled/disabled/unresolved plus source/artifact/support-root failure states."
  - "environment_skill_read remains the authoritative bounded content access path; diagnostics wrap its local errors without parsing Skill prose."
  - "environment_skill_files inventories only the Skill artifact directory or explicit configured support roots, skips symlinks and caps results."
  - "A broken Skill reports per-Skill state and does not poison other Skills, MCP, file, verifier or ordinary Runtime operations."
requirements-completed: [SKILL-COMP-02, SKILL-COMP-03, SKILL-COMP-04, ADM-GW-003]
completed: 2026-09-09
status: complete
---

# Phase 12 Plan 02: Skill Availability, Support Inventory + Diagnostics Summary

**Skill runtime context is now a first-class diagnosable ADM capability.** Agents can ask ADM whether a Skill is available in one Environment, why it is unavailable, and which authorized artifact/support files are visible, while ADM still does not interpret or execute Skill instructions.

## Accomplishments

- Added `SkillAvailability` with stable Environment-specific states:
  - `available`
  - `disabled`
  - `unresolved`
  - `source_missing`
  - `artifact_missing`
  - `artifact_unreadable`
  - `support_root_missing`
  - `unconfigured` for legacy metadata-only entries.
- Added source/artifact/support facts to availability results: Skill ID, source ID, source root, artifact path, source-relative artifact path, configured support roots and missing support roots.
- Added `EnvironmentSkillAvailabilities` so list-style access returns per-Skill status instead of failing the whole surface when one selected Skill is broken.
- Added `InspectEnvironmentSkill` for single-Skill diagnostics, including unresolved Environment selections left behind by source refresh/remove.
- Added `EnvironmentSkillFiles`, a bounded inventory over either the Skill artifact directory or an explicit support root (`artifact`, `support`, `support:<index>`), capped at 256 entries by default and skipping symlinks.
- Wrapped `ReadEnvironmentSkill` failures in structured `SkillError` kinds, including `not_enabled`, `unresolved`, `outside_allowed_roots`, `file_missing`, `artifact_missing`, `artifact_unreadable`, `support_root_missing`, `binary_file`, `file_oversize`, `not_regular` and `read_failed`.
- Added Gateway tools:
  - `environment_skill_inspect`
  - `environment_skill_files`
- Updated `environment_skill_list` to return Environment-specific availability facts rather than the older configured/unconfigured split.
- Added management wrappers for availability list, inspect and file inventory.
- Preserved existing `environment_skill_read` as the authoritative content-read boundary; artifact/support containment and max-byte checks remain in the Skill runtime package.

## Verification Evidence

### Availability states and broken-Skill isolation

- `TestEnvironmentSkillAvailabilityStatesAndIsolation` proves available, disabled, unresolved, source-missing, artifact-missing, artifact-unreadable and support-root-missing states.
- The same test proves list-style availability isolates broken Skills and still reports unrelated healthy/disabled Skills.

### Support inventory and structured read errors

- `TestEnvironmentSkillFilesAndStructuredReadFailures` proves artifact inventory includes `SKILL.md` and sibling files, support inventory includes configured support-root files, and support reads remain authorized.
- The test also proves structured errors for outside-root reads, missing support files, artifact-missing reads, support-root-missing reads, binary content and max-byte violations.

### Gateway and management surfaces

- `TestGatewaySkillInspectFilesAndReadDiagnostics` proves real Gateway calls for `environment_skill_inspect`, `environment_skill_files` and `environment_skill_read`, including structured outside-root failure.
- `TestGatewaySkillSourceLifecyclePreservesUnresolvedSelections` was updated to expect the new `unresolved` availability state after source removal.
- `TestManagementSkillAvailabilityDelegatesToApplicationBoundary` proves the management layer delegates to the application availability/inventory boundary.
- Gateway tool registration now includes `environment_skill_inspect` and `environment_skill_files`.

### Real-host and regression gates

All implementation gates passed on Windows:

- `go test -count=1 ./cmd/ai-dev-manager ./internal/skill ./internal/catalog ./internal/app ./internal/management ./internal/gateway`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check` after staging new files with intent-to-add so new files were included
- `go test -race -count=1 ./internal/app ./internal/management ./internal/gateway -run Test.*Skill`

The full Gateway package includes the existing real-host installed GSD Skill HTTP acceptance and multi-Environment authorization regression. It remained green after the 12-01 source model and 12-02 availability changes.

## Security / Boundary Review

- No Skill interpreter, command generation, Planner/Executor/Reviewer workflow, `.planning` interpretation or task/phase advancement surface was added.
- `environment_skill_files` only inventories an enabled Skill's artifact directory or configured support roots; it does not crawl arbitrary host paths.
- Symlinks are skipped during inventory and existing read containment checks continue to resolve and reject escapes.
- Diagnostics expose configured source/artifact/support facts and local error kinds, not unrelated filesystem content.
- Broken Skill states remain operation-local and do not mark the entire Environment or unrelated capabilities unusable.

## Phase 12 Result

With 12-01 and 12-02 complete, Skill is now a refreshable, source-aware, Environment-gated and diagnosable ADM Core capability. ADM provides Skill artifacts/support context safely to the consuming Agent, but the Agent remains responsible for following Skill instructions.

## Next Phase Readiness

Phase 13 can now build the authoritative Environment capability diagnostics report on top of:

- completed MCP desired/observed runtime facts from Phase 11;
- completed Skill source/refresh identity from 12-01;
- completed Skill availability and support inventory diagnostics from 12-02.

Phase 13 remains unimplemented at this closeout.

---

*Phase: 12-skill-runtime-completion*
*Plan: 12-02 complete; Phase 12 complete; Phase 13 next*
