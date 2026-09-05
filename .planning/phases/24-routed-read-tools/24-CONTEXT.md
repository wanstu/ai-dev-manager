# Phase 24 Context — Routed Read Tools

## Goal

让外部 Agent 在同一个稳定 Gateway MCP 上，通过显式 `environment_id` 使用 Environment 内的只读开发能力：`tree/read/search/git_status/git_diff/git_branch`。每次调用必须重新验证该 Environment 对应的 managed worktree，再构建 derived Runtime；不得直接信任持久化 worktree path。

Phase 24 不实现 write/edit/exec/verify，也不要求 writer；这些留给 Phase 25。

## Agent-facing surface

新增 tools：

- `tree`
- `read`
- `search`
- `git_status`
- `git_diff`
- `git_branch`

每个输入都必须带 `environment_id`。文件类工具再带现有 Direct MCP 对应参数：

- tree: path/max_depth/max_entries
- read: path/max_bytes
- search: path/query/max_files/max_matches/max_bytes_per_file

Git read tools只需要 `environment_id`。

## Environment routing boundary

新增 Environment Manager 的 protocol-neutral read invoke boundary，而不是让 Gateway adapter自行拼 worktree path：

1. 从 Store 获取 Environment。
2. 通过 `validatedRuntimes`：
   - base Workspace 必须仍存在；
   - base Runtime 查询 managed worktree inventory；
   - persisted path 与 managed inventory 必须一致；
   - 用实际 managed path 构建 derived Runtime。
3. 只允许 Phase 24 的 read operation allowlist。
4. 在具体 derived Runtime 上检查对应 capability。
5. Invoke Runtime Adapter并返回 structured result。

Gateway 不能读取 `environments.json`、不能直接使用 persisted `WorktreePath`、不能创建 fake Runtime。

## Read-only semantics

- 不需要 writer lease。
- 不 acquire/renew writer。
- 不 touch `LastActivityAt`。
- 不改变 Environment state，除非 managed worktree validation 本身发现真实缺失/不一致；这种错误由 existing Environment domain semantics 报告。
- 不自动 commit/rebase/merge/push。

## Capability semantics

Gateway tool surface 是统一的：Agent 不需要因为不同 Environment capability 每次重新配置 MCP。

具体调用时：

- Environment runtime有能力 => 执行。
- 缺能力 => structured domain error `capability_missing`，附 environment_id 和需要的 capability。

`environment_inspect` 继续是预先发现 capabilities 的主要入口；调用时仍必须二次校验，不能只信旧 inspect。

## Error semantics

保持 Phase 23 `isError=true + structuredContent.error`。

常见 codes：

- `environment_not_found`
- `worktree_missing`
- `worktree_mismatch`
- `capability_missing`
- `invalid_input`
- `runtime_error`

Runtime/path安全错误不泄露 stack/secret；已有 RuntimeError/GitError可转成稳定 message，但不把内部 wrapped chain直接输出。

## Concurrency

Phase 24 read调用要允许不同 Environment 并行，不应为了简单安全把整个 Environment Manager 用全局独占锁包住长时间 tree/search/read。

Destroy 与 read 的极端竞态可以表现为一次正常的 runtime/worktree unavailable tool error；Phase 25 才需要为 writer-safe mutation提供更强的 writer/invoke/destroy协调保证。

## Exit Criteria

1. Gateway tool list包含 Phase 23 8 个工具 + 6 个 routed read tools，仍无 raw worktree/mutation tools。
2. 两个 Environment来自同一 Project时，`read/search/git_*` 各自只看到自己的 worktree状态。
3. main checkout 的 dirty变动不被默认 Environment read看到；include_changes Environment则看到转移后的变动。
4. 每次 routed call都通过 Manager managed-worktree revalidation；tampered/missing persisted worktree返回 structured error。
5. read tools不要求 writer，即使另一个 owner持有 writer也可读取。
6. read tools不刷新 LastActivityAt。
7. capability missing产生 structured `capability_missing`，不从 Gateway tool list动态删除工具。
8. daemon restart后同一 Gateway URL仍可访问 routed read tools。
9. Direct MCP regression保持全绿。
10. targeted + full fmt/test/vet/build通过，然后 auto-advance Phase 25。
