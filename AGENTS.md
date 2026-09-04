# AGENTS.md — ai-dev-manager

## Project Mission

Build an independent local AI development workspace/runtime core that combines the strongest ideas from CodexPro and DevSpace without forking or coupling to either project.

## Mandatory Planning Workflow

Before changing code:

1. Read `.planning/STATE.md`.
2. Read `.planning/PROJECT.md` for locked requirements and out-of-scope items.
3. Read the current Phase `CONTEXT.md`.
4. Read the current executable `PLAN.md`.
5. Implement only the current plan unless a blocking contradiction is discovered.

If a new idea is useful but outside the current Phase, record it in planning instead of implementing it immediately.

## GSD Rules

- Small vertical slices over broad framework scaffolding.
- Every Phase has explicit exit criteria.
- No phase is complete without real verification.
- Important design decisions must be written into `.planning/`, not left only in chat history.
- Update STATE after each completed plan/phase.
- Requirements move to Validated only after verified behavior exists.
- When `.planning/config.json` has `workflow.auto_advance=true`, a verified Phase transition immediately continues into the next planned Phase in the same active work session; stop only at milestone completion, a real blocker, or a safety/requirement boundary that needs user input.
- Do not silently change locked decisions; update Context/Project first.

## Architecture Rules

- Control Plane and Runtime are separate boundaries.
- Runtime consumes EffectiveConfig; it does not resolve config inheritance.
- Config precedence: Runtime Override > Project > Profile > Global.
- Shared Global MCP/Skills are inherited; projects only store differences.
- Project-level disable/override is first-class.
- Runtime capabilities are explicit and queryable.
- Security policy must be enforced in execution code, not only described to an AI model.
- Prefer adapters over forks for CodexPro, DevSpace, Codex CLI, Claude Code, OpenCode, etc.
- Do not create speculative packages/interfaces with no current test or vertical-slice use.

## Repository Boundaries

- Do not modify `D:\projects\codexprov4` from this project.
- Do not copy implementation code from CodexPro or DevSpace without first documenting license/provenance and why direct reuse is preferable to reimplementation.
- Planning artifacts belong under `.planning/`.
- Global GSD/Skill installation is external to this repository; `.planning/` is project state, not the global Skill itself.

## Verification

For Go phases, the default gate is:

```text
go test ./...
```

Run additional phase-specific verifiers when defined in the current plan. If the toolchain is unavailable, report verification as blocked; never claim success from static inspection alone.
