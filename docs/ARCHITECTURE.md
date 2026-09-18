# ADM v1.1 工作机制与架构

本文解释 ADM 各处功能是如何协作的。它不是代码 API 文档，而是“为什么 CLI/Desktop/Agent 看起来这样工作、哪些状态会持久化、哪些权限在哪里检查”的产品机制说明。

实际命令见 [USER_GUIDE.md](USER_GUIDE.md) 与 [cli.md](cli.md)。Agent MCP 工具见 [AGENT_GATEWAY.md](AGENT_GATEWAY.md)。

## 1. 总体模型

ADM 可以理解为四层：

```text
+--------------------------------------------------------------+
| Human / Agent clients                                        |
|  adm CLI        adm-desktop        MCP Agent                 |
+-----------------------+------------------+-------------------+
                        |                  |
                 /admin/mcp             /mcp
                        |                  |
+--------------------------------------------------------------+
| HTTP / stdio Gateway                                         |
|  Admin surface        Agent surface       /healthz            |
|  persistent Runtime Owner: MCP/process/run/vfrun observations |
+--------------------------------------------------------------+
| Application/Core services                                    |
| Workspace Environment Files Runtime MCP Skill Memory          |
| Verifier Isolation Retention Discovery Capability Context     |
+--------------------------------------------------------------+
| Persistent store                                             |
| desired state + durable metadata                              |
+--------------------------------------------------------------+
```

重要的是：CLI、Desktop、Agent **不是三套产品模型**。

- Workspace/Environment/MCP/Skill/Memory 等核心数据只有一套。
- CLI/Desktop 管理都通过 Admin MCP/application contracts。
- Agent `/mcp` 暴露开发所需能力。
- Runtime Owner 保存当前 Gateway 生命周期内的 live observation。

## 2. Persistent desired state 与 Runtime observation 分离

这是理解 ADM 的关键。

### 2.1 Desired state

Desired state 表示“用户配置 ADM 希望是什么样”，会进入持久 store，例如：

- Workspace registration；
- Environment metadata；
- Writer lease metadata；
- exec allowlist；
- MCP definition；
- MCP health policy；
- Skill sources 与 discovery snapshot；
- Environment MCP/Skill selections；
- Memory；
- Verifier definitions；
- managed-worktree metadata；
- retention metadata。

### 2.2 Runtime observation

Observation 表示“当前 Gateway 进程已经看到了什么”，例如：

- 某 MCP 当前 session；
- 最近一次 MCP health；
- tool inventory；
- auto reconnect 的当前计数/下一次时间；
- `proc_` process；
- process log tail 和 listening port；
- `run_`；
- `vfrun_`。

这些 live observation **不写成 desired state**。

### 2.3 为什么必须分开

如果把 observation 持久化成“事实”，Gateway restart 后就会出现危险假象：

- 一个已经死掉的 process 仍被显示 running；
- 一个旧 MCP session 被认为仍健康；
- 一个 async Run 被错误地“恢复”；
- cleanup 因陈旧 blocker 被永久卡住，或反过来忽略真实 blocker。

所以 ADM restart 的语义是：

```text
persistent configuration survives
runtime observation starts fresh
```

## 3. Gateway Runtime Owner

HTTP Gateway 启动时创建一个 persistent Runtime Owner。

Owner 的职责包括：

- 持有外部 MCP live sessions 与 observations；
- 运行/观察长期 process；
- 运行/观察 async `run_`；
- 运行/观察 async verifier `vfrun_`；
- 在 context/capability report 中提供已有 observation；
- 在 Environment 删除或 Gateway shutdown 时清理 owned runtime resources。

Runtime Owner 有自己的 owner ID，可从 `/healthz` / `gateway status` 看到。

HTTP MCP handler 虽然是 stateless transport，但 Runtime Owner 本身是进程级共享状态，因此后来的客户端可以观察同一 Gateway owner 已经启动的 process/run/verifier。

## 4. Agent surface 与 Admin surface

### 4.1 Agent `/mcp`

Agent surface 的目标是“开发项目”，所以主要暴露：

- list/inspect Workspace/Environment；
- Environment context/capabilities；
- Writer；
- files；
- exec；
- process/run/verifier；
- Git/worktree；
- Environment-authorized MCP；
- Environment-authorized Skill；
- Memory read，以及 Environment-private Memory 操作；
- temporary Environment lifecycle。

它不会暴露很多全局管理写操作，例如任意添加 Workspace、修改 exec allowlist、修改 global MCP/Skill catalog 等。

### 4.2 Admin `/admin/mcp`

Admin surface 是 Agent surface 的管理 superset。CLI/Desktop 用它管理：

- Workspace add/rename/remove；
- durable Environment create/rename/remove；
- exec allowlist；
- global MCP/Skill catalog；
- Skill Sources；
- Global Memory write/delete；
- resource retention admin operations；
- management snapshot。

### 4.3 为什么分面

这样可以避免“一个拿到 Agent 开发工具的客户端就天然拥有机器级配置管理权”。

HTTP Gateway 默认监听 loopback；显式启用非 loopback 监听时，必须同时配置 Host allowlist、Admin API Key 与 Agent API Key。Admin surface 始终应被视为 privileged management interface。

## 5. CLI 为什么依赖 Gateway

正常 `adm workspace/environment/exec/mcp/skill/memory` 命令不会直接打开 `state.json` 修改。

执行路径是：

```text
CLI
  -> select Base URL (--adm-url / ADM_V2_URL)
  -> /admin/mcp
  -> application service
  -> persistent store
```

如果 `/admin/mcp` 不可达，命令失败。没有“远端失败后偷偷修改本机 state”的 fallback。

这样避免两个问题：

1. Desktop/CLI/Gateway 各自实现不同验证规则；
2. 多进程同时直接写 state 导致第二套 authority/persistence 模型。

本机 `gateway` / `doctor` / `state` 是明确的 bootstrap/recovery 例外。

## 6. Workspace 授权机制

Workspace 是文件系统 authority 的第一层。

创建 Workspace 时：

1. caller 提供现有目录；
2. ADM canonicalize path；
3. 持久化 stable Workspace ID 与 path；
4. 之后 Environment 必须引用已注册 Workspace。

Workspace remove 只删 metadata，不删目录；仍有 Environment 引用时阻止。

### 6.1 Path canonicalization

Windows 上同一个目录可能有长路径和 8.3 短路径，例如：

```text
C:\Users\runneradmin\...
C:\Users\RUNNER~1\...
```

ADM 使用 canonical path equality，而不是普通字符串 equality。这一点既用于 containment，也用于“同一 physical root 的 Writer 冲突”判断。

## 7. Environment authority

Environment 在 Workspace 内进一步缩小 scope。

创建时：

1. resolve Workspace；
2. root 为空时采用 Workspace root；
3. canonicalize root；
4. 验证 root 是已存在目录；
5. 验证 root 位于 Workspace 内；
6. 从当前 global MCP/Skill default settings 复制初始 selection；
7. 创建 stable `env_...`。

因此 Environment 是“开发上下文 + root authority”，不是当前 shell cwd 的隐式选择。

ADM 没有隐藏的 current Environment。所有 Agent/CLI operation 都用 stable ID。

## 8. Read authority 与 Mutation authority

ADM 把权限分成两个维度：

### 8.1 Scope authority

由 Workspace/Environment root 决定“能碰哪里”。

### 8.2 Mutation authority

由 Writer lease 决定“谁现在能写”。

这意味着：

- 能 read Environment 不等于能 write；
- Writer 也不能把 path 扩出 Environment root；
- 允许 executable 也不等于没有 Writer 就能执行。

## 9. Writer lease 内部机制

Writer 是 physical root 级别的单写者 lease。

### 9.1 Acquire

`environment_writer_acquire(env, owner)` 会：

1. 找到目标 Environment；
2. 扫描所有 Environment；
3. 对 canonical physical root 相同的 Environment 检查 active Writer；
4. 清掉已过期 lease；
5. 如果同 Environment/同 owner 已持有，则续租；
6. 如果其他 owner 已持有同 physical root，拒绝；
7. 否则创建 lease。

### 9.2 TTL

默认 5 分钟。lease 带：

- owner；
- acquired_at；
- last_seen_at；
- expires_at。

过期 lease 在 view/authority check 中不再有效。

### 9.3 Mutation heartbeat

Runtime mutation 在验证 Writer 后会更新 lease；长运行 owned operations 也会 heartbeat，使一个合法的长命令不会仅因为持续时间长就意外失去 authority。

## 10. Filesystem containment

所有文件/Runtime cwd 都经过 Environment containment。

ADM 的目标不是“给 Agent 一个 shell，然后信任它自己别跑出去”，而是服务端在 operation boundary 上检查 scope。

常见防护：

- path canonicalization；
- relative path revalidation；
- symlink/actual directory resolution；
- managed worktree identity revalidation；
- delete 只支持单文件，不提供递归目录删除 Agent tool。

## 11. Exec allowlist 工作机制

Exec authority 是独立的全局 management state。

一次执行要同时满足：

```text
Environment exists
+ Writer matches
+ executable is allowed
+ cwd remains inside Environment
+ managed root (if any) still valid
+ operation-specific timeout/output rules
```

因此 allowlist 只是必要条件之一。

### 11.1 为什么是 executable allowlist

ADM 不把整个 shell command line 当持久授权项。Runtime 接口接受：

```text
executable + args[] + cwd
```

这种结构更容易：

- 明确允许哪个程序；
- 不把参数/secret 作为 allowlist key；
- 复用到 exec/process/run/verifier/stdio MCP。

### 11.2 Denial observation

被拒绝的 executable 会产生轻量 observation，帮助 Desktop 管理员判断是否需要 allow。记录不包含 args/output，降低敏感数据持久化风险。

## 12. MCP 内部机制

MCP 是 ADM 最容易被混淆的模块之一。

### 12.1 Definition

持久 MCP definition 包含：

- stable MCP ID；
- name；
- transport；
- endpoint 或 executable/args；
- auth mode；
- header/env references；
- health policy；
- default include。

这是 desired configuration，不是 health。

### 12.2 Selection

Environment 只存 `EnabledMCPIDs`。

创建 Environment 时从 global defaults 复制一次。以后 global default 改变不反向修改已有 Environment。

### 12.3 Activation

外部 MCP 真正使用前，ADM 在 activation boundary：

1. 检查 Environment selection；
2. 读取 definition；
3. 验证 transport；
4. 解析 secret refs；
5. stdio 时验证 executable allowlist；
6. 创建连接；
7. initialize/Ping/list tools；
8. 保存 owner-local observation。

Secret literal 不应该成为 persisted definition 的普通字段。

### 12.4 Streamable HTTP

HTTP MCP 使用 MCP Streamable HTTP client transport。header refs 在 activation 时解析并注入请求。

### 12.5 stdio

stdio MCP 使用本机 child executable，因此：

- executable 必须被 allow；
- Environment-authorized runtime 调用会使用对应 authority/cwd 逻辑；
- global `mcp_probe` 不绑定 Environment，所以使用 ADM 临时目录，不推断 project cwd。

### 12.6 Probe / Status / Inspect / Refresh

四者副作用不同：

| 操作 | Environment | 网络/进程连接 | 读取已有 observation | 写 desired state | 调业务 tool |
|---|---:|---:|---:|---:|---:|
| global probe | 否 | 是，transient | 否 | 否 | 否 |
| environment status | 是 | 可进行健康探测 | 可 | 否 | 否 |
| inspect | 是 | **否** | 是 | 否 | 否 |
| refresh | 是 | 是，显式重连/Ping/list tools | 更新 | 否 | 否 |

这张表是 MCP 诊断语义的核心。

### 12.7 Tool call

Agent 应先通过 `environment_injection_plan` 判断 MCP 是否 `injectable=true`。`environment_mcp_tools` / `environment_mcp_call` 只有在：

- Environment 选择允许；
- activation healthy；
- MCP ID 有效；

时才成立。

失败是 operation-local，不会使普通 file/exec/Skill 失效。

### 12.8 Health/reconnect

Persistent Runtime Owner 可以根据 definition 的 health policy 做周期检查/固定间隔 reconnect。

自动 reconnect 不会 replay 失败 tool call，因为业务 call 是否安全重放是外部 Agent 的决策，不应由 ADM 猜测。

## 13. MCP import 机制

Import 是 normalization boundary，不是“把外部 JSON 原样存进去”。

流程：

```text
external JSON/JSONC
  -> source adapter / format detection
  -> canonical candidates
  -> credential literal -> reference requirement
  -> validation
  -> preview
  -> explicit apply
  -> atomic global catalog mutation
```

原始 import blob 不持久化。apply 也不修改已有 Environment selections。

Conflict policy：

- `error`
- `skip`
- `update_by_name`

batch apply 走同一 validation path，保持原子性。

## 14. Skill 内部机制

### 14.1 Source 与 Skill 分离

Skill Source 是 discovery 配置：

- root；
- support roots；
- default include；
- refresh status。

Skill entry 是一次成功 discovery snapshot 中发现的 artifact identity。

### 14.2 Source update 不等于 refresh

`source-update` 只更新 desired source config，并把 source 标记成需要 refresh；不扫描 filesystem。

`source-refresh` 才读取 source root，发现 `SKILL.md` 并原子替换该 source-owned snapshot。

这样能保证：

- UI 编辑路径不会产生隐藏 scan；
- refresh 失败时旧 snapshot 可以保留；
- source config 与 artifact availability 可以分别诊断。

### 14.3 Support roots

Skill 可以需要 shared supporting files，但必须由 source 明确配置 support roots。

Environment Skill read authority 是：

```text
Skill enabled in Environment
AND target belongs to artifact root or explicit support root
```

不是“Skill 里写了一个路径就自动授权”。

### 14.4 Availability

ADM 有两种 availability：

- global structural availability：artifact/source/support 是否结构上可用；
- Environment availability：再叠加 Environment enabled/disabled/unresolved。

Availability inspection 不等于读取/解释 Skill instructions。

## 15. Memory 工作机制

Global Memory 与 Environment-private Memory 共用持久 service，但 key space/scope 明确分开。

### 15.1 Global

不依赖 Environment 生命周期。Environment 删除不会删除 Global Memory。

### 15.2 Environment-private

存放在 Environment 私有上下文。Environment 删除时会随 Environment metadata 消失，但不会影响 project files。

### 15.3 Privacy defaults

Management snapshot、Environment list/inspect、Context Bundle 不自动暴露 private values。

这是“默认最小披露”，不是 Memory 不可用；需要内容时必须走显式 `memory_*_read`。

## 16. Verifier 工作机制

Verifier definition 是持久声明，不是执行权限。

Definition 包含：

- kind：test/lint/build/custom；
- executable；
- args；
- cwd；
- timeout；
- enabled。

执行时还必须通过 Runtime allowlist/Writer/cwd containment。

### 16.1 Sync verifier

`environment_verifier_run` 在当前 request 中完成执行并返回 structured result。

### 16.2 Async verifier

长任务使用 Runtime Owner 的 `vfrun_`：

1. prepare verifier execution；
2. 完成所有 authority check；
3. 创建 owner-local verifier run；
4. child process 执行；
5. bounded capture stdout/stderr；
6. heartbeat Writer；
7. terminal 时保留 structured verifier result；
8. owner shutdown / Environment drop / explicit cancel 时终止 process tree。

它不会写入 state、不会 restart resume。

## 17. Process 与 Run 为什么是两个概念

### 17.1 Process

用于“长期服务”，例如 dev server。

重点是：

- 进程可以持续 running；
- bounded log tail；
- listening port observation；
- stop by `proc_` ID。

### 17.2 Run

用于“异步执行一个命令，最终会结束”。

重点是：

- `running/succeeded/failed/canceled`；
- bounded running/terminal output；
- cancel by `run_` ID；
- 仍保持 task-semantic-neutral。

### 17.3 Verifier Run

`vfrun_` 再独立一层，因为 verifier 有自己的 structured classification/definition/timeout 语义，不能简单降级成 opaque `run_`。

## 18. Git 是 operation-local capability

Git status/diff/branch 在 Runtime 中按需检查。

没有 `.git`：

```text
git_status -> clear local capability failure
read/write/search/exec -> unaffected
```

这体现 ADM 的“一个 capability 失败不污染其他能力”原则。

## 19. Managed worktree identity

ADM-managed worktree 不是普通 Environment flag，而是额外持久 identity：

- managed worktree ID；
- source Workspace；
- Environment ID；
- generated branch；
- base commit；
- Git common-dir；
- managed root。

创建要求 Workspace root 本身是 Git top-level。

调用 routed Runtime operation 时会 revalidate：

- root 仍是那个 worktree；
- common-dir 匹配；
- top-level 匹配；
- branch identity 未被偷偷替换。

Destroy 也不是普通 Environment remove：必须走 Git safety，检查 dirty/unpublished 等事实。

## 20. Temporary Environment retention 机制

Temporary Environment 在创建时就原子写入 temporary retention，而不是先造 durable 再偷偷改。

Retention 包括：

- persistence class；
- creator surface；
- owner ID；
- creation time；
- expiry；
- optional session/run provenance。

### 20.1 TTL 不等于自动删除

过期只表示“时间条件可能满足”。真正 cleanup 还要 fresh runtime/Git safety check。

### 20.2 Cleanup preview

preview 不 mutation，汇总当前 blockers/uncertainties。

### 20.3 Execute

execute 再做一次 fresh recheck，防止 preview 后状态发生变化。

ordinary root：只删 Environment state/private context。

managed worktree：调用 managed destroy safety，并且 temporary workflow 不提供 force。

### 20.4 Promote

matching lifecycle owner 可以把 retention 改成 durable。Environment ID/root/MCP/Skill/Memory 等上下文不变。

## 21. Capability Report

Capability Report 是统一的结构化“现在这个 Environment 能做什么/为什么不能做”。

它的设计目标是：

- optional capability failure 可诊断；
- 不需要 Agent 逐个试错；
- 不为了诊断主动产生副作用。

静态 application report 可以包含 file/exec/verifier/Git/isolation/MCP/Skill 等 facts。

有 Runtime Owner 时可加入已有 MCP/process/run observation，但不会为了 report 主动 connect/probe/execute。

## 22. Environment Context Bundle

Context Bundle 是 Capability Report 的 Agent onboarding 视图，不是 prompt orchestration。

组成：

```text
identity
+ root/tree digest
+ available capabilities / issues
+ MCP summaries
+ Skill summaries
+ verifier summaries
+ guidance
+ omissions/truncation evidence
```

它有 hard budgets，会 compact/truncate，并明确记录 omissions。

默认 privacy boundary：

- 不读 Memory values；
- 不读完整 Skill content；
- 不主动 MCP connect；
- 不运行 verifier；
- 不拿 Writer；
- 不创建 Run/process。

## 23. Workspace discovery / tree digest 为什么是 bounded metadata-only

自动扫描整个磁盘或无限递归会带来：

- 不可预测 latency；
- 权限外扩风险；
- 巨量输出；
- UI/Agent 隐藏后台工作。

所以 discovery 只在明确 Workspace/Environment 内进行，并带：

- max depth；
- max entries；
- max candidates；
- max digest entries；
- max output bytes。

达到预算会返回 partial/omission evidence，而不是偷偷继续扫。

## 24. Desktop 的状态模型

Desktop 本身不拥有第二套业务状态。

它保存的主要本地 UI/连接配置是 connection profiles，例如：

```text
~/.config/adm/desktop-connections.json
```

真正 Workspace/Environment/catalog/Memory 数据仍来自当前 `/admin/mcp`。

CLI 与 Desktop 共享用户级 dotenv 配置 `~/.config/adm/.env`，并可由各自 executable 同目录 `.env` 覆盖。进程显式环境变量仍拥有最高优先级。这些 `.env` 属于客户端/进程配置，不取代 Core persistent state。

### 24.1 Connection generation / stale result guard

切换连接或 scope 时，Desktop 会：

1. 清空旧连接数据；
2. 增加 generation/scope identity；
3. 发新请求；
4. 旧请求即使晚回来，也不能覆盖新 scope UI。

这避免了“切到服务器 B，却突然显示服务器 A 的结果”。

## 25. Gateway shutdown 与资源清理

Gateway shutdown 会：

- cancel Runtime Owner reconciliation；
- cancel active `run_` / `vfrun_`；
- stop owned process trees；
- close MCP sessions；
- bounded wait cleanup；
- shutdown HTTP server。

它不会把这些 runtime observations 持久化后下次复活。

## 26. 故障如何局部化

ADM 的核心错误传播原则：

```text
one optional capability fails
!= Environment unusable
!= Workspace unusable
!= Gateway unusable
```

例子：

- Git 不可用，只影响 Git/worktree。
- Skill artifact missing，只影响那个 Skill。
- 一个 MCP auth error，只影响那个 MCP。
- Verifier executable 未 allow，只影响 verifier execution。
- managed worktree 被 tamper，只影响该 managed root 的 routed operations。

这种 operation-local failure 是 ADM 不把“开发环境”变成脆弱大一统 prerequisite chain 的关键。

## 27. 当前安全边界与非目标

v1.1 明确不提供：

- 非 loopback 未认证 Remote Admin MCP；
- 任意全盘文件系统访问；
- 任意 OS PID process manager；
- Agent recursive directory delete；
- 自动 Memory/Skill 全量 prompt injection；
- 自动 task/GSD orchestration；
- 自动 merge/rebase/push；
- temporary auto-GC；
- MCP business call replay；
- Run/verifier restart resume。

这些不是“忘了做”，而是当前产品边界。

## 28. 数据/权限速查表

| 对象/能力 | 是否持久化 | 是否 Environment-scoped | 是否通常需 Writer | 是否需 exec allowlist |
|---|---:|---:|---:|---:|
| Workspace | 是 | 否 | 否 | 否 |
| Environment | 是 | — | 否 | 否 |
| Writer | lease metadata | 是/physical root | — | 否 |
| File read/search/tree | 否 | 是 | 否 | 否 |
| File write/edit/delete | 项目文件本身 | 是 | 是 | 否 |
| Exec | result 不作为 desired state | 是 | 是 | 是 |
| MCP definition | 是 | global | 否 | stdio 需要 |
| MCP live session/inventory | 否 | 是/owner-local | 否 | stdio 需要 |
| Skill source/catalog | 是 | global | 否 | 否 |
| Skill content read | 否 | 是 | 否 | 否 |
| Global Memory | 是 | global | 否 | 否 |
| Private Memory | 是 | 是 | 否（显式 Memory mutation 自身授权） | 否 |
| Verifier definition | 是 | 是 | 否 | 否 |
| Verifier execution | observation owner-local | 是 | 是 | 是 |
| Process `proc_` | 否 | 是/owner-local | start/stop 是 | 是 |
| Run `run_` | 否 | 是/owner-local | start/cancel 是 | 是 |
| Verifier Run `vfrun_` | 否 | 是/owner-local | start/cancel 是 | 是 |
| Managed worktree metadata | 是 | Environment | destroy 是 | Git capability |
| Temporary retention | 是 | resource/Environment | cleanup 本身按 lifecycle owner/safety | 按内部 blocker |

## 29. 设计时应保持的 invariant

后续开发 Phase 24+ 时，下列 invariant 不应因为 UI/package 重构而改变：

1. Workspace/Environment 不要求 Git。
2. Desktop/CLI 不创建第二 state model。
3. Agent 与 Admin privilege boundary 保持。
4. normal CLI 管理不 direct-write state fallback。
5. stable ID routing，不引入隐藏 current Environment。
6. Writer 继续按 physical root 限制 mutation。
7. executable allowlist 继续被所有本机执行路径复用。
8. MCP desired config 与 runtime observation 分离。
9. Skill source config 与 explicit refresh 分离。
10. Memory scopes 不隐式合并。
11. Runtime resources 不跨 Gateway restart 伪恢复。
12. temporary cleanup preview-first、fresh recheck、无 force。
13. optional capability failure operation-local。
14. ADM 不吸收 Agent/GSD task orchestration。

以上就是 v1.1 的工作机制基线。
