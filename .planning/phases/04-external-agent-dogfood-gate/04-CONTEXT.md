# Phase 4: External Agent Dogfood Gate — Context

## Scope and requirements
Prove ADM-GOAL-001, ADM-DEV-004, ADM-CORE-003/005/006/007/008/012/013 and R1 Skill/verifier consumption through an actual external-Agent development loop. The current ChatGPT session is the external Agent; no nested LLM is permitted.

## Locked decisions
- Project I/O, search, edits, Git, commands and verification use ADM only. For tools absent from the connector, an allowlisted PowerShell command may be a thin HTTP client to ADM's own Gateway. It must not replace ADM file operations with direct filesystem operations.
- Read the real installed gsd-next Skill and authorized smart-entry workflow through environment_skill_read; consume deterministic smart-entry state to select Phase 4 planning. User has already selected manual planning/execution; do not launch the recommended LLM workflows or claim those gates ran.
- Existing D:/projects/adm-v2-dogfood-nongit registration is stale: the directory was missing on 2026-09-07. Do not reuse its historical acceptance.
- Build a separate non-Git Go CLI, D:/projects/adm-verifier-report, that reads an ADM verifier result envelope from stdin and exits 0 only for a complete passed result, 1 for failed/skipped/timed-out results, 2 for malformed/incomplete input. This is a real consumer of this run's verifier evidence, not an ADM runtime abstraction.
- Tests must drive one observed failure and a follow-up implementation correction before the structured verifier passes.
- Store/re-read task context in Environment-private Memory. Prove restart persistence on a task-owned Gateway/state, not by interrupting the shared Gateway used by other Environments.
- No external upstream MCP is needed for this task; conditional external MCP gating acceptance is N/A. ADM's own HTTP transport is not an upstream filesystem/exec MCP.
- Desktop/package work stays frozen. Stop at the R1 milestone for review; do not begin Phase 5.

## Non-goals and prerequisites
No new ADM product prerequisite, no provider/model calls, no GSD agents, no GitNexus repair, no verifier timeout changes, no generic process manager, no Desktop or packaging work.
Go and PowerShell are existing allowlisted capabilities required only by this task; Git is optional for the target.

## Initial findings
- Phase 4 branch is feat/external-agent-dogfood-gate; worktree clean before planning.
- Shared Gateway on 127.0.0.1:41137 has Skill and verifier tools but lacks environment_mcp_status; connector schemas expose an older subset.
- Source at the Phase 3 baseline contains all three runtime families. Use a freshly built task-owned Gateway for final R1 acceptance; leave the shared Gateway running.
- GSD smart-entry reports planning, Phase 4 of 13 needs a plan.
- Windows verifier startup flake is a concern only; no stable regression established.

## Blocker amendment — 2026-09-07
Two independent runs (full suite and focused -count=1) failed TempDir cleanup of verifier_timeout_fixture because Windows retained a running process. Runtime.Exec uses exec.CommandContext and Windows configureCommand only hides the console; cancellation has no process-tree handling. Add bounded Windows cancellation for the owned command tree using Windows' built-in taskkill /T /F, with direct-process kill fallback if cleanup fails; no change to timeout classification, no global Environment prerequisite and no new installed dependency. This is short-lived command cancellation only, not Phase 5 persistent lifecycle. Add a Windows parent/child cancellation regression, retain the real HTTP acceptance unchanged, then rerun it and the full suite. No retries/sleeps are added to acceptance cleanup to mask leaked processes.
User additionally authorizes @pj only to repair @pjadm when unusable; return to @pjadm afterward. @pjadm remained usable here, so no fallback was needed.
