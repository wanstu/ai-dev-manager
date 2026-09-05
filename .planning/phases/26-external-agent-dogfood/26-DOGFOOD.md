# Phase 26 Dogfood Evidence

Date: 2026-09-05
External Agent path: ChatGPT -> MCPHub -> `pj-adm` -> ADM Gateway -> Environment
Environment: `env_c288406b777466cc18139f25131a4480`

## Proven in a real external session

- Gateway discovery returned the expected 19-tool Agent surface.
- `workspace_list` returned the registered `ai-dev-manager` Workspace.
- Docker reachability works through `host.docker.internal` on the persisted Gateway port.
- MCPHub can parse all ADM tool schemas after replacing boolean `true` result schemas with permissive object-form `{}` schemas.
- Human-authorized `workspace prepare` enables the minimum `standard + git` policy required for managed Environment creation while keeping default registration read-only.
- `environment_create(include_changes=true)` created an isolated managed worktree and transferred current checkout changes.
- A persistent single-writer lease was acquired by `chatgpt-phase26-dogfood`.
- Routed Git status observed changes inside the Environment without using the raw worktree path.

## Completion proof

- `read` loaded the Phase 26 plan and dogfood documentation through the Environment route.
- `search` confirmed the missing Workspace preparation guidance before it was added.
- `write` created this evidence file inside the Environment.
- `edit` updated `docs/external-agent-dogfood.md` with the human-authorized `workspace prepare` flow.
- structured `exec` ran `git diff --check` under the Workspace policy and exited 0.
- routed `git_diff` and `git_status` captured the final review state.
- no raw worktree path was supplied by the Agent and no merge, commit, rebase, or Environment destruction was performed.

The first real external-Agent task is complete. The Environment and branch remain available for human review.
