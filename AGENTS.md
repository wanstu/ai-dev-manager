# ai-dev-manager-v2 Development Rules

These rules exist to prevent implementation details from silently becoming product requirements.

## 0. Mandatory planning workflow

Before changing feature code:

1. Read `.planning/STATE.md`.
2. Read `.planning/PROJECT.md` for locked requirements, reality status and frozen work.
3. Read the current Phase `CONTEXT.md`.
4. Read the current executable `PLAN.md`.
5. Implement only that Plan unless a blocking contradiction invalidates the Phase.

A useful idea outside the current Phase goes to `.planning/STATE.md` backlog instead of being implemented immediately. Important decisions and Phase transitions must be written into `.planning/`; chat history is not the project plan.

Until Phase 01 proves the real GSD Skill can be consumed through ADM, this planning workflow is a manual bootstrap and must not be described as V2 Skill/GSD runtime completion.

## 1. Product contract wins

Before implementing a behavior, map it to an explicit requirement in `docs/PRODUCT_CONTRACT.md`.

If a behavior is not required, do not make it a prerequisite for unrelated functionality.

## 2. Capabilities are local, not global prerequisites

Git, worktree, Docker, shell, verifier, MCP, language toolchains, and similar integrations are optional capabilities.

A missing capability may block only the operation that needs that capability.

Examples:

- no Git -> `git.status` is unavailable; file read/write still works
- no verifier -> verifier execution is unavailable; Environment creation still works
- no shell -> exec is unavailable; read/search still works
- no worktree -> worktree isolation is unavailable; normal development still works

Never add a global prerequisite such as "Environment requires Git" without an explicit product requirement approved by the user.

## 3. Environment is not an isolation technology

Workspace = registered development directory.

Environment = development context rooted at a directory.

An Environment must not intrinsically depend on Git, branches, commits, worktrees, Docker, or verifier configuration.

Isolation mechanisms may later produce another root directory, but they remain optional tools around the Environment model.

## 4. No compatibility burden during active development

Until the user explicitly declares a stable compatibility boundary, do not add migrations, legacy field inference, fallback decoding, or compatibility shims for old development data.

When a model is wrong, change it directly and update tests/data.

## 5. Requirements must include negative acceptance tests

For every important capability, test both what must work and what must NOT be required.

Examples:

- a plain non-Git directory can be registered and developed
- Environment creation works without Git
- file operations work without verifier configuration
- Git operations fail locally and clearly when Git capability is absent

## 6. Keep phases small and requirement-traceable

A phase is only a delivery grouping. It must not invent product semantics.

Every phase must list:

- requirement IDs implemented
- explicit non-goals
- acceptance tests
- new prerequisites, if any

Any new prerequisite requires explicit user approval before implementation.

## 7. Dogfood before abstraction

Before adding a new abstraction, use the current ADM build to perform real development work on another project.

If daily development reveals a blocker, fix that blocker first.

Do not build speculative lifecycle, orchestration, compatibility, or isolation features merely because they may be useful later.

## 8. Prefer deletion over layering when an abstraction is wrong

Do not preserve an incorrect abstraction by adding modes, compatibility branches, or translation layers unless compatibility has explicitly become a requirement.
