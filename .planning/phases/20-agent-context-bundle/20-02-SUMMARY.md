# 20-02 — Gateway context surface and capability guidance

Date: 2026-09-12
Implementation head: `20ae96d` (`feat(gateway): expose Agent context bundle and guidance`)
Status: complete

## Delivered

- Added Gateway-owner `ContextBundle` composition over the committed 20-01 Core bundle.
- Context capability facts use the same Gateway-owner enrichment logic as `environment_capability_report`, but start from the new passive no-Git capability report so context generation never executes Git.
- Enabled MCP summaries are enriched only from existing owner-local observations. Tool inventory contributes bounded tool **names** only; descriptions/schemas are not copied. Missing observations stay `not_observed`.
- Added read-only `environment_context_bundle` to the shared Agent/Admin MCP surface. Input is stable `environment_id` plus the 20-01 bounded tree/output budgets; no writer is required.
- Added static MCP initialize instructions through `mcp.ServerOptions.Instructions`. Instructions explain stable Workspace/Environment IDs, the explicit context bundle workflow, local optional-capability failures, writer-gated mutation and the no-orchestration boundary. They contain no Environment-specific paths, IDs, Memory, catalog data or owner observations.
- Gateway enrichment re-runs Core compaction after owner data is projected, so observed tool names cannot bypass section or total byte limits.
- No CLI/Desktop context feature was added; Phase 23 remains the script-friendly CLI context direction.

## G01-G12 acceptance evidence

| Gate | Evidence |
|---|---|
| G01 Agent/Admin parity | `TestGatewayEnvironmentContextAgentAdminParityInstructionsAndAuthority` connects real in-memory MCP ClientSessions to both surfaces. Both expose `environment_context_bundle` and return the same stable Environment/Workspace context semantics for a disposable non-Git fixture without writer/exec/verifier/MCP/Skill prerequisites. |
| G02 authority rejection | The same test verifies unknown Environment IDs and `path:".."` are rejected on both Agent and Admin surfaces through the existing Core authority. |
| G03 no-observation / zero traffic | `TestGatewayEnvironmentContextUsesOnlyExistingMCPObservationAndRecompacts` enables an HTTP MCP backed by a request counter. Context before any owner observation reports `not_observed`; upstream request count remains exactly 0. |
| G04 observed inventory / no extra calls | The fixture records an owner observation with 160 synthetic tools via owner observation state, then calls context again. Tool names appear with observation freshness/state; upstream request count remains 0 before and after context. Tool descriptions are absent. |
| G05 Skill/verifier safety | `TestGatewayEnvironmentContextDoesNotLeakMemorySkillContentOrExecuteVerifier` verifies enabled Skill identity/support facts and verifier ID/state are present while `SKILL.md` sentinel content and verifier args are absent; verifier side-effect file is never created. |
| G06 static instructions | Agent and Admin `InitializeResult().Instructions` contain the stable-ID/context guidance and `environment_context_bundle`, but do not contain the fixture Environment ID, Workspace ID or root path and never promise implicit current-project state. |
| G07 writer observation only | An existing writer lease remains byte-for-byte equivalent in owner/expiry/last-seen semantics after context; context neither acquires nor heartbeats it. The compact bundle exposes writer presence/expiry but not the writer owner string. |
| G08 no hidden side effects/data injection | Global/private Memory sentinels and Skill content are absent; verifier does not execute; Gateway owner MCP-session/process/Run counts remain unchanged; Git uses the passive `not_observed` projection; no Skill refresh, MCP probe or Environment retarget occurs. |
| G09 output/omission after enrichment | 160 long observed tool names hit the 128 aggregate tool-name cap and report `mcp_tool_names: 32` plus `mcp_tool_name_limit`. A 4096-byte request reports `output_byte_limit` after owner enrichment. |
| G10 regressions | Focused capability/context/surface tests, full `internal/gateway`, full repository Go tests and vet all pass on the final implementation tree. Existing Agent/Admin privilege separation remains unchanged; the context tool is shared/read-only. |
| G11 fixed-head artifact | `ai-dev-manager-phase20-02-20ae96d.exe`, 16,817,152 bytes, SHA-256 `79F1377506E59EEBA8EF643999822191937AF13C1A70556DC579064564110F97`. Built with `go build -trimpath -o ai-dev-manager-phase20-02-20ae96d.exe ./cmd/ai-dev-manager`. |
| G12 closeout | This summary plus `20-CLOSEOUT.md`, STATE/PROJECT/ROADMAP/post-1.0 phase map record implementation commits, run IDs, negative counters, initialize evidence and artifact hash. |

## Validation

- Focused final-head context/capability/surface suite: `go test -count=1 ./internal/app ./internal/gateway -run "EnvironmentContext|CapabilityReport|SeparatesAgentAndAdminMCPPaths"` — PASS, run `run_456774d2120b3a8f`.
- Full Gateway: `go test -count=1 ./internal/gateway` — PASS, run `run_0be1e9a1f9f95c94`, 109.990s.
- Full repository: `go test -count=1 ./...` — PASS, run `run_34634311551124f4`; Gateway 108.134s.
- Vet: `go vet ./...` — PASS, run `run_ec2cf832e35190a6`.
- `git diff --check` and staged diff check — PASS around implementation/closeout commits.
- A synchronous full-Gateway invocation exceeded its outer tool timeout and is not acceptance evidence; the async Gateway run above completed successfully.

## Boundary

Phase 20 does not add hidden current-Environment state, dynamic initialize-time project data, automatic Memory composition, full Skill instruction injection, MCP probing on context reads, task planning/orchestration, CLI context output or Desktop context UI. No push/tag/release was performed. Phase 21 is not started by this closeout.
