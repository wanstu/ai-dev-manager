# Phase 25 Context — Writer-safe Mutation & Verification

## Goal

完成 v0.6 Agent MCP Gateway：在同一个稳定 Gateway MCP 上，让外部 Agent 通过 `environment_id + writer_owner` 安全使用 `write/edit/exec/run_verifier/run_verifiers`。ADM 必须在真正 Runtime Invoke 前强制 single-writer，并保证 writer 校验、invoke、writer renew/activity touch 与 destroy/release 之间没有竞态穿透。

## Agent-facing tools

新增：

- `write`
- `edit`
- `exec`
- `run_verifier`
- `run_verifiers`

所有输入必须包含：

- `environment_id`
- `writer_owner`

其余参数保持现有 Direct MCP 语义。

## Writer semantics

- Agent 必须先通过 `environment_writer_acquire` 获得 writer lease。
- mutation tool调用时 ADM重新从 Store读取当前 writer，不信任调用方声明。
- 没 writer => `writer_not_owner`。
- writer owner不同 => `writer_not_owner`，并可在 facts中描述当前 owner；不得自动 steal/force acquire。
- 同 owner才允许继续。
- Runtime Invoke成功后：
  - 保持 AcquiredAt；
  - 更新 Writer.LastSeenAt；
  - 更新 UpdatedAt / LastActivityAt；
  - 持久化。
- Runtime Invoke失败不自动释放 writer，也不伪造成功 activity。

## Concurrency contract

必须防止：

1. writer校验通过；
2. 另一个调用 release/destroy；
3. worktree被删；
4. 原 mutation继续写旧路径。

Phase 25使用现有 Manager coordination lock形成原子边界：

- Manager `mu` 升级为 `sync.RWMutex`。
- mutation operation持有 `RLock` 贯穿 writer check -> `validatedRuntimes` -> Invoke -> success renew/touch/store。
- writer acquire/release 和 destroy继续持有 `Lock`。
- 多个 mutation可并行，因此不同 Environment不会因全局 mutex被串行化。
- 同 owner在同一 Environment并发发出多个 mutation仍属于调用方并发行为；ADM保证 lease/destroy安全，不在 v0.6实现复杂 per-Environment transaction scheduler。

Read-only Phase 24 tools不需要这把 coordination RLock；read/destroy极端竞态仍可表现为正常 worktree unavailable read error。

## Mutation routing

Environment Manager增加严格 `InvokeMutation` allowlist：

- `files.write` -> `files.write`
- `files.edit` -> `files.edit`
- `shell.exec` -> `shell.exec`
- `verify.run` -> `verify.run`
- `verify.run_many` -> `verify.run`

每次仍调用 `validatedRuntimes`，不使用 persisted path直接构建。

具体 derived Runtime缺 capability => `capability_missing`。

## Exec safety

`exec` 不提供 shell string；继续使用现有 structured：

- executable
- args[]
- cwd
- timeout_ms
- max_output_bytes

所有 executable allowlist/path containment/timeout/output限制仍由 Runtime执行层强制。

## Verify semantics

Verifier属于 active operation，因此需要 writer_owner，即使 verifier本身通常只读。原因：测试/build可能生成文件、缓存或运行项目命令；将其放入 writer lease边界更符合 Environment active-use模型。

Verifier fail是成功调用返回的 structured verifier result，不应自动变成 Gateway domain error；只有 Runtime/operation failure才是 tool error。

## Error semantics

继续使用 Phase 23 structured domain error：

- `writer_not_owner`
- `environment_not_found`
- `worktree_missing`
- `capability_missing`
- `runtime_error`
- `invalid_input`

不得返回 required_action/next_step；可以描述事实和轻量提示。

## Exit Criteria

1. Gateway最终 tool surface包含 discovery/lifecycle/read + 5 mutation/verify tools，不暴露 raw worktree ops。
2. 无 writer或错误 writer_owner调用 write/edit/exec/verify均被拒绝，Runtime未执行。
3. 正确 owner调用成功，writer LastSeenAt / LastActivityAt更新且 AcquiredAt保持。
4. mutation Runtime失败不释放/偷换 writer。
5. mutation执行期间并发 release/destroy必须等待，不能删除 worktree或清 writer直到 invoke结束。
6. 两个 Environment、两个 writer可以并行 mutation，修改互不污染。
7. daemon restart后 writer持久化，同一 Gateway URL继续可写对应 Environment。
8. exec继续受 Runtime structured policy约束。
9. verify返回真实 verifier结果；fail不自动变成 infrastructure error。
10. Direct MCP regression + targeted + race + full fmt/test/vet/build全绿。
11. Phase 25完成后 v0.6 milestone complete，停止在 milestone boundary，不自动进入 v0.7。
