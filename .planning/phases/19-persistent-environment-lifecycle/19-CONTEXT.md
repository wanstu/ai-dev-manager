# Phase 19 Context — Persistent Environment Lifecycle

## Goal

把 v0.4 Phase 18 已验证的临时 managed worktree + derived Runtime 提升为持久的一等 Environment，使同一 Project 能长期并行承载多个互不污染的开发任务。

Phase 19 只建立 Environment Core。它不做 Agent MCP Gateway、不做 Process/Docker/Debug、不做 UI，也不内置新的 AI Agent。

## Product Boundary

ADM 负责：

- Environment 的事实、identity、生命周期和持久状态。
- managed worktree / branch / Runtime 的安全创建与校验。
- Runtime policy、PathGuard、structured Git/exec 等既有安全边界。
- 返回事实、warnings 和轻量 hints。

Agent / 用户负责：

- 任务意图和开发判断。
- 是否 rebase/merge/commit、commit message、最终 review/合并。
- 是否根据 hint 询问用户并采取下一步。

ADM 不返回“必须 rebase”“下一步 merge”这类开发指令；只有安全策略拒绝可以是强制性的。

## Environment Model

Environment 是持久对象，不等同于 Agent Run，也不等同于 Workspace Registry entry。

最小字段：

- `environment_id`: `env_` + crypto/rand identity。
- `workspace_id`: base registered Workspace / Project。
- `name`: 人/Agent 可读 display name。
- `branch`: Environment 独立 Git branch。
- `base_ref`: 创建时选择的 branch/tag/commit 表达。
- `base_commit`: 创建时解析出的 immutable commit。
- `worktree_name`: managed worktree 内部安全名称，优先直接使用 Environment ID。
- `worktree_path`: 持久记录用于诊断；每次构建 Runtime 前仍必须重新验证它确实是 base Runtime 当前 managed worktree。
- `state`: creating / ready / error（Phase 19 最小状态）。
- `created_at` / `updated_at` / `last_activity_at`。
- optional structured error。

Environment Registry 是持久用户状态，但 Runtime object、listener、goroutine 等 observed object 永不序列化。daemon restart 后通过 Registry + 实际 Git worktree 状态重新构建 derived Runtime context。

## Creation Semantics

### Base

默认：创建时当前 checkout HEAD。

- 若当前 checkout 在 `dev`，Environment 从该 HEAD 创建。
- 若紧急修复当前 checkout 在 `master`，默认自然从 master HEAD 创建。
- 不把 dev/master 写死在产品逻辑。
- `--base <branch|tag|commit>` 可显式覆盖。

必须同时记录 `base_ref` 与实际解析出的 `base_commit`，后续才能准确报告 base divergence。

### Branch

默认 branch：

```text
adm/<sanitized-name>
```

允许显式 `--branch`。

- branch 已存在：返回明确 `branch_exists`，不偷偷改成 `-2`。
- display name 冲突可以生成唯一 suffix，但机器身份始终依赖 `env_` ID。
- branch 允许 Git 合法的 `/`，不能继续直接复用 Phase 18 只允许字母数字/`.`/`_`/`-` 的 lane identifier 检查。

### Main checkout dirty

Phase 19 默认仍从 HEAD 创建，并在结果中报告 base checkout dirty / changes_not_included。

`include_changes=true` 属于 Phase 20：届时复制 staged + unstaged + untracked，并排除 ignored 文件。tracked `.gitignore` 是普通仓库内容，会自然存在于 worktree。

## Existing Foundation To Reuse

Phase 18 已有：

- Native `GitWorktreeCreate/List/Remove`。
- managed root：`<workspace-parent>/.ai-dev-manager-worktrees/<workspace-id>/`。
- `ControlPlane.BuildDerivedRuntime(workspaceID, derivedID, path)`。
- derived Runtime 复用 base EffectiveConfig，但 root 指向 worktree 且不写 Workspace Registry。
- real Git acceptance 已证明不同 worktree roots 与 main checkout HEAD/branch 隔离。

Phase 19 不重做这些能力，而是补持久 identity、registry、reconciliation 和产品 lifecycle。

## Required Runtime Evolution

当前 `GitWorktreeCreate(name, branch)` 固定从 `HEAD` 创建，而且 branch validator 不允许 `/`。Phase 19 需要最小扩展：

- 能把可选 base ref 安全解析为 immutable commit。
- worktree create 接受 resolved start commit，而不是让任意 raw path/command 绕过 Runtime。
- branch 使用 Git branch/ref 合法性检查并拒绝参数注入型输入。
- Runtime Adapter 的 `git.worktree.create` 保持兼容：新增 optional start point/base commit，而旧调用仍默认 HEAD。

所有 Git 操作继续走 Native structured execution / ToolPaths / policy，不直接绕到 `os/exec`。

## Persistence / Recovery

Environment Store 使用独立 schema/version 与 atomic replace，不能塞进 Workspace Registry 或 desired-runtimes schema。

建议位置：config root 下独立 Environment state file，由 `internal/environment` package 持有；daemon 只组合 Manager，不让 domain store 依赖 HTTP/CLI。

Create transaction：

1. 生成 `env_` ID、resolve Workspace、base ref/base commit、branch。
2. 持久化 `creating` record，避免 side effect 后完全失去追踪。
3. 通过 base Runtime 创建 managed worktree。
4. 重新从 `git worktree list` 验证返回 path 属于 managed worktree。
5. 构建 derived Runtime 进行可用性验证。
6. record 更新为 ready。
7. 任一步失败记录 `error`，不伪装 ready；不自动删除未知用户工作。

Daemon restart：

- load Environment Registry。
- 不恢复旧 Runtime object。
- list/inspect/runtime access 时验证 base Workspace + actual managed worktree。
- Registry path 不能单独作为可信任的 derived Runtime root；必须与 base Runtime 当前 managed worktree inventory 匹配。
- worktree 被外部手工删除/移动时，Environment 应进入 error/missing fact，而不是静默从 Registry 消失。

## CLI / Control API

Phase 19 最小 CLI：

```powershell
ai-dev-manager env create --name coupon-share [--base master] [--branch hotfix/coupon] [path|workspace-id]
ai-dev-manager env list
ai-dev-manager env inspect <env-id>
ai-dev-manager env destroy <env-id>
```

规则：

- `env create` 沿用 `up` 的 path/workspace-id resolution；未知 path 可按现有日常 UX 注册 base Workspace。
- CLI 是 loopback daemon Control API 的薄客户端。
- Phase 19 的 destroy 采用保守语义：只允许确认没有 dirty working tree 且没有相对 base 的新 commit 的 Environment；Phase 20 再补准确的 unpushed/force contract。
- destroy 删除 managed worktree 和 Environment Registry record，但不自动删除 branch。

## Out of Scope — Phase 19

- `include_changes=true` materialization（Phase 20）。
- force destroy / unpushed detection（Phase 20）。
- stale policy / writer attachment / base divergence hints（Phase 21）。
- Agent MCP Gateway / `environment_id` tool routing（v0.6）。
- per-Environment hand-written MCP configuration；长期方向恰恰是避免它。
- automatic merge/rebase/cherry-pick/commit。
- generic Process Manager / Docker / Debug。
- CodexProPlus UI/Manager integration。

## Exit Criteria

1. 同一 registered Workspace 可创建至少两个持久 Environment，各自有不同 `env_` ID、branch、managed worktree path。
2. 默认 base 使用创建时 current checkout HEAD；显式 `--base` 能从指定 ref 的 resolved commit 创建。
3. 默认 `adm/<name>` branch 可正常工作；已有 branch 返回明确冲突，不生成替代 branch。
4. main checkout HEAD/branch/dirty files 不因普通 Environment create 被修改或带入。
5. daemon stop/start 后 Environment list/inspect 仍存在，并能基于实际 managed worktree 安全重建 derived Runtime context。
6. 手工移除 worktree 后 Registry 不静默丢失，inspect 报告结构化异常事实。
7. conservative destroy 不删除 dirty/有新 commit 的 Environment，不删除 branch。
8. 两个 Environment 的 Runtime root 隔离；A 的 edit/status/diff 不泄漏到 B 或 main checkout。
9. process-level CLI acceptance + full fmt/test/vet/build pass。
