# 20-01 — Bounded Environment context bundle Core

Date: 2026-09-12
Planning baseline: `b0e675d` (`docs: plan Phase 20 agent context bundle`)
Status: complete
Commit node: `feat(context): add bounded Environment context bundle`

## Delivered

- Added shared `EnvironmentContextRequest` / `EnvironmentContextBundle` DTOs and `Service.EnvironmentContextBundle`.
- The bundle composes stable Environment/Workspace identity, the Phase-19 Environment tree digest, compact capability summaries, enabled MCP/Skill summaries, verifier summaries and factual operation guidance.
- Tree budgets default to depth 3 / 1000 visited entries / 80 digest entries / 65536 output bytes, with hard caps 8 / 10000 / 500 / 262144 and minimum output budget 4096 bytes.
- Added deterministic per-section caps and total serialized byte compaction. Stable IDs and roots are never string-truncated; whole lower-priority items are omitted and counted, and partial coverage is explicit.
- Added a passive capability-report path used only by the context bundle. It reuses existing capability truth while reporting Git as `degraded/not_observed` instead of executing Git. Existing public `EnvironmentCapabilityReport` behavior is unchanged.
- Core MCP summaries are static and explicitly `not_observed`; 20-01 does not connect, probe or call MCPs. Skill summaries expose availability and bounded artifact/support metadata only, never `SKILL.md` contents. Memory values are never included; only private-memory count is exposed.
- Existing writer state may be observed, but bundle generation does not acquire, heartbeat or renew a writer. No verifier, process, Run, Git command or mutation is executed.

## Acceptance evidence

| Gate | Evidence |
|---|---|
| C01 plain non-Git / no optional prerequisites | `TestEnvironmentContextBundlePlainNonGitNeedsNoOptionalCapabilityOrWriter` validates identity, bounded tree, file guidance, explicit empty arrays and no writer requirement. |
| C02 safe populated summaries | `TestEnvironmentContextBundleSummariesAreSafeAndDoNotExecute` covers MCP, Skill, Memory, verifier and active-writer fixtures; sentinel secrets/content/args do not appear. |
| C03 authority failures | `TestEnvironmentContextBundleRejectsInvalidIdentityPathRootAndBudgets` plus `TestEnvironmentContextBundleRejectsTamperedManagedWorktree` cover unknown IDs, path escape/absolute path, missing root, invalid budgets and managed-root tamper. |
| C04 traversal/output budgets | `TestEnvironmentContextBundleBudgetsSectionCapsAndStableOrdering` exercises depth, entry, digest and 4096-byte total output limits with explicit omission/coverage facts. |
| C05 section caps | Same test creates 80 MCPs, 80 Skills and 80 verifiers and verifies MCP 64, Skill 64, verifier 32 and capability-issue 64 caps plus omission counts. |
| C06 optional failure locality | Plain non-Git and populated fixtures retain usable file context while Git is passive `not_observed` and optional MCP/Skill/verifier states remain local. |
| C07 no side effects | State bytes are byte-for-byte unchanged, active writer expiry is unchanged, verifier side-effect helper is not executed, and a real `httptest` MCP endpoint observes zero requests during bundle composition. |
| C08 deterministic ordering | Repeated static bundles compare equal after timestamp normalization. |

## Validation

- Focused context/discovery: `go test -count=1 ./internal/app ./internal/model ./internal/workspace -run "EnvironmentContext|Discovery|TreeDigest"` — PASS.
- Core regression: `go test -count=1 ./internal/app ./internal/workspace ./internal/environment ./internal/catalog ./internal/verifier` — PASS.
- Context repeat stress: `go test -count=3 ./internal/app -run "^TestEnvironmentContextBundle"` — PASS.
- Full repository: `go test -count=1 ./...` — PASS, final-tree run `run_5f025df7d6c4d3f0` (Gateway 93.606s).
- Vet: `go vet ./...` — PASS, final-tree run `run_6fbbc979ec9b272c`.
- An earlier non-final full run `run_935b3b873c8bd9d7` hit a Desktop loopback connection timing failure in `TestClientAdapterUsesAdminMCPAndDoesNotFallbackAfterDisconnect`; the same test passed three times standalone, the whole Desktop package passed five times, and the final full rerun above passed. `run_562541c2645cfb30` is also excluded because it overlapped an in-progress DTO edit and copied a transient inconsistent tree into the nested verifier fixture.
- `git diff --check` and `git diff --cached --check` — PASS before commit.

## Boundary / next step

20-01 exposes no Agent/Admin MCP tool and adds no CLI/Desktop surface. It does not create hidden current-Environment state, inject Memory values or full Skill instructions, or start Phase 21. 20-02 is the next plan and may add Gateway-owner passive observation enrichment, the explicit Agent/Admin `environment_context_bundle` tool and static server usage guidance. No push/tag/release was performed in 20-01.
