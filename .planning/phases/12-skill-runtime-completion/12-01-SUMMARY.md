---
phase: 12-skill-runtime-completion
plan: "01"
subsystem: skill-runtime
tags: [skill, source, refresh, identity, catalog, gateway, cli]
requires:
  - phase: 11-mcp-runtime-completion
    plan: "03"
    provides: Completed MCP runtime/import boundary and current Core regression baseline
provides:
  - Persisted SkillSource model with explicit discovery root, support roots and default-include policy
  - Source/artifact-based stable Skill identity
  - Atomic per-source refresh with safe failure preservation
  - Source-aware Gateway, management and CLI operations
  - Unresolved Environment selection preservation after source-owned Skill removal
implementation_commit: 4074d3e8e5349ee717bf63ad027391d14579cecc
key-files:
  created:
    - cmd/ai-dev-manager/skill_source_cli_test.go
    - internal/app/skill_source_test.go
    - internal/catalog/skill_source_test.go
    - internal/gateway/skill_source_test.go
  modified:
    - cmd/ai-dev-manager/main.go
    - internal/model/types.go
    - internal/skill/service.go
    - internal/catalog/service.go
    - internal/gateway/server.go
    - internal/management/service.go
requirements-completed: [SKILL-COMP-01, SKILL-COMP-03, ADM-CORE-012, ADM-GW-003]
completed: 2026-09-09
status: complete
---

# Phase 12 Plan 01: Skill Sources, Stable Identity + Atomic Refresh Summary

**Skill catalog lifecycle is now source-aware and refreshable. ADM no longer has to identify discovered Skills by display name alone.**

## Accomplishments

- Added persisted `SkillSource` desired state with stable source ID, canonical discovery root, configured support roots, default-include policy, creation/update timestamps and last refresh status.
- Extended `CatalogEntry` with `source_id` and `relative_artifact_path` while keeping legacy metadata-only entries usable for existing tests/dev-state inspection.
- Changed discovered Skill identity to derive from source ID plus source-relative `SKILL.md` path. Content changes at the same artifact path preserve ID; the same Skill display name from two explicit sources can coexist.
- Added `AddSkillSource`, `ListSkillSources`, `RefreshSkillSource` and `RemoveSkillSource` catalog operations.
- Implemented atomic per-source refresh: ADM scans one registered source, then replaces only that source-owned Skill snapshot in one store update.
- Failed refresh records source error/status but leaves the previous source-owned catalog snapshot intact.
- Successful empty refresh is treated as a valid empty source snapshot, so deleted Skills are removed only after the source was actually scanned.
- Removing a source removes only that source's discovered Skills and preserves Environment `EnabledSkillIDs`, making old selections explicitly unresolved instead of silently rebinding to another same-name Skill.
- Kept `skill add --root` / `skill_add` as a compatibility helper that registers a source and refreshes it once. If that compatibility refresh discovers no Skills or fails, it rolls back the just-created source.
- Added Gateway tools: `skill_source_list`, `skill_source_add`, `skill_source_refresh`, and `skill_source_remove`.
- Added management wrappers for source add/list/refresh/remove.
- Added CLI commands: `skill source-list`, `skill source-add`, `skill source-refresh`, and `skill source-remove`.

## Verification Evidence

### Source identity and refresh behavior

- `TestSkillSourcesAllowSameNameSkillsAndStableArtifactIdentity` proves two sources may expose the same display name without ID collision, source default policy is applied, and repeated refresh at the same source-relative path preserves identity.
- `TestSkillSourceRefreshAddUpdateRemoveAndFailurePreservesSnapshot` proves add/remove refresh counts, successful removal of deleted artifacts, and failed refresh preserving the last known snapshot while recording source error status.
- `TestSourceArtifactSkillIDDependsOnSourceAndRelativeArtifactPath` proves source ID participates in Skill identity and relative artifact path is normalized.

### Environment authorization boundary

- `TestSkillSourceRefreshRemovalPreservesUnresolvedEnvironmentSelection` proves a default-included Skill is selected by a newly created Environment, then after successful source refresh removes the artifact, the old Environment selection remains as an unresolved Skill ID instead of being rewritten.
- `TestGatewaySkillSourceLifecyclePreservesUnresolvedSelections` proves Agent-facing Gateway source add/refresh/list/remove behavior and unresolved selection preservation.
- `TestSkillSourceCLIRegistersRefreshesListsAndRemoves` proves CLI source add/refresh/list/remove uses the same application/catalog boundary and preserves unresolved Environment selections after source removal.

### Regression gates

The following gates passed on Windows:

- `go test -count=1 ./cmd/ai-dev-manager ./internal/skill ./internal/catalog ./internal/app ./internal/management ./internal/gateway`
- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`
- `go test -race -count=1 ./cmd/ai-dev-manager ./internal/catalog ./internal/app ./internal/gateway -run SkillSource`

`git diff --check` reported only existing Windows LF-to-CRLF warnings.

## Boundary Review

- ADM still does not interpret Skill instructions or execute a Skill engine.
- Source add/refresh scans only the explicit registered root and uses the existing symlink/containment protections.
- Support roots remain explicit authorization facts; support inventory/diagnostics are deferred to Plan 12-02.
- Broken source refresh is source-local and does not delete previously known Skills or affect unrelated MCP/file/runtime operations.
- Environment selection remains the authorization boundary and is not silently repaired/rebound by name.

## Next Plan Readiness

Plan 12-02 can now build availability/diagnostics on top of first-class source and artifact facts:

- known source IDs and roots;
- source-owned Skill definitions;
- unresolved selection behavior;
- explicit support roots and artifact paths;
- existing safe bounded read behavior.

Plan 12-02 should add Environment-specific Skill availability and bounded support/source inspection without adding a Skill execution engine.

---

*Phase: 12-skill-runtime-completion*
*Plan: 12-01 complete; 12-02 next*
