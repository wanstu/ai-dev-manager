# Phase 20 Context — Dirty Change Transfer & Safe Destruction

## Goal

让 Environment 能覆盖两个已经出现的真实开发场景：

1. 当前 main/dev checkout 已有 staged / unstaged / untracked 工作时，显式创建 Environment 并把这些变动完整带入，同时不复制 ignored 文件。
2. 销毁 Environment 时，默认保护 dirty / 未推送工作；只有用户或 Agent 显式选择 force 才允许潜在数据丢失。

Phase 20 不做 stale、single-writer attachment、base divergence/rebase hint，也不做 MCP Gateway。

## Locked Semantics

### include_changes

默认 `env create` 行为保持 Phase 19 不变：

- Environment 从 committed base/HEAD 建立。
- base checkout 的 dirty changes 不带入。
- 返回 `changes_not_included` warning。

只有显式：

```text
--include-changes
```

才复制创建瞬间的：

- staged tracked changes
- unstaged tracked changes
- untracked files

明确不复制 Git ignored 文件。仓库中 tracked `.gitignore` 本身是 committed/tracked 内容，正常存在于 worktree。

### Preserve staged / unstaged distinction

不能把 dirty checkout 简化成“最终文件内容复制过去”，因为真实场景可能存在 partially-staged file。

因此 transfer 必须保留 index / working-tree 语义：

1. 导出 staged patch。
2. 导出 unstaged patch。
3. 创建目标 Environment worktree。
4. staged patch 以 index-aware 方式应用到目标。
5. unstaged patch 只应用到目标 working tree。
6. untracked files 单独通过 Runtime-safe file write 写入。

这保证 Agent 在新 Environment 中看到的 `git status` 尽量与源 checkout 保持同样语义。

### ignored files

ignored 文件默认不复制，包括常见：

- `node_modules/`
- `vendor/`
- build/cache/log/runtime files
- `.env` / local-only secrets（如果项目选择 ignore）

未来若真实需求证明需要复制某些 ignored path，应设计显式 allowlist，而不是加入 `copy_all_ignored`。

## Runtime Boundary

change transfer 仍必须走 Native Runtime / Runtime Adapter 的结构化能力。

不允许 Environment Manager：

- 直接使用 `os/exec git ...`
- 信任任意文件系统 path
- 绕开 PathGuard / policy / ToolPaths

建议增加内部结构化 Git operations：

- export tracked change set
- apply tracked change set
- enumerate/copy untracked through existing safe file boundary

`git apply` 如需要 stdin，应扩展内部 `runtime.Command` 的 stdin 能力，但不因此自动扩大 MCP `shell.exec` surface。

## Atomicity / Failure Semantics

`include_changes` 是一个多步骤 materialization，不可能与 Git worktree create 做真正跨文件系统事务。

因此：

- Environment record 仍先进入 `creating`。
- worktree 创建成功但 patch/untracked apply 失败时，Environment 进入 `error` 并保留实际 worktree 供诊断。
- 不自动删除 branch/worktree 试图“回滚”，避免在未知部分成功状态下丢工作。
- Inspect 必须仍可报告该 Environment 的真实状态。

## Safe Destroy

### Normal destroy

普通 `env destroy <env-id>` 必须拒绝：

- working tree / index dirty
- Environment 存在尚未安全存在于 upstream 的提交

Phase 19 的 `HEAD == base_commit` 只是保守临时规则；Phase 20 要区分“有新提交但已推送”和“有未推送提交”。

### Push safety facts

Environment Runtime 应能结构化报告：

- current HEAD
- 是否配置 upstream
- upstream ref（如有）
- local HEAD 相对 upstream 是否有 ahead commits

规则：

- 没有新 commit（HEAD == base commit）→ commit 维度安全。
- 有新 commit且没有 upstream → 视为存在未推送工作。
- 有 upstream 且 local HEAD ahead > 0 → 视为未推送。
- 有 upstream 且 local HEAD ahead == 0 → 允许普通 destroy（前提 working tree/index clean）。

不在 Phase 20 自动 push，也不建议 Agent 必须 push。

### Force destroy

显式：

```text
env destroy --force <env-id>
```

允许丢弃 dirty/unpushed Environment worktree，但：

- 必须仍只操作 managed worktree。
- 不删除 branch。
- 若 worktree 已被外部删除，force 可清理 stale Environment registry record，但不能删除或操作任何 Registry 中任意记录的外部 path。
- force 是显式 destructive policy override，不是 Agent 自行推断动作。

## Agent-facing feedback

ADM 可以返回：

```text
facts:
- dirty=true
- unpushed_commits=2
warnings:
- environment contains work that may be lost by force destroy
```

但不返回：

```text
next_step=push
required_action=commit
```

是否 commit/push/review 由 Agent/用户决定。

## CLI

Phase 20 extends:

```powershell
ai-dev-manager env create --name task --include-changes [path|workspace-id]
ai-dev-manager env destroy [--force] <env-id>
```

现有 Phase 19 CLI 保持兼容。

## Exit Criteria

1. staged-only、unstaged-only、partially-staged tracked change 在 `--include-changes` Environment 中保持 index/working-tree 区别。
2. untracked regular files 被复制，ignored files 不被复制。
3. 默认 create 仍不带 dirty changes，且 warning 行为不回归。
4. binary tracked patch / normal rename-delete change 至少通过真实 Git change-set acceptance，不因文本-only实现损坏。
5. change transfer 失败留下 error Environment/worktree 供诊断，不假装创建成功或静默删除。
6. normal destroy 拒绝 dirty Environment。
7. normal destroy 拒绝有未推送 commit 的 Environment；没有 upstream 的新 commit 也按未推送处理。
8. 已推送且 clean 的 Environment 可普通 destroy，branch 保留。
9. `--force` 可销毁 dirty/unpushed managed Environment，并保留 branch；missing worktree 时仅清理明确 Environment record，不触碰任意 path。
10. real Git + cross-process CLI acceptance + full fmt/test/vet/build pass。
