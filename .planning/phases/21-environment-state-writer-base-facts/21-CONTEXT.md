# Phase 21 Context — Environment State / Writer Guard / Base Facts

## Goal

补齐长期并行开发真正需要的 Environment 可观测性与协作边界：

1. `inspect` 能回答 Environment 当前是否 dirty、base 是否前进/分叉、当前 writer 是谁、多久没有开发活动。
2. 一个 Environment 默认只允许一个显式 writer owner；第二个 writer 不能静默抢占。
3. stale/长期不活跃只作为事实和轻量提示，永不自动删除、自动释放 writer、自动 rebase/merge。

Phase 21 是 v0.5 最后一阶段。它不实现 Agent MCP Gateway；Gateway 将在 v0.6 使用这里建立的 Environment identity / writer / facts contract。

## Base relation semantics

Environment 创建时已经持久化：

- `base_ref`
- `base_commit`
- Environment branch

Inspect 时重新解析当前 `base_ref`，并比较 Environment branch 与当前 base：

- `ahead`: Environment branch 中、当前 base 没有的 commit 数。
- `behind`: 当前 base 中、Environment branch 没有的 commit 数。
- `diverged`: `ahead > 0 && behind > 0`。
- `base_moved`: 当前 resolved base commit 与创建时 `base_commit` 不同。

如果 `base_ref` 是 immutable commit/tag 且没有变化，上述事实自然保持稳定；不把 dev/master 写死。

Base ref 被删除或无法解析时，inspect 仍返回 Environment 本体和结构化 warning/fact，不自动改 base。

## Dirty / push facts

Inspect 同时返回：

- `dirty`
- `dirty_change_count`
- current HEAD
- upstream presence/ref
- upstream ahead count

这些只用于描述 Environment，不自动 commit/push。

## Hints, not instructions

ADM 可以在明显需要关注时返回轻量 hint：

- Environment 已发生 base divergence。
- Environment 明显落后当前 base（默认阈值 10 commits）。

文案必须类似：

> Base has advanced/diverged; consider confirming whether synchronization is needed before continuing.

不得返回：

- `required_action=rebase`
- `next_step=merge`
- “必须先同步”

是否 rebase/merge/继续开发由 Agent/用户决定。

## Activity / stale

`last_activity_at` 表示 Environment 开发协作活动，不应被普通 `env inspect` 刷新，否则持续监控会掩盖真实 inactivity。

Phase 21 的活动更新点：

- create/materialize 完成
- writer acquire / same-owner renew
- writer release
- future v0.6 routed mutation can复用同一 touch boundary

Phase 21 使用保守默认：

- 7 天无活动 => `stale=true` fact / warning candidate
- 返回 `inactive_for_seconds` 与 `stale_after_seconds`
- stale 不改变 lifecycle `state`
- stale 不自动 destroy
- stale 不自动 release writer

未来如果真实使用证明需要不同阈值，再升级为配置；Phase 21 不为此扩大配置系统。

## Single-writer lease

Environment 增加持久 writer lease：

- `owner`: 外部 caller 自己提供的稳定 owner/session identifier
- `acquired_at`
- `last_seen_at`

操作：

```text
env writer acquire --owner <owner> <env-id>
env writer release --owner <owner> <env-id>
env writer release --force <env-id>
```

规则：

- 无 writer -> acquire 成功。
- 同 owner 再 acquire -> idempotent renew，刷新 last_seen/activity。
- 不同 owner acquire -> `writer_conflict`，不得抢占。
- 普通 release 只允许当前 owner。
- `--force` release 是显式管理动作，只清 writer lease，不删除 Environment 或代码。
- writer lease 持久化并跨 daemon restart；不能因为 daemon 重启假装 owner 消失。
- 不因 stale 自动释放 writer，因为用户明确提到可能暂停开发后再回来。

`owner` 是协调标识，不是身份认证；真正远程 MCP auth 属于 v0.6 exposure/auth 设计。

## Runtime boundary

Base relation继续通过结构化 Runtime Git operation 完成，不允许 Environment Manager 直接 `os/exec git`。

新增 protocol-neutral Git relation operation，例如：

- `git.relation {left, right}`

它必须：

- 拒绝 option-like/unsafe ref input
- 使用已有 ToolPaths/policy execution
- 返回 structured ahead/behind

## CLI / Control API

Phase 21 extends existing `env` CLI：

```powershell
ai-dev-manager env inspect <env-id>
ai-dev-manager env writer acquire --owner codex-task-1 <env-id>
ai-dev-manager env writer release --owner codex-task-1 <env-id>
ai-dev-manager env writer release --force <env-id>
```

JSON inspect 返回 facts / warnings / hints；不得增加 prescriptive `required_action` / `next_step`。

## Exit Criteria

1. Environment branch相对当前 base 的 ahead/behind/diverged/base_moved 在真实 Git 场景正确。
2. inspect 报告 dirty、push/upstream、worktree、base relation 等事实，不修改 Git 历史。
3. base ref 删除/不可解析时 inspect 不崩溃、不改 base，返回结构化 warning。
4. divergence 或显著 behind 最多产生一条非指令性同步 hint；普通轻微 behind 只有 fact。
5. inspect 不刷新 `last_activity_at`；7 天 inactivity 只报告 stale，不自动删除/释放。
6. writer acquire/release 跨进程 daemon restart 持久化。
7. 第二 owner不能抢占；同 owner renew idempotent；显式 force release 可清 lease。
8. writer stale 不自动释放。
9. Process-level CLI acceptance + full fmt/test/vet/build pass。
10. v0.5 在 Phase 21 验证后停止于 milestone boundary；不自动进入 v0.6 MCP Gateway 实现。
