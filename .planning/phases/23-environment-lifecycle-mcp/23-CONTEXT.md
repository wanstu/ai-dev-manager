# Phase 23 Context — Environment Lifecycle MCP

## Goal

让外部 Agent 通过 Phase 22 的同一个稳定 Gateway MCP 完成 Environment lifecycle：创建、销毁、writer acquire/release；完整复用 v0.5 Environment Manager guardrails，并把 domain failure 变成 Agent 可恢复的 structured tool error。

Phase 23 不实现 routed files/git/verify；这些进入 Phase 24/25。

## Agent-facing tools

### environment_create

Input:

- `workspace_id` required
- `name` required
- `base` optional
- `branch` optional
- `include_changes` optional, default false

语义完全复用 v0.5：

- default base = creating checkout current branch/HEAD
- dirty source 默认不带入并 warning
- `include_changes=true` 保留 staged/unstaged + untracked non-ignored regular files
- branch 默认 `adm/<sanitized-name>`
- branch exists => explicit conflict，不自动 suffix

Output成功时返回完整 `InspectResult`。

### environment_destroy

Input:

- `environment_id` required
- `force` optional, default false

语义完全复用 Manager.Destroy：

- normal拒绝 dirty/unpushed/active writer
- force是显式 destructive override
- branch不自动删除
- 不自动 commit/push/merge/rebase

### environment_writer_acquire

Input:

- `environment_id`
- `owner`

同 owner renew，第二 owner `writer_conflict`。

### environment_writer_release

Input:

- `environment_id`
- `owner`（normal release required）
- `force` optional

force只清 writer lease，不修改代码或 destroy Environment。

## Structured domain errors

Lifecycle tools不能退回到只有模糊字符串的错误体验。

统一 protocol-neutral error shape：

```text
code
message
environment_id?
facts?
warnings?
hints?
```

MCP tool failure should:

- set `isError=true`
- still provide structured output containing the stable error envelope when SDK supports typed output + explicit CallToolResult IsError
- never include Go stack/wrapped internal chain/secrets

Examples:

```text
code=branch_exists
message=branch "adm/task" already exists
```

```text
code=writer_conflict
facts.writer_owner=owner-a
```

```text
code=unsafe_destroy
facts from best-effort Environment inspect: dirty/upstream/writer/base state
```

Warnings/hints remain descriptive, not prescriptive; no `required_action`, `next_step`, `must_rebase`.

If a lifecycle operation fails but returns a valid Environment/InspectResult snapshot, preserve that snapshot in structured output where useful.

## Gateway core boundary

Extend protocol-neutral `internal/gateway.Service` to depend on the existing Environment Manager lifecycle contract, not on files/JSON directly.

Gateway must never:

- create raw worktrees itself
- read/write `environments.json` directly
- bypass Environment Manager destroy/writer policy
- delete branches

## Tool surface boundary

At end of Phase 23 Gateway tool list is exactly discovery + lifecycle:

- gateway_info
- workspace_list
- environment_list
- environment_inspect
- environment_create
- environment_destroy
- environment_writer_acquire
- environment_writer_release

No `git_worktree_*` tools.
No files/read/search/write/edit/exec yet.

## Activity semantics

- create naturally sets Environment activity through Manager.
- writer acquire/renew/release updates activity as v0.5 defined.
- list/inspect do not touch activity.
- failed writer conflict does not steal/renew another writer.

## Exit Criteria

1. official MCP client creates Environment from registered Workspace and gets stable env_id/branch/worktree facts.
2. create default dirty behavior and `include_changes=true` semantics remain covered through existing Manager tests plus MCP path acceptance.
3. branch conflict returns tool `isError=true` plus structured `branch_exists` code.
4. writer A acquire succeeds; writer B gets structured `writer_conflict`; owner A renew/release works.
5. normal destroy of active-writer/dirty/unpushed env returns structured `unsafe_destroy` with best-effort facts.
6. explicit force destroy succeeds and branch remains.
7. no raw worktree tools appear in Gateway tool list.
8. daemon restart keeps same Gateway endpoint and Environment/writer persistence remains visible via MCP.
9. Direct MCP regression remains green.
10. targeted + full fmt/test/vet/build pass, then auto-advance Phase 24.
