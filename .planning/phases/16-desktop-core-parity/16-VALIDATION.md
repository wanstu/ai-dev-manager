# Phase 16 Validation — Desktop Core Parity + 1.0 RC Readiness

## Global gates

Before any Phase 16 implementation node is closed:

1. `go test -count=1 ./...` passes, unless the node is documentation-only and `git diff --check` is sufficient.
2. `go vet ./...` passes for code changes.
3. `git diff --check` passes, allowing only platform line-ending warnings and no whitespace errors.
4. CLI and Desktop smoke builds pass when build or frontend behavior changes.
5. `git status` is clean after commit.
6. No push unless explicitly requested.

## 16-01 CI baseline gates

- GitHub Actions workflow exists under `.github/workflows/`.
- Workflow runs on `master`, pull request and manual dispatch.
- Workflow runs Windows `go test -count=1 ./...`.
- Workflow runs `go vet ./...`.
- Workflow builds Windows CLI and Desktop smoke binaries.
- Workflow uploads short-retention artifacts.
- Workflow does not publish releases, deploy, push commits or require secrets.

## 16-02 MCP/Skill visual management gates

### MCP UI gates

- Desktop can list global MCP definitions with name/id, transport, sanitized config summary, default include state and selected Environment enablement.
- Desktop can add a basic MCP through transport-aware fields instead of a raw one-size-fits-all endpoint field.
- Desktop can import supported MCP JSON/JSONC through a preview/apply flow backed by existing Core import semantics.
- Desktop can show preview candidates, warnings, errors and reference requirements before apply.
- Desktop can toggle default include without implying existing Environments changed.
- Desktop can toggle one MCP for the selected Environment without implying the global definition changed.
- Desktop can show MCP health/status/probe result and `CapabilityReport` reason for `mcp/<id>`.
- Desktop never displays resolved secret values or private Memory values in MCP views.

### Skill UI gates

- Desktop can list Skill sources with root, support root count, last refresh result/time and discovered artifact count where available.
- Desktop can add a Skill source with support roots.
- Desktop can refresh one Skill source and display success/failure without hiding unrelated valid Skills.
- Desktop can remove a Skill source with copy that says disk files are not deleted.
- Desktop can list discovered Skills with source/artifact facts.
- Desktop can toggle one Skill for the selected Environment without implying the global source changed.
- Desktop can show Skill availability state/reason/message and `CapabilityReport` reason for `skill/<id>`.
- Desktop never displays private Memory values in Skill views.

### UX safety gates

- Labels distinguish global definition/source, default include and selected Environment enablement.
- Destructive actions require explicit confirmation and explain whether they remove ADM metadata or affect only one Environment.
- Empty states tell the user how to add/import MCPs or add Skill sources.
- Disabled/unconfigured/unavailable/degraded states have visible reasons, not only icons.
- UI works without hover-only controls.

## Manual smoke checklist for 16-02

1. Start Desktop.
2. Open MCP management.
3. Add one HTTP MCP definition.
4. Import a valid MCP JSON/JSONC config and apply after preview.
5. Import an invalid config and confirm apply is blocked with errors.
6. Toggle MCP default include and confirm copy says it affects new Environments only.
7. Toggle MCP selection for one Environment and see status/reason.
8. Open Skill management.
9. Add one Skill source.
10. Refresh the source and see discovered Skills or refresh errors.
11. Toggle Skill selection for one Environment and see availability/reason.
12. Confirm normal MCP/Skill views do not reveal private Memory values or resolved secret values.

## Post-16-02 RC gates

After MCP/Skill visual management is usable, continue Phase 16 with remaining RC blockers only:

- capability report display outside MCP/Skill detail if still needed;
- verifier/process/run minimal visibility or documented fallback;
- known limitations / RC notes;
- final local RC smoke build and CI observation after push when requested.
