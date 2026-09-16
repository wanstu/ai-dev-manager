# 23-02 Summary — Environment Agent workflows and Phase 23 integrated acceptance

Date: 2026-09-13
Implementation baseline: `c1c3978` (`docs: record Phase 23-01 CLI acceptance`)
Final implementation head: `f85c566` (`feat(cli): expose environment agent workflows`)
Status: complete and validated; Phase 23 ready for docs closeout.

## Delivered

- Added thin `internal/adminmcp.Client` wrappers for the existing shared `environment_context_bundle` and `environment_temporary_create` tools. Existing temporary status/promote/cleanup wrappers are reused; no second protocol, local writable fallback, or CLI-only state path was introduced.
- Added `adm environment context --environment-id ...` with the Phase-20 canonical `model.EnvironmentContextRequest` budgets and `model.EnvironmentContextBundle` JSON result. Scope remains explicit by stable Environment ID; cwd is not an authorization or selection mechanism.
- Corrected stale `environment capability-report` help: normal production CLI already calls the Admin-MCP/Gateway capability-report path and may include existing owner-local observations, while still remaining side-effect-free with respect to reconnect/probe/tool/verifier operations.
- Added `adm environment temporary` lifecycle projection over Phase 22:
  - `create` requires explicit Workspace ID, name, lifecycle owner and positive TTL; optional session/run fields remain provenance only; `mode`, `root` and `base-ref` are passed to the existing server contract without caller control of managed-worktree destination/branch or creator surface.
  - `status` is read-only and owner-free.
  - `promote` requires the explicit lifecycle owner and changes retention only.
  - `cleanup` defaults to preview; `--execute` is the only execution switch. There is no force flag or hidden override.
- Help explicitly states that ordinary existing-root cleanup does not delete project directories/files, while managed-worktree cleanup retains the generated branch and reuses server-side safety.
- All new successful management commands emit the canonical shared models through the existing JSON stdout path; errors remain non-zero through the existing CLI error path.

## Real CLI/Admin-MCP acceptance

`cmd/ai-dev-manager/phase23_environment_cli_test.go` uses a real disposable HTTP Gateway with a persistent runtime owner and calls the production top-level CLI target selection path.

- E01/E02 — `environment context` returns the canonical bundle for one explicit non-Git Environment; unknown ID and `..` traversal fail through existing authority.
- E03 — Global/private Memory sentinels and writer-owner identity are absent from output; only safe bounded facts/counts remain.
- E04 — context does not renew/change an existing writer lease. Gateway owner startup may reconcile selected MCPs, so the acceptance snapshots protocol-request count immediately before the context call and proves the context call adds zero upstream MCP requests.
- E05/E06 — plain non-Git temporary create succeeds with explicit owner+TTL plus session/run provenance. Required owner/TTL validation remains explicit; existing-root mode introduces no Git/MCP/Skill/verifier prerequisite.
- E07 — requesting `managed_worktree` against the same non-Git Workspace fails locally through the existing Git-only managed mode, proving Git remains operation-local rather than a global Environment prerequisite. Phase-22 full regression continues to cover successful managed-worktree creation, retained branches and safety blockers.
- E08 — temporary status is read-only and returns the current retention/cleanup state without owner input.
- E09 — wrong-owner promote and cleanup execute are rejected; matching-owner promote preserves the same stable Environment ID and makes retention durable.
- E10 — cleanup without `--execute` is a non-mutating preview; execute removes only the selected temporary Environment after fresh server safety checks and leaves an unrelated promoted Environment untouched.
- E11 — ordinary cleanup preserves a real marker file under the Workspace/project root. Managed-worktree dirty/unpublished/tamper and retained-branch semantics remain covered by Phase-22 Gateway/full regression rather than reimplemented in CLI.
- E12 — an active writer appears as the canonical `active_writer` blocker in CLI preview and prevents eligibility; after writer release, preview becomes eligible and explicit execute succeeds. Full Phase-22 regression retains process/`run_`/`vfrun_`/MCP blocker coverage.
- E13 — context/status/create against an unavailable selected Admin MCP target return Admin-MCP connection failures and do not mutate a disposable local `ADM_V2_HOME` state.
- E14 — context/temporary help documents explicit stable scope, lifecycle owner/TTL, preview default and no-force behavior; successful outputs are valid JSON.
- E15 — fixed-head full repository regression includes all 23-01 MCP/Skill acceptance plus pre-existing workspace/environment/exec/memory/Core behavior.

## Fixed-head validation on `f85c566`

- Focused CLI/Admin-MCP/Gateway/Core context + temporary lifecycle: `run_e6dd66630f674d97` — PASS.
  - `go test -count=1 ./cmd/ai-dev-manager ./internal/adminmcp ./internal/gateway ./internal/app -run "Phase23|EnvironmentContext|TemporaryEnvironment"`
  - CLI 2.309s; Gateway 29.919s; app 4.354s.
- New Environment CLI acceptance repeat x3: `run_73ace1e85bf573f4` — PASS.
  - `go test -count=3 ./cmd/ai-dev-manager -run "^TestPhase23CLIEnvironment"`
- Full repository: `run_b7593afcb1f20f56` — PASS.
  - `go test -count=1 ./...`
  - Gateway 205.344s; all packages green.
- Vet: `run_88f9c56ff957c7a4` — PASS (`go vet ./...`).
- CLI unique build: `run_56be6a0b03f2bee9` — PASS.
- `git diff --check` and staged implementation check — PASS before the implementation commit.

## Fixed-head artifact

- CLI: `dist/ai-dev-manager-phase23-final-f85c566.exe`
  - size: 17,175,552 bytes
  - SHA-256: `6E0E54F0EBDC3A7DEF20316C24B15E188692548E3D2AEA92DD54ADB561CF690A`

No Wails/Desktop build is required for Phase 23 because no Desktop feature code changed.

## Boundary notes

Phase 23 remains a CLI surface-convergence phase over accepted Core/Gateway semantics. It adds no hidden current Environment, cwd-based authority, direct normal-mode state writes, second Admin API, universal output formatter, MCP/Skill semantic redesign, automatic context injection, task/GSD orchestration, force cleanup, automatic GC, merge/rebase/push, branch deletion, or Desktop feature scope.

No push, tag, release, or subagents were used.
