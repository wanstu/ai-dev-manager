# Phase 18 Context — Parallel Agents / Worktrees

## Goal

完成 v0.4 的最后一层：一个 daemon-owned parent Agent Run 可以把多个 lane 放进真实 managed Git worktree 中并发执行，保证每个 lane 的 filesystem/runtime root 独立，再由 parent reviewer 聚合结果。

Phase 18 不伪装 LLM subagent。第一条 production workflow 明确命名为 `parallel-verify`：它在多个新建 managed worktree 上并发运行 Phase 16 已验证的 verify workflow。未来 coding subagent 可以复用同一 lane/worktree/derived-runtime contract。

## CLI

```powershell
ai-dev-manager agent run --workflow parallel-verify \
  --lane check-a:adm-check-a \
  --lane check-b:adm-check-b \
  [--keep-worktrees] \
  [path|workspace-id]
```

Rules:

- `--lane` 可重复，格式 `<name>:<branch>`。
- 至少 2 个 lane，最多 8 个。
- lane name / branch 必须唯一；最终仍由现有 managed worktree runtime validation authoritative。
- 默认 verification 完成后清理 managed worktree；`--keep-worktrees` 显式保留用于诊断。

## Architecture

### Parent Run

仍由 Phase 15 Agent Manager 持有，顶层 state/status/cancel 语义不变。

新增 optional `parallel` audit state：

- lane name
- branch
- managed worktree path
- lane state
- verify plan/steps/review/error
- cleanup state

Parent top-level `review`：

- 所有 lane verifier review pass → pass
- 任一 lane verifier review fail → fail
- worktree create / derived runtime / execution infrastructure error → parent Run error

### Managed Worktree Boundary

Worktree 必须通过 base Workspace Runtime 的 `git.worktree.create` 创建，禁止 Agent 自己拼任意文件系统路径。

创建成功后得到 runtime-owned Worktree path，再调用 Control Plane 的 derived-runtime builder：

```text
Base Workspace ID + resolved EffectiveConfig + managed worktree path
    -> derived Workspace identity (observed only)
    -> Native Runtime rooted at worktree
    -> Runtime Adapter
```

Derived Workspace 不写入 persistent Workspace Registry；它只属于 parent Run observed state。

### Derived Runtime Security

- EffectiveConfig / Policy / ToolPaths / Verifiers 与 base Workspace 一致。
- filesystem PathGuard root 改为 managed worktree path。
- runtime operations 仍受 Native policy/capability 强制。
- derived runtime path 只接受由 `git.worktree.create` 返回的 path，不提供用户任意 path API。

## Concurrency

1. sequentially create worktrees，避免 Git repository metadata 并发写冲突。
2. derived runtimes ready 后，verify lanes 并发执行。
3. parent 等待全部 lane 完成再 review。
4. cancellation 停止启动后续 lane，并让 parent 不被 late result 覆盖。
5. cleanup 在 lane execution 完成后进行；cleanup failure 记录 error，不静默忽略。

## Worktree Lifecycle

Default:

```text
create -> verify -> aggregate -> remove managed worktree
```

With `--keep-worktrees`:

```text
create -> verify -> aggregate -> preserved
```

Branches 不自动删除；Phase 18 不做 merge/branch delete。这样不会在验证 workflow 里隐式改变 Git history。

## Exit Criteria

1. parent Run 接收 2+ lane spec，并在不同 managed worktree roots 执行。
2. 两个 lane verifier 真正并发，parent status 可审计每个 lane。
3. lane A 的文件变化/状态不泄漏到 lane B 或 main checkout。
4. parent review 正确聚合 pass/fail；infrastructure error 与 review fail 分离。
5. default cleanup 删除 managed worktrees；`--keep-worktrees` 可显式保留。
6. main checkout HEAD/branch 不被切换。
7. process-level CLI acceptance + full fmt/test/vet/build pass。

## Deferred Beyond v0.4

- LLM-generated lane tasks / subagent prompts
- automatic merge/cherry-pick/conflict resolution
- branch deletion policy
- generic process manager / Docker / Debug (v0.5)
