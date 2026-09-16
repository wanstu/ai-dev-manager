# 19-02 — Agent, CLI and Desktop navigation

Date: 2026-09-12
Implementation baseline: `f8b3197` (19-01 Core complete)
Shared-surface commit: `3537c54 feat(discovery): expose workspace discovery through Agent and CLI`
Final implementation commit: `8a5c99a feat(desktop): add explicit project discovery and root selection`
Status: complete; automated acceptance passed. Native Wails click-through is recorded pending because this session has no native GUI-control tool.

## Delivered

Phase 19 discovery is now exposed through the existing Core authority on Agent MCP, Admin MCP, normal CLI and Desktop without adding a new prerequisite, state file or authorization path.

- Agent and Admin MCP expose read-only `workspace_discover` and `environment_tree_digest` with the same bounded `DiscoveryRequest` / `DiscoveryReport` contract. Neither operation acquires a writer or probes executable/verifier/MCP/Skill capabilities.
- The shared Admin MCP client and management service wrap both reads. Normal CLI `workspace discover` and `environment tree-digest` use the connected Admin MCP path only; there is no local writable fallback.
- Desktop Workspace rows now have an explicit **Discover projects** action. Opening the dialog is unloaded; only explicit submit scans. Path/query/budget edits and loaded-result filtering do not scan.
- Desktop renders stable scope, scan path, effective limits, observation time, candidate marker evidence, compact digest, partial coverage and stop reasons. Empty, query no-match, partial, loading and failure states are distinct.
- Discovery request identity includes connection generation, Workspace ID and dialog generation. Late results are rejected after Workspace changes, profile changes or close/reopen. Duplicate submissions are disabled while pending.
- **Use root** copies the exact candidate relative root and stable Workspace ID into the existing Create Environment form. It does not submit, create an Environment or retarget the current Management Environment. Explicit Create resolves that relative root under the registered Workspace before existing Core containment validation.
- Existing exact-root Environment matches are shown by stable identity and can be navigated explicitly.
- Environment detail has an explicit directory-summary action backed by `environment_tree_digest`; opening detail or Diagnostics never triggers it automatically. Digest results use detail/connection generation guards.
- Responsive discovery/digest presentation keeps long candidate names and narrow/scaled layouts bounded without whole-window horizontal overflow.

## Integrated acceptance — B01–B10

| Gate | Result / evidence |
|---|---|
| B01 | PASS. In-memory real MCP client sessions for Agent and Admin call the same disposable non-Git Workspace fixture and return equivalent bounded reports after removing observation time. Fixture does not configure writer/exec/verifier/MCP/Skill prerequisites. |
| B02 | PASS. Core tests reject unknown Workspace/Environment IDs and out-of-scope paths. Gateway tests cover unknown IDs plus Workspace and Environment `..` traversal. CLI tests cover the same unknown-ID and traversal failures through Admin MCP. |
| B03 | PASS. Production browser smoke asserts unloaded → explicit loading → results, no-match, empty, partial and failure. Counters prove dialog open, typing, local filtering and option edits do not scan. |
| B04 | PASS. Production browser smoke delays Workspace A discovery, closes/reopens on Workspace B and rejects A's late result; duplicate submit remains one call. A delayed discovery before profile A→B is drained and old discovery scope is closed before the new profile snapshot is shown. A close/reopen event-order race found at 125% scale was fixed by ignoring stale `close` events when the dialog has already reopened. |
| B05 | PASS. Selecting `apps/p2` pre-fills Workspace `ws-a` and exact relative root `apps/p2`; no Create call occurs before explicit submit and Management Environment is unchanged. Fixture Create failure preserves Workspace/root/name form values; the explicit submitted root is resolved under the registered Workspace for existing Core revalidation. |
| B06 | PASS. 19-01 covers nested/markerless/duplicate-name discovery. Production browser smoke covers long candidate names, local filters, focus restoration, 820x560 and 125% scale with no discovery-row/dialog horizontal overflow. |
| B07 | PASS. Environment digest is Runtime-authorized to its Environment root, rejects sibling traversal, and 19-01 app acceptance rejects tampered managed-worktree roots. Desktop digest is explicit and stale detail results cannot cross close/reopen scope. |
| B08 | PASS. Existing Desktop regression smoke still covers global MCP probes independent of Environment selection, unresolved-reference badge precedence/recovery, global Skill structural availability, explicit Memory reads, Runtime stale-scope behavior, Management Environment ownership and connection switches. Full repository Go tests pass. |
| B09 | PARTIAL by plan wording: uniquely named CLI and Wails production artifacts built successfully. Native Wails/WebView2 click-through is **pending**, not PASS, because this chat session has no native GUI-control/screenshot tool. The active Gateway was not replaced or terminated to manufacture native evidence. |
| B10 | PASS. This summary and `19-CLOSEOUT.md` record code commits, A/B results, run IDs, final artifact paths/SHA-256 and the native pending note; STATE/PROJECT/ROADMAP are updated only for delivered Phase 19 status. |

## Final validation on `8a5c99a`

- `go test -count=1 ./...`: PASS — `run_4df936e2226f72ec`. Gateway package completed in ~96s; this explains the earlier synchronous 120s tool-window timeout, which was not counted as a result.
- `go vet ./...`: PASS — `run_f84bb4ef0561b86d`.
- Production Chromium smoke `node tests/desktop-ui/browser-smoke.cjs`: PASS — `run_105573529a6a9764`; 184 checks each at 1120x760, 820x560 and 1120x760 @ 125% scale.
- Desktop helper regression: PASS, 20/20 (`node --test tests/desktop-ui/*.test.cjs`) on the committed Desktop implementation.
- Focused discovery/Gateway/CLI/Desktop Go suites: PASS during implementation, including final symmetric B02 additions.
- CLI unique build: PASS — `run_c8a7c2806498667f`.
- Wails v2.15.0 production build: PASS — `run_88c7536254ea3e9d`. Direct `wails` execution was denied by the existing Runtime allowlist, so the same pinned Wails CLI was invoked through the already-allowed Go executable: `go run github.com/wailsapp/wails/v2/cmd/wails@v2.15.0 build -clean ...`. No allowlist change was made.
- `git diff --check`: PASS before implementation commits; final docs closeout reruns this gate before the docs commit.

## Artifacts

- CLI: `ai-dev-manager-phase19-02-8a5c99a.exe`
  - size: 16,717,824 bytes
  - SHA-256: `84f2ef07f0425a6e8ff3bdcf31c0bda42732adb86e2ba4834b0b8a8ba00cd17d`
- Wails Desktop: `cmd/ai-dev-manager-desktop/build/bin/adm-desktop-phase19-02-8a5c99a.exe`
  - size: 17,639,936 bytes
  - SHA-256: `2564136b5d2a3bb375a24b96e30731d3eb5744e5b4e9324a09ba3f1e2253d210`

## Boundary notes

Discovery remains metadata-only and explicit. It does not read file contents, acquire writers, execute commands, run verifiers, probe MCPs, evaluate Skills, read Memory values, create Environments automatically, change current Management Environment or introduce task orchestration. Phase 20 context injection was not started.

No push, tag or release was performed.
