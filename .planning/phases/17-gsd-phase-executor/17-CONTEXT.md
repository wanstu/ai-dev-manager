# Phase 17 Context — GSD Phase Executor

## Goal

让 daemon-owned Agent Run 能直接消费项目 `.planning/` 状态，并执行一个可审计的 GSD phase plan，而不是把自然语言 PLAN.md 直接当作任意 shell 或隐式代码执行提示。

## Core Decision: Auditable Execution Spec

Phase 17 不绑定 Codex CLI / Claude Code / OpenAI API。GSD `PLAN.md` 继续保留自然语言说明，同时可包含一个明确的 machine execution spec：

````markdown
## Execution Spec

```json
{
  "steps": [
    {
      "id": "edit-example",
      "operation": "files.edit",
      "purpose": "Apply the planned source change",
      "input": {
        "path": "example.go",
        "old_text": "before",
        "new_text": "after",
        "expected_replacements": 1
      }
    }
  ],
  "run_verifiers": true
}
```
````

This separates:

- human/LLM planning text
- auditable structured operations
- Runtime policy enforcement
- verifier/reviewer acceptance

Future LLM planners may generate this spec, but Phase 17 execution does not require a specific model vendor.

## `gsd` Workflow

```powershell
ai-dev-manager agent run --workflow gsd [path|workspace-id]
```

The workflow:

1. Build Workspace Runtime Adapter.
2. Read `.planning/STATE.md`.
3. Resolve current phase/plan identity from `## Current Position`.
4. Locate/read `.planning/PROJECT.md`, current `CONTEXT.md`, current `PLAN.md`.
5. Parse `Execution Spec` JSON from PLAN.md.
6. Validate every operation against an explicit GSD operation allowlist and Runtime capabilities.
7. Execute steps sequentially through Runtime Adapter.
8. If `run_verifiers=true`, append `verify.run_many`.
9. Reviewer returns pass/fail from structured verifier results.
10. Run status retains planning source paths, structured Plan, StepResults and Review.

## GSD Allowed Operations — Phase 17

Only existing Runtime Adapter operations:

- `files.read`
- `search.text`
- `files.write`
- `files.edit`
- `git.status`
- `git.diff`
- `verify.run_many`

No `shell.exec`, worktree create/remove, Docker or arbitrary operation in Phase 17 GSD specs.

Runtime capability/policy remains authoritative even for an allowlisted operation.

## Planning Resolution

STATE is the source of current identity. Phase 17 parses:

- `Phase: N — ...`
- `Plan: NN-XX — ...`

The executor scans `.planning/phases` only to locate:

- one directory prefixed `NN-`
- `NN-CONTEXT.md`
- `NN-XX-PLAN.md`

Ambiguity/missing files is an error; no guessing.

## Status / Trace

RunStatus workflow data must make GSD provenance visible:

- workflow = `gsd`
- stage
- planning source paths
- parsed execution plan
- step results
- review

Do not include secrets; planning text is not copied wholesale into status unless required by a step output.

## Auto-advance Boundary

Phase 17 has two plans:

- 17-01: planning loader + execution spec + verifier/reviewer vertical slice.
- 17-02: verified state transition / next-phase auto-advance.

17-01 must not silently modify STATE beyond operations explicitly present in its execution spec. 17-02 owns automatic STATE transition semantics after successful review.

## Exit Criteria for Phase 17

1. `agent run --workflow gsd` resolves the current GSD phase/plan from real `.planning` files.
2. A fixture PLAN execution spec can perform a real file edit through Runtime policy and run configured verifier.
3. forbidden operation or missing capability becomes Run error before unsafe execution.
4. missing/ambiguous STATE/CONTEXT/PLAN is a clear planning error.
5. review pass/fail remains distinct from orchestration error.
6. 17-02 validates successful plan completion updates STATE and can resolve the next planned phase without guessing.
7. full fmt/test/vet/build + real binary acceptance pass.

## Deferred

- natural-language-to-execution-spec model backend
- retry/fix loop driven by an LLM
- parallel subagents/worktrees — Phase 18
