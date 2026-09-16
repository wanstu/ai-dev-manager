---
phase: 01-skill-runtime
plan: 01
subsystem: runtime
tags: [skill, mcp-gateway, gsd, security, wails]
requires: []
provides:
  - real Skill discovery from explicit roots using SKILL.md artifacts
  - stable global Skill identity and source/support-root metadata
  - Environment-gated Skill list/read over the ADM Gateway
  - real-host GSD acceptance through Streamable HTTP MCP
affects: [verifier-runtime, external-agent-dogfood, gsd-runtime, desktop-manager]
actuals:
  tokens: 16652
  tasks: 10
  commits: 1
tech-stack:
  added: []
  patterns:
    - explicit discovery roots instead of arbitrary host scans
    - authorization by Environment selection plus canonical path containment
    - real-host acceptance gated by explicit environment variables
key-files:
  created:
    - internal/skill/service.go
    - internal/skill/service_test.go
  modified:
    - internal/model/types.go
    - internal/catalog/service.go
    - internal/app/service.go
    - internal/gateway/server.go
    - cmd/ai-dev-manager/main.go
    - cmd/ai-dev-manager-desktop/frontend/app.js
    - docs/PRODUCT_CONTRACT.md
key-decisions:
  - "Skill identity is stable by normalized Skill name; catalog entries point to real artifacts instead of copying instructions."
  - "GSD needs two explicit host boundaries on this machine: the OpenCode skills discovery root and sibling gsd-core support root."
  - "Environment selection remains the authorization gate; support roots do not grant arbitrary host filesystem access."
  - "The real-host GSD acceptance is opt-in through ADM_REAL_GSD_SKILLS_ROOT / ADM_REAL_GSD_SUPPORT_ROOT so ordinary test runs stay machine-independent."
patterns-established:
  - "Skill discovery: canonical explicit root -> recursive SKILL.md discovery -> stable catalog entries."
  - "Skill read: enabled Environment -> catalog entry -> canonical artifact/support-root containment -> bounded text read."
requirements-completed:
  - SKILL-RUN-01
  - SKILL-RUN-02
  - SKILL-RUN-03
  - SKILL-RUN-04
  - SKILL-RUN-05
coverage:
  - id: D1
    description: "Skills are discovered only from explicitly configured roots and resolve to real SKILL.md artifacts with stable identity."
    requirement: "SKILL-RUN-01"
    verification:
      - kind: unit
        ref: "internal/skill/service_test.go"
        status: pass
      - kind: unit
        ref: "internal/catalog/service_test.go"
        status: pass
    human_judgment: false
  - id: D2
    description: "Environment selection gates Skill list/read, two enabled Environments share one installation, and disabled/escape/broken-artifact cases fail locally."
    requirement: "SKILL-RUN-03"
    verification:
      - kind: integration
        ref: "internal/gateway/server_test.go#TestGatewayUsesOnlyEnabledConfiguredExternalMCPAndSkillRuntime"
        status: pass
    human_judgment: false
  - id: D3
    description: "The real installed gsd-next Skill and gsd-core smart-entry workflow are consumable through a real Streamable HTTP ADM Gateway client."
    requirement: "SKILL-RUN-05"
    verification:
      - kind: e2e
        ref: "ADM_REAL_GSD_SKILLS_ROOT + ADM_REAL_GSD_SUPPORT_ROOT go test ./internal/gateway -run TestRealHostGSDSkillThroughHTTPGateway"
        status: pass
    human_judgment: false
  - id: D4
    description: "CLI and Desktop management configure real Skill discovery/support roots instead of hand-entered instruction text."
    requirement: "SKILL-RUN-02"
    verification:
      - kind: integration
        ref: "cmd/ai-dev-manager/main_test.go#TestCatalogCLIManagesGlobalMCPAndSkill"
        status: pass
      - kind: integration
        ref: "internal/desktop/adapter_test.go#TestAdapterExposesManagementBoundaryWithExplicitMemoryReads"
        status: pass
    human_judgment: false
duration: 1h20m
completed: 2026-09-06
status: complete
---

# Phase 1 Plan 01: Real Skill Vertical Slice Summary

**Real global Skill artifacts now flow from explicit host roots through Environment authorization to the ADM Gateway, proven against the installed OpenCode GSD suite.**

## Performance

- **Duration:** 1h20m
- **Completed:** 2026-09-06
- **Tasks:** 10
- **Files modified:** 21 implementation files plus planning state

## Accomplishments

- Replaced the product-facing text-only Skill path with explicit discovery roots, real `SKILL.md` artifacts, stable Skill IDs, canonical source metadata, and bounded supporting-file reads.
- Added `environment_skill_list` and `environment_skill_read` with enabled/disabled Environment isolation, shared-installation reuse, path-escape rejection, and broken-artifact failure isolation.
- Proved the vertical slice against the actual host installation at `C:\Users\wanstu\.config\opencode\skills` with explicit `C:\Users\wanstu\.config\opencode\gsd-core` support access over a real Streamable HTTP MCP client.
- Replaced CLI/Desktop Skill instruction entry with real root/support-root configuration while leaving unrelated Desktop work frozen.
- Consumed `gsd-next` and `smart-entry.md` through ADM, which exposed and then allowed correction of the manually bootstrapped planning schema.

## Task Commits

1. **Real Skill Vertical Slice** - `1fbcceb` (`feat(skill): add real environment-scoped skill runtime`)

## Files Created/Modified

- `internal/skill/service.go` - discovery, stable identity, canonical path authorization, bounded Skill content reads.
- `internal/skill/service_test.go` - discovery, stable ID, path containment, and binary/bounds tests.
- `internal/catalog/service.go` - imports discovered real Skills into the global catalog.
- `internal/app/service.go` - resolves Environment-enabled Skills and enforces selection before reads.
- `internal/gateway/server.go` - Agent-facing Skill list/read tools.
- `internal/gateway/server_test.go` - Environment isolation plus real host GSD HTTP acceptance.
- `cmd/ai-dev-manager/main.go` - CLI `skill add --root ... --support-root ...` management path.
- `cmd/ai-dev-manager-desktop/frontend/*` - minimal real-root Skill management adjustment.
- `docs/PRODUCT_CONTRACT.md` - real Skill artifact/runtime acceptance contract.

## Decisions Made

- Treat the globally installed GSD suite as many `gsd-*` Skills, not a fictional single `gsd/SKILL.md`.
- Keep discovery root and support roots distinct because GSD Skill entrypoints reference the sibling `gsd-core` tree.
- Authorize only the selected Skill artifact directory and explicitly configured support roots; path text supplied by an Agent is never authorization.
- Keep real-host acceptance optional in the normal suite but mandatory for this Phase completion gate.

## Deviations from Plan

### Auto-fixed Issues

**1. Real GSD installation shape differed from the initial assumption**
- **Found during:** Task 1 host inspection.
- **Issue:** GSD is an OpenCode suite of many `gsd-*` Skill directories and references sibling `gsd-core` workflows.
- **Fix:** Split discovery root from explicit support roots and updated the Phase context/plan accordingly.
- **Verification:** Real `gsd-next` + `gsd-core/workflows/smart-entry.md` HTTP Gateway acceptance passes.
- **Committed in:** `1fbcceb`.

**2. Manual planning bootstrap was not actually GSD-native**
- **Found during:** Task 10, after consuming `gsd-next` through ADM.
- **Issue:** Real `gsd-tools smart-entry` initially returned `needs-first-phase` because ROADMAP/STATE did not match installed GSD templates.
- **Fix:** Normalize ROADMAP/STATE to the actual GSD schema before Phase transition.
- **Verification:** `gsd-tools smart-entry --json` now reports `Phase 1 of 13 · executing` and recommends `/gsd:progress --next`.
- **Committed in:** pending Phase transition documentation commit.

---

**Total deviations:** 2 auto-fixed correctness/bootstrap issues.
**Impact on plan:** Both were required to satisfy the real GSD acceptance case; neither expands product scope beyond Phase 1.

## Issues Encountered

- `go fmt ./...` touched unrelated historical formatting files; those changes were explicitly reverted before verification/commit.
- `git diff --check` caught Markdown hard-break trailing spaces in the normalized ROADMAP; they were removed.

## User Setup Required

None for normal ADM operation beyond choosing explicit Skill roots. The real-host acceptance fixture uses this machine's existing OpenCode GSD installation.

## Next Phase Readiness

- Skill runtime is ready for external-Agent use and no longer counted as partial CRUD-only functionality.
- Phase 2 can focus on structured verifier runtime without extending Skill or Desktop scope.
- Phase transition still requires canonical GSD verification/UAT and STATE/ROADMAP advancement.

---
*Phase: 01-skill-runtime*
*Completed: 2026-09-06*
