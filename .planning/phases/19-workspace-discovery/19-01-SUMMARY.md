# 19-01 — Bounded discovery Core

Date: 2026-09-12
Implementation baseline: 660aadc (master). Source: baseline plus the implementation in this summary's commit; use Git log for the resulting commit identity.
Status: complete; local validation passed with the documented OS-dependent permission skip.

## Delivered

Requirements ADM-GOAL-001 and ADM-CORE-001/002/003/004/005/006: registered Workspace discovery and Runtime-authorized Environment directory digest. No new prerequisite, persistence, writer requirement, executable invocation, file-content read or Environment creation in these operations.

Added model DTOs, the workspace metadata scanner and app entry points DiscoverWorkspace / EnvironmentTreeDigest. os.Root pins the authority boundary; metadata link checks include Windows reparse points. Directory enumeration uses batches of at most 64 and the remaining entry budget. Root read failures are operation failures; child failures/cancellation yield explicit partial evidence.

Plan budgets retained: defaults 4/2000/50/100/65536; hard caps 8/10000/200/500/262144; minimum JSON budget 4096. Calibration: at most 64 marker observations per candidate and 32 diagnostics, with omission counters. Output drops whole items rather than shortening actionable roots. Counts are observed metadata, not complete filesystem totals.

Candidates merge by relative directory path; nested projects and duplicate basenames remain distinct. Literal query rank, marker evidence and paths determine final ordering. Partial traversal may depend on OS enumeration order. Exclusion policy is returned. Slow OS filesystem calls are not claimed to be preemptible.

## Acceptance evidence

| Gate | Evidence |
|---|---|
| A01 | Non-Git fixture and app authority test discover p1/p2 with no configured optional capabilities or writer. |
| A02 | Empty directory, root marker, merged markers, nested projects and duplicate basename tests preserve relative roots. |
| A03 | Exact path/name and substring ranking; empty match result; narrower subpath identity. |
| A04 | Invalid IDs, absolute/parent paths, file roots, link targets and tampered managed worktree rejected; Environment digest excludes siblings. Windows symlink test passed. Junction-specific creation was not separately exercised. |
| A05 | 150-directory fixture independently exercises entry/candidate/digest/byte caps; depth and marker caps tested. Five-entry budget records exactly five visits and one read batch; JSON stays within 4096 bytes. |
| A06 | Pre-cancel, mid-scan cancel and disappearing child tested deterministically. Permission test skipped on this Windows account because chmod cannot enforce denial; OS permission branch remains a platform limitation in local evidence. |
| A07 | Harmless sentinel file contents absent from output; state bytes unchanged and no Environment auto-created. |
| A08 | Repeated complete scans match after removing observation timestamp; partial results disclose coverage and stop reasons. |

## Validation

- Focused workspace/app/pathutil: PASS, run_a69f53c000589ad5; permission test SKIP as described above.
- Final full Go: PASS, run_21ea236e2e208fda (includes final root-read error and optional-Git test corrections).
- go vet ./...: PASS, run_ce40800fe8ecf487.
- git diff --cached --check: PASS; reviewed 14-file staged change.

## Handoff

19-01 delivers Core only. 19-02 Agent/Admin MCP, CLI and Desktop surfaces are not implemented in this slice. No Wails artifact is claimed. The existing MCP global probe, error badges and Skill structural availability semantics are unchanged.
No push/tag/release. Phase 19 remains incomplete until 19-02 acceptance.
