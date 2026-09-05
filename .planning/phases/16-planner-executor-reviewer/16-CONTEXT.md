# Phase 16 Context — Planner / Executor / Reviewer

## Goal

在 Phase 15 已验证的 daemon-owned Agent Run lifecycle 上，接入第一条真实、可审计、非伪装的 Planner → Executor → Reviewer workflow。Phase 16 不绑定外部 LLM CLI；第一条 production workflow 使用现有 Native Runtime / Runtime Adapter 的结构化 Git + Verifier 能力，验证编排边界本身。

## Product Behavior

现有命令保持兼容：

```powershell
ai-dev-manager agent run [path|workspace-id]
```

仍使用 `lifecycle` executor。

Phase 16 新增显式 workflow：

```powershell
ai-dev-manager agent run --workflow verify [path|workspace-id]
```

它运行一个 deterministic verify workflow：

1. Planner 根据 Runtime capabilities 生成可执行 Plan。
2. Executor 通过 protocol-neutral Runtime Adapter 执行结构化 operations。
3. Reviewer 读取结构化结果，给出 pass / fail decision 和 summary。
4. Run status 保留 plan、step results、review，供 `agent status/list --json` 审计。

## Why `verify` First

- 它是真实生产能力：读取 Git 状态/diff并执行项目 configured verifiers。
- 它复用 Phase 5 已验证的 Git/Verifier runtime 能力和 Phase 6 Runtime Adapter，而不是新造 shell/process path。
- 它可以明确验证 Planner / Executor / Reviewer 的职责和数据契约。
- 它不声称已经具备自然语言代码规划能力；LLM/GSD planner 在 Phase 17 接入同一 orchestration contract。

## Workflow Contracts

### Planner

输入 RunSpec + Runtime capabilities，输出 `Plan`：

- stable step id
- operation
- input
- human-readable purpose

Verify planner 只选择 Runtime 明确支持的 operations：

- `git.status`
- `git.diff`
- `verify.run_many`

若 verifier capability 不可用，planner 返回明确错误；不绕过 capability 直接执行。

### Executor

按 Plan 顺序调用 `runtimeadapter.Runtime.Invoke`：

- 每步记录 started/finished/error/output。
- context cancel 必须立即停止后续步骤。
- 不允许任意 operation；只能执行 Plan 中已生成的结构化 operation。

### Reviewer

Reviewer 不重新执行命令。

Verify reviewer：

- runtime invocation error → fail
- verifier result 中任一 `failed` → fail
- passed/skipped 且无 execution error → pass
- 记录 decision + summary

## Run Status Evolution

Phase 15 字段保持兼容，新增可选 workflow fields：

- `workflow`
- `stage` (`planning|executing|reviewing|completed`)
- `plan`
- `steps`
- `review`

只有 workflow executor 使用这些字段；`lifecycle` Run 不强行填充。

Run top-level lifecycle state 仍只有：

`running -> completed | cancelled | error`

Reviewer 的业务 `fail` 不等于 orchestration crash：workflow 可正常完成但 `review.decision=fail`。这使“验证未通过”和“Agent 系统异常”保持可区分。

## Executor Registry

Phase 15 Manager 从单 executor 升级为注册表：

- default executor remains `lifecycle`
- explicit `verify` workflow selects verify executor
- unknown executor/workflow rejected
- executor name is persisted only in observed RunStatus, not desired state

This is necessary real use of the Executor abstraction, not speculative plugin scaffolding.

## Security / Architecture

- Control API remains loopback-only.
- Workflow uses `controlplane.Service.BuildRuntime` to construct the Runtime Adapter; no direct Native construction inside agent package.
- Runtime policy/capability enforcement remains authoritative below Agent.
- No raw shell, generic Process Manager, Docker or arbitrary command additions.
- No mutation/code-edit step in Phase 16; write/edit workflows are deferred until GSD/LLM planning has an auditable plan contract.
- Do not modify `D:\projects\codexprov4`.

## Exit Criteria

1. `agent run --workflow verify <workspace>` starts a daemon-owned Run and eventually reaches completed with structured plan/steps/review.
2. configured verifier pass produces `review.decision=pass`; configured verifier failure produces `review.decision=fail` while orchestration state remains completed.
3. unsupported/missing required capability becomes Run `error`, not silent success.
4. lifecycle executor compatibility remains intact.
5. cancel during workflow stops remaining steps and results in cancelled.
6. process-level CLI acceptance observes workflow state from a separate invocation.
7. `go fmt ./...`, `go test ./...`, `go vet ./...`, `go build ./cmd/ai-dev-manager` pass.

## Deferred

- natural-language/LLM planner — Phase 17
- GSD `.planning` executor and auto-advance — Phase 17
- code mutation/retry loop — Phase 17
- subagents/worktrees/parallel review — Phase 18
