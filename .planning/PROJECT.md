# ai-dev-manager

## What This Is

一个面向现有 AI Coding Agent 的 **AI Coding Environment**。`ai-dev-manager` 不内置或替代 ChatGPT、Codex、Claude Code、OpenCode、WorkBuddy、CodexPro 等 Agent，而是通过统一的 Control Plane、Runtime 与 MCP 能力，为它们提供可隔离、可管理、可验证的开发环境。

## Core Value

**让同一个工程可以同时承载多个互不污染的开发任务：每个任务拥有独立 Environment / managed worktree / branch / Runtime，外部 Agent 通过统一能力安全工作，main checkout 与其他任务保持隔离。**

产品优先级固定为：

1. 工具本身可靠可用。
2. 让外部 AI Agent 通过 MCP 用得好。
3. 再让人通过 CLI / Manager / UI 用得好。

## Product Positioning

不是另一个 Agent，也不是另一个“文件读写 MCP Server”，而是：

> AI Coding Environment

ADM 管理 Environment 的事实、能力、安全边界与执行；Agent 管理开发意图、判断和需要用户确认的决策。ADM 可以返回 `facts` / `warnings` / `hints` 形式的轻量建议，但不得把开发建议伪装成必须执行的下一步。

## Relationship With Existing Projects

- CodexPro：继续作为成熟 Agent 使用；借鉴其 workspace containment、safe execution、structured file/search/edit、handoff / `.ai-bridge` 和 Agent-friendly tool experience。
- DevSpace：借鉴 managed worktree、runtime、skills、execution contract、environment lifecycle 等成熟思路。
- `codexpro-plus`：当前是已发布且验证可用的 CodexPro 进程管理工具，原功能不为 ADM 实验修改；未来如需 Manager 集成，优先 fork/独立实验版本，不影响稳定版本。
- `codexprov4`：继续独立存在；不作为 ADM Core 的依赖。
- 本项目不 fork CodexPro 或 DevSpace Core；优先吸收设计，并通过 Adapter / Capability / MCP 接口保持兼容空间。

## Current Milestone: v0.5 Multi-Task Development Environments ✅ Complete

**Goal:** 把 v0.4 已验证的 managed worktree + derived Runtime 从临时 parallel lane 提升为持久的一等 Environment，使同一 Project 能长期并行承载多个隔离开发任务；先完成 Environment Core，再在后续 milestone 建立统一 Agent MCP Gateway 与人类 Manager/UI。

## Requirements

### Validated — v0.5 Phase 21

- [x] **ENV-06**: inspect 通过结构化 Runtime Git relation 报告当前 base 的 `ahead` / `behind` / `diverged` / `base_moved`，base ref 不可用时只返回 warning；base 不自动同步。明显 divergence 或 behind>=10 时最多返回一条非指令性确认 hint。— Real Git + cross-process CLI acceptance passed.
- [x] **ENV-08**: Environment 持久化 single-writer lease；同 owner acquire 幂等 renew，第二 owner 返回 `writer_conflict`，普通 release 需 owner 匹配，显式 force release 只清 lease；writer 跨 daemon restart 保留且 stale 不自动释放。— Manager + CLI restart acceptance passed.
- [x] **UX-01**: Environment inspect 返回 facts/warnings/hints，包含 dirty/upstream/base/activity/writer 状态；测试确认不暴露 `required_action` / `next_step`，也不自动 rebase/merge/push/commit/cleanup。— Phase 21 acceptance passed.

### Validated — v0.5 Phase 20

- [x] **ENV-03**: 默认 create 仍从 committed HEAD/base 建 Environment 并对 dirty source 返回 `changes_not_included`；显式 `--include-changes` 会保留 staged/unstaged index 语义、复制 untracked regular files、排除 ignored files，并拒绝把 untracked symlink 静默解引用成普通文件；binary patch 也通过真实 Git acceptance。— Runtime + Manager + cross-process CLI acceptance passed.
- [x] **ENV-07**: 普通 destroy 拒绝 dirty、未推送提交和 active writer；有 upstream 且 local ahead=0、clean 且无 writer 的 Environment 可销毁；显式 `--force` 才能覆盖这些 destructive guards，missing worktree 时只清 Registry record，所有路径都继续依赖 managed inventory revalidation，branch 永不自动删除。— Real Git and CLI acceptance passed.

### Validated — v0.5 Phase 19

- [x] **ENV-01**: Environment 是持久一等对象，拥有稳定 `env_` identity，并绑定 base Workspace、name、branch、base ref/base commit、managed worktree path 与生命周期时间；daemon restart 后通过持久 Store + 实际 managed worktree 重新验证并构建 derived Runtime，而不是持久化内存 Runtime。— Real manager + CLI restart acceptance passed.
- [x] **ENV-02**: Environment 默认从创建时当前 checkout HEAD 创建，且可显式指定 branch/tag/commit base；默认不会把 dev/master 写死。— Real Git tests covered current `dev` and explicit older commit.
- [x] **ENV-04**: 默认 branch 为 `adm/<sanitized-name>`，正常支持 `/`；已存在 branch 返回明确冲突，不自动生成替代 branch。— Runtime + Environment tests passed.
- [x] **ENV-05**: Phase 19 Environment lifecycle 不自动 commit/merge/squash/cherry-pick/rebase，destroy 也不删除 branch。— CLI acceptance verified branch remains after clean destroy.
- [x] **ENV-09**: Environment path 不能仅凭持久记录获得授权；每次 inspect/destroy 都通过 base Runtime 的 managed worktree inventory 重新验证 worktree，再构建 derived Runtime。— Missing/tampered worktree validation tests passed.

### Validated

- [x] **CONF-01**: 支持 Global 配置。— Resolver validated in Phase 1; JSON persistence validated in Phase 2
- [x] **CONF-02**: 支持可选 Profile 配置。— Resolver validated in Phase 1; profile persistence validated in Phase 2
- [x] **CONF-03**: 支持 Project 私有配置。— Resolver validated in Phase 1; project persistence/isolation validated in Phase 2
- [x] **CONF-04**: 支持 Runtime Override。— Resolver validated in Phase 1; Phase 2 verified runtime override remains in-memory and is not persisted
- [x] **CONF-05**: 按 `Runtime > Project > Profile > Global` 解析 EffectiveConfig。— Validated in Phase 1
- [x] **CONF-06**: 支持 enable / disable / override，而不是要求项目复制完整全局配置。— Validated in Phase 1
- [x] **CONF-07**: 配置来源可追踪，能够解释实体主来源和最终 enable/disable 来源。— Validated in Phase 1
- [x] **RUN-01**: Control Plane 与 Runtime 分离；Runtime 只消费 EffectiveConfig。— Phase 1 validated the Core boundary and EffectiveConfig contract; concrete Runtime consumption will be exercised again in Phase 4
- [x] **COMPAT-01**: 核心不绑定桌面 UI。— Validated in Phase 1
- [x] **WORK-01**: 注册多个 Workspace，并为每个 Workspace 保存稳定 ID、路径、Profile 和 Runtime 选择。— Validated in Phase 2 with persisted two-workspace reload coverage
- [x] **MCP-01**: Global MCP Catalog 可被所有 Workspace 继承。— Validated in Phase 3 with two-workspace integration coverage
- [x] **MCP-02**: Profile 可追加或覆盖 MCP。— Persistence/resolution validated in Phase 2; stable Catalog consumption validated in Phase 3
- [x] **MCP-03**: Project 可配置私有 MCP。— Validated in Phase 3; Project A private entry does not appear in B
- [x] **MCP-04**: Project 可禁用继承来的某个 MCP。— Validated in Phase 3; disabled entry remains in Catalog with EnabledSource=project
- [x] **MCP-05**: MCP 定义包含稳定 ID、transport、command/url、env、scope、enabled、health/capability 扩展位。— Validated in Phase 3; health intentionally defaults to unprobed
- [x] **SKILL-01**: 支持全局 Skill 路径，GSD 不需要复制到每个工作目录。— Validated in Phase 3 with one Global root resolving into two Workspaces
- [x] **SKILL-02**: 支持 Project 私有 Skill。— Validated in Phase 3 with Project A/B isolation
- [x] **SKILL-03**: 支持继承、显式禁用和项目覆盖。— Resolver disable semantics validated in Phase 1; discovery/explicit/project/runtime override precedence validated in Phase 3
- [x] **SKILL-04**: `.planning/` 作为项目 GSD 状态与决策记录，不与全局 Skill 安装目录混用。— Validated in Phase 3; discovery only scans explicitly configured SkillRoots
- [x] **WORK-02**: Workspace 之间默认隔离，不允许一个 Runtime 越界访问其他 Workspace。— Validated in Phase 4 with absolute/relative/symlink escape tests
- [x] **EXEC-01**: 支持安全受控的系统命令执行，而不是仅依靠模型遵守提示词。— Validated in Phase 4 with execution-layer policy enforcement
- [x] **EXEC-04**: Windows 执行环境支持显式 executable path，不假设 Runtime PATH 等于宿主 PATH。— Validated in Phase 4 via ToolPaths helper executable
- [x] **SEC-01**: ReadOnly / WorkspaceWrite / Standard / Full 权限策略在执行层强制生效。— Validated in Phase 4
- [x] **WORK-03**: 同一项目可绑定 Main checkout 或独立 Git worktree。— Validated in Phase 5 with managed worktree create/list/remove while preserving the main checkout
- [x] **EXEC-02**: 支持 Shell、Git 和 Verification 作为第一批执行能力。— Structured Shell validated in Phase 4; Git + Verification validated in Phase 5
- [x] **VERIFY-01**: Project 可以声明结构化 verifier（test / lint / build / custom）。— Validated in Phase 5 via v1 JSON + resolver coverage
- [x] **VERIFY-02**: Runtime 可执行 verifier 并返回结构化结果。— Validated in Phase 5 with pass/fail/skip/timeout coverage
- [x] **VERIFY-03**: 修改代码后的验证结果可追踪，不只返回一段 shell 文本。— Validated in Phase 5 with real Edit → GitDiff → test/build acceptance
- [x] **RUN-02**: Runtime 通过 capability 描述自身支持的能力。— Capability-driven model validated across Phases 4–6; Files/Search/Edit/Shell/Git/Worktree/Verify are advertised only when supported, while unsupported Docker/Debug are intentionally absent rather than faked
- [x] **RUN-03**: 支持多个 Workspace 使用独立 Runtime / MCP Server 实例。— Native contexts validated in Phase 4; two simultaneous isolated Streamable HTTP MCP instances validated in Phase 6
- [x] **EXEC-03**: 为 Docker、Process、Debug 能力预留结构化扩展接口。— Generic capability + Runtime Adapter contract validated in Phase 6 without speculative implementations
- [x] **COMPAT-02**: 为未来 CodexPro v4 Adapter 预留稳定接口。— Runtime Adapter contract and mapping documented in Phase 6; codexprov4 remains untouched
- [x] **COMPAT-03**: 为未来 DevSpace Runtime Adapter 预留稳定接口。— Runtime Adapter contract and mapping documented in Phase 6
- [x] **COMPAT-04**: 为外部 MCP / Agent runtime 预留 Adapter 接口，不要求重写 Core。— Validated in Phase 6 with a non-Native fake external runtime hosted through the same manager/MCP status path

### Validated — v0.2

#### Control Plane

- [x] **CTRL-01**: 从 Workspace ID 统一解析 Workspace + EffectiveConfig，并构造 Native Runtime / Runtime Adapter；调用方不再手工拼装这些组件。— Phase 7 validated with persisted Workspace → Resolve → NativeAdapter composition and structured unsupported-runtime errors
- [x] **CTRL-02**: Control Plane 提供 Workspace runtime introspection，至少返回 workspace identity、runtime selection、policy、capabilities、MCP/Skill/Verifier effective state 与来源信息。— Phase 7 validated deterministic safe snapshot with Source/EnabledSource and no Env/EnvRefs values
- [x] **CTRL-03**: Control Plane 统一管理本进程内 MCP Host instance 的 start/list/get/stop 生命周期，并保持一个 instance 固定绑定一个 Workspace Runtime。— Phase 7 validated two persisted Workspace MCP instances with official clients, isolation, stop independence and loopback host reuse

#### CLI

- [x] **CLI-01**: 提供真正的 `ai-dev-manager` 可执行入口，并保持 CLI 为 Control Plane 的薄客户端，不复制 resolver/runtime 逻辑。— Phase 8 validated standard-library CLI entrypoint importing Control Plane/Workspace only for composition/CRUD
- [x] **CLI-02**: CLI 支持 workspace add/list/show 与 effective/runtime inspect 的最小日常管理闭环。— Phase 8 validated temp-root add/list/show/inspect JSON flow
- [x] **CLI-03**: CLI 支持按 Workspace 启动 foreground loopback MCP serve，输出可连接 endpoint，并在进程退出/信号时安全停止。— Phase 8 validated cancellable foreground serve with official MCP client read and graceful shutdown

#### MCP Runtime Activation

- [x] **MCPA-01**: 已配置且 Enabled 的 MCP Definition 能从 EffectiveConfig 进入显式 activation/probe 生命周期；disabled 项不启动。— Phase 9 validated real stdio and Streamable HTTP Connect + ListTools probes; disabled entries are not activated
- [x] **MCPA-02**: Global/Profile/Project 的 MCP 继承差异在真实 activation 中保持 Workspace 隔离，实际 secret 只在启动边界通过 EnvRefs 解析，不写回配置或普通状态输出。— Phase 9 validated Global MCP A/B isolation, Project disable, EnvRef resolution and safe status output
- [x] **MCPA-03**: activation health/status 可通过 Control Plane/CLI 检查；失败必须结构化报告，不能把配置存在误报成 runtime healthy。— Phase 9 validated unprobed/disabled/healthy/error lifecycle, structured missing-EnvRef failure and CLI MCP surface

### Validated — v0.3 Phase 10

#### Local Daemon / Control API

- [x] **LIFE-01**: 提供一个 local-only daemon，长期持有单一 `controlplane.Service`；启动 CLI 退出后 daemon 继续存活。— Phase 10 validated with cross-process CLI test and real Windows binary acceptance
- [x] **LIFE-02**: 独立 CLI invocation 可以通过稳定 discovery metadata 找到同一个 daemon，并完成 status/stop；同一 config root 只有一个健康 owner。— Phase 10 validated same instance/PID across start/status/repeat-start/stop
- [x] **LIFE-03**: daemon lifecycle metadata 只保存 PID/instance identity/control endpoint 等可恢复信息；不序列化 Runtime、MCP ClientSession、listener 或其他内存对象。— Phase 10 validated JSON metadata + heartbeat lease + loopback endpoint safety

### Validated — v0.3 Phase 11

#### Persistent Workspace Runtime Ownership

- [x] **LIFE-04**: daemon 可以长期拥有多个 Workspace runtime instance，并提供跨进程 start/status/stop。— Phase 11 validated daemon-owned RuntimeOwner, two persistent Workspace MCP hosts, runtime list/status/stop and real binary acceptance
- [x] **LIFE-05**: configured MCP activation 与 MCP Host instance 由 daemon 中同一个长期 Control Plane 管理，CLI 退出不再自动结束 session。— Phase 11 validated retained configured HTTP MCP session stays healthy through later owner status calls and daemon control surface
- [x] **LIFE-06**: Workspace runtime lifecycle 使用 desired/observed state；A/B ownership、health 与 stop/failure 相互隔离。— Phase 11 validated idempotent start, activation rollback, distinct A/B endpoints and stop-A-with-B-still-running

### Validated — v0.3 Phase 12

#### Restart / Reconciliation

- [x] **LIFE-07**: daemon 重启可从最小 desired state 重建应该运行的 Workspace runtime/MCP，不尝试恢复已经失效的内存对象。— Phase 12 validated persisted sorted desired Workspace IDs, clean restart reconciliation, explicit-stop persistence and rebuilt runtime/MCP endpoints
- [x] **LIFE-08**: crash/stale metadata 可安全修复；重启后旧 endpoint/session 不会被误报 healthy，并通过真实多进程 acceptance。— Phase 12 validated direct daemon kill, old endpoint unreachability, bounded stale-lease reclaim, new daemon identity and recovered live runtime

### Validated — v0.3.1

- [x] **USE-01**: Workspace MCP 默认 loopback-only，但可由用户显式选择 Docker-reachable bind；该 desired bind intent 跨 daemon restart 保留，Control API 不随之暴露。— Phase 13 validated explicit exposed Host path, v1→v2 desired compatibility, `runtime start --docker`, restart reconciliation, Host/Origin allowlist and real `mcphub` Docker MCP initialize
- [x] **USE-02**: 提供 `up/down/ps/ctl` 日常 CLI，隐藏常规 Workspace ID 与 daemon 启动顺序，同时保留现有底层命令兼容性。— Phase 14 validated one-shot path auto-registration, `up --docker`, path-based `down`, merged `ps`, desired-preserving `ctl restart/stop`, and desired-clearing `ctl shutdown`

### Validated — v0.4 Phase 15

- [x] **ARUN-01**: Agent Run 由 daemon 长期拥有；创建 Run 的 CLI 退出后，Run 仍可由独立 CLI 查询。— Phase 15 process-level + real binary acceptance verified `agent run` returns while later `agent list/status` observes the same running Run
- [x] **ARUN-02**: Agent Run 提供稳定 `run_` identity、Workspace identity、executor、running/completed/cancelled/error 状态与时间信息，并支持 list/status/cancel。— Phase 15 unit tests cover completed/error/cancel/isolation; CLI acceptance covers list/status/idempotent cancel
- [x] **ARUN-03**: Run 是 observed state；daemon shutdown 会取消 active Runs，restart 不得把旧 Run 误报为 running，且 Agent Control API 继续只存在于 loopback daemon boundary。— Phase 15 real binary restart changed daemon identity and returned an empty Agent list; handlers share the existing loopback Control API only

### Validated — v0.4 Phase 16

- [x] **FLOW-01**: Agent Run 可显式选择 production workflow executor，同时保留 `lifecycle` 默认兼容。— Phase 16 adds executor registry and `agent run --workflow verify`
- [x] **FLOW-02**: Planner / Executor / Reviewer 通过结构化 Plan、StepResult、ReviewResult 协作，并把审计轨迹暴露在 Run status。— verify workflow records three-step plan, step outputs and review decision
- [x] **FLOW-03**: reviewer 业务失败与 orchestration error 分离，capability/runtime 错误和 cancellation 保持明确。— unit tests cover pass/fail, missing capability and cancel race; reviewer fail keeps Run completed

### Validated — v0.4 Phase 17

- [x] **GSD-01**: `gsd` workflow 从 Workspace `.planning/STATE.md` 解析当前 Phase/Plan，并读取 PROJECT/CONTEXT/PLAN provenance。— process-level acceptance records planning source paths in Run status
- [x] **GSD-02**: PLAN 的 machine Execution Spec 只能调用显式 allowlist Runtime operations，`shell.exec`/unknown operation 在执行前被拒绝。— unit tests verify forbidden operation never reaches executor
- [x] **GSD-03**: review pass 后 STATE 只推进到仓库里预先存在且身份明确的下一 Plan；review fail、缺失/歧义、无 edit capability 都不伪推进。— same-phase/next-phase/blocked/skipped/capability tests + cross-process STATE advance acceptance

### Validated — v0.4 Phase 18

- [x] **PAR-01**: parent Agent Run 可在 2–8 个 managed Git worktree 上运行真实并发 lane，且 derived Runtime 复用 base EffectiveConfig 但不持久化 Workspace。— concurrency barrier + derived-runtime registry isolation tests passed
- [x] **PAR-02**: parent review 聚合 lane verifier pass/fail，同时 infrastructure error 与 review fail 分离，并保留 lane plan/steps/review/cleanup audit。— parallel executor unit tests and process-level CLI acceptance passed
- [x] **PAR-03**: lane roots 相互独立且不切换 main checkout；默认清理 managed worktree，`--keep-worktrees` 显式保留。— real Git acceptance verified two distinct worktree paths, removed both, and preserved main HEAD/branch

## Out of Scope — v0.3

- **AGENT-01** Agent / Subagent Registry 与 orchestration — 顺延到 v0.4；先建立它所依赖的 persistent ownership boundary。
- 桌面 UI / 托盘 — `codexprov4` 当前继续独立使用；v0.3 只交付 local daemon/control surface。
- 完整 DevSpace / CodexPro runtime 兼容层 — 先用 Native Runtime 验证 Control Plane 产品边界。
- Docker 结构化 API、通用 Process Manager、Debugger / DAP — 延后到后续 milestone；v0.3 只允许为 daemon/runtime/MCP ownership 实现最小 child-process cleanup primitive。
- 公网 HTTPS / tunnel / auth — v0.3 继续 loopback-only。
- 多机远程 Runtime — 后续阶段。
- 云端账号、团队协作、计费 — 当前是本地个人开发工具。
- 一开始支持所有 OS — 架构尽量不锁死 Windows，但 v0.x 优先保证当前 Windows 开发环境。

## Architecture Principles

1. **Control Plane / Runtime 分离** — 配置、注册表和生命周期属于 Control Plane；文件、Shell、Git、Test 等属于 Runtime。
2. **EffectiveConfig 是唯一输入** — Runtime 不负责猜配置继承；Config Resolver 输出完整有效配置。
3. **Capability-driven** — UI / Agent 不假设某 Runtime 一定支持 Docker、Worktree 或 Debug；先查询 capability。
4. **Policy enforced below tools** — 安全策略由执行引擎真正校验，不能只写在 MCP tool 描述里。
5. **Global by default, private when needed** — 常用 MCP / Skill 一次配置，全 Workspace 继承；项目只写差异。
6. **Explicit disable is first-class** — 项目可以关闭继承来的 Skill/MCP。
7. **Traceability** — 重要配置、执行、验证都能追踪来源和结果。
8. **Adapter over fork** — 对 CodexPro / DevSpace / Codex / Claude 等优先做适配，不把核心绑定到任一 upstream。
9. **GSD planning is part of engineering** — 需求、决策、阶段退出标准写进仓库，不只存在于聊天记录。
10. **Small vertical slices** — 每个 Phase 都必须形成可以测试的闭环，不按“先把所有抽象写完”开发。
11. **Environment first** — 同一 Project 的多任务隔离优先通过独立 Environment / managed worktree 解决，不让多个任务共享一个 dirty checkout。
12. **One MCP later, many Environments** — 面向 Agent 的长期目标是只配置一个 ADM MCP Gateway，由 `environment_id` 路由到对应 Runtime；不要求每个任务手工维护一个 MCP 配置。
13. **Facts and guardrails, not agent thinking** — ADM 返回事实、warnings/hints 并执行安全策略；是否 rebase/merge/commit/继续旧任务等开发决策属于 Agent/用户。
14. **Human review before integration** — Environment 完成不等于自动 merge；默认保留 review、commit message 调整和最终合并给人/明确调用者。

## Initial Technical Direction

当前建议：

- Language: Go
- Core: 纯 Go package，尽量不依赖 UI 框架
- Config: 先定义领域模型与 resolver；序列化格式由 Phase 1 落地时验证后确定
- Runtime: interface + Native Runtime
- Protocol: MCP 作为 Adapter / Host 层，而不是 Core 本身
- Tests: Go unit tests 为每个领域闭环建立退出标准

Go 的原因：单文件部署友好、并发和进程管理适合本地 runtime、Git/Docker/子进程交互方便、Windows 支持好，并且未来 `codexprov4`（Go/Wails）接入成本低。

## Key Decisions

| Decision | Rationale | Outcome |
|---|---|---|
| 新建独立项目，不在 codexprov4 上继续堆 Core | codexprov4 仍在独立实现且当前需要使用；避免互相干扰 | Accepted |
| 不 fork CodexPro / DevSpace | 产品目标已经超出两者原始边界；fork 会增加 upstream 合并负担 | Accepted |
| CodexPro / DevSpace 作为设计参考和未来 Adapter | 可以吸收优点，又不把 Core 锁死 | Accepted |
| 配置采用四层继承模型 | 直接覆盖“全局共享 + workspace 隔离 + 项目私有”的核心需求 | Accepted |
| 第一阶段不做 UI | 先稳定领域模型和可测试 Core，避免 UI 推动错误抽象 | Accepted |
| GSD 的 Skill 安装与 `.planning/` 项目状态分离 | 全局能力可共享，项目计划保持私有 | Accepted |
| `go.mod` 以 Go 1.22 作为当前最低 baseline，验证使用本机 Go 1.26.5 | Phase 1 不需要新版本特性；保持 Core 最低工具链要求克制，同时记录真实验证环境 | Accepted |
| Phase 1 SourceTrace 采用“实体主来源 + Enabled 来源”，暂不做逐字段 trace | 当前测试已能解释继承/覆盖/禁用链；逐字段 trace 留到真实 UI/诊断需求证明必要时再升级 | Accepted |
| v0.1 持久化使用 JSON；用户配置根目录为 `os.UserConfigDir()/ai-dev-manager`，项目差异为 `<workspace>/.ai-dev-manager/config.json` | 标准库即可完成、可读 Git diff，且 Project 配置可以随项目共享 | Accepted |
| Store / Workspace Registry / ConfigService 分责 | Store 只做 IO/schema，Registry 只做 Workspace CRUD，Service 只编排层并复用 resolver，避免 Runtime/MCP 逻辑污染持久层 | Accepted |
| Runtime Override 永不持久化 | Runtime 临时状态/选择不能污染 Global/Profile/Project 的持久配置 | Accepted |
| Workspace ID 使用 `ws_` + 16-byte crypto/rand hex；Windows 路径按大小写不敏感语义去重 | ID 稳定且无第三方依赖；防止同一 Windows 目录被重复注册 | Accepted |
| `AGENT-01` 从 Phase 3 延后到 v0.3 Agents / GSD Runtime | 当前没有真实 agent orchestration vertical slice；提前建立 Agent 空接口违反 small vertical slice 原则 | Accepted |
| Skill discovery 仅扫描显式 `SkillRoots`；后 root 覆盖前 root，同层显式 Skill 最终覆盖 discovery | 避免任意全盘扫描，并让共享能力与项目差异的优先级可预测 | Accepted |
| Skill ID v0.1 使用 `SKILL.md` 父目录名，不解析 frontmatter | 足够验证共享/私有/覆盖闭环，同时避免为未证明需求引入 YAML 依赖 | Accepted |
| MCP 敏感环境只持久化 `EnvRefs` 引用名，不在 Phase 3 解析宿主环境值 | 配置可共享且不把实际凭据复制进 JSON、trace 或 catalog | Accepted |
| MCP Catalog 的 health 初始固定 `unprobed`；`Enabled=nil` 不在 Catalog 阶段默认解释为启用 | Phase 3 没有真实 runtime/probe，不伪造连通性或运行语义 | Accepted |
| Native Runtime 使用 `Executable + Args` 结构化执行，不自动包 raw shell | 让 allowlist、参数边界、timeout 和日志语义可验证，避免 shell metacharacter 绕过策略 | Accepted |
| Workspace containment 使用 `filepath.Rel` + `EvalSymlinks`，写入还验证最近存在 parent | 同时防住 `..`、绝对路径、目录前缀和 symlink escape | Accepted |
| `.git` 与 `.ai-dev-manager/runtime` 禁止 Runtime 直接写入 | Git/runtime state 由后续结构化能力管理，避免普通文件工具破坏元数据 | Accepted |
| Policy mode 固定为 read-only / workspace-write / standard / full；缺省为 read-only | 默认最小权限，权限升级必须来自 EffectiveConfig | Accepted |
| `ToolPaths` 优先于 PATH lookup | 解决已观察到的 Windows host PATH 与 MCP bash PATH 不一致，并允许项目显式选择工具链 | Accepted |
| 命令执行有默认/硬 timeout 与 stdout/stderr capture 上限 | 防止失控进程和无限输出占用资源 | Accepted |
| Git structured API 必须复用 Native.Exec，而不是绕过 policy/tool resolution | Git 与普通命令共享同一安全边界和 ToolPaths 解析 | Accepted |
| Worktree 只允许创建在 `<workspace-parent>/.ai-dev-manager-worktrees/<workspace-id>/` managed root | 提供并行 checkout，同时不开放任意外部路径写权限 | Accepted |
| Verifier 是四层 EffectiveConfig 的一等配置，并复用 Native.Exec | test/lint/build/custom 自动继承 cwd containment、allowlist、ToolPaths、timeout/output 限制 | Accepted |
| v0.1 不实现 arbitrary unified patch parser | Exact Edit + GitDiff 已验证完整修改/审查/验证闭环；等真实需求证明后再增加 parser | Accepted |
| MCP adapter 使用官方 Go SDK v1.7.0，协议目标为 2026-07-28 | 避免自行维护协议细节；当前官方 Tier-1 SDK 已覆盖目标协议与本地/HTTP transport | Accepted |
| MCP SDK v1.7.0 将项目最低 Go baseline 从 1.22 提升到 1.25.0 | 这是 Phase 6 adapter 依赖带来的真实工具链要求，不是 Phase 1–5 Core 本身需要的新语言特性 | Accepted |
| Streamable HTTP 使用 stateless handler；一个 HTTP instance 固定绑定一个 Runtime Adapter/Workspace | 对齐当前协议模型，并把 Workspace 隔离放在 server instance 边界而不是请求参数 | Accepted |
| v0.1 HTTP Host 默认且仅允许 loopback listen | 防止本地开发 Runtime 被无意暴露；v0.3.1 仅在用户显式 Docker/custom listen 时增加受控 exposure path，默认行为不变 | Accepted |
| MCP Host 只依赖协议无关 Runtime Adapter interface，不依赖 Native/Config Store/Workspace Registry | CodexPro、DevSpace、其他外部 runtime 可以通过 adapter 接入而不重写 Core | Accepted |
| 未知 external capability 只通过 runtime_info 展示，不自动生成可执行 MCP tool | 防止 capability 名称自动变成未经设计/审核的远程执行入口 | Accepted |
| v0.2 先做 Usable Control Plane，而不是优先 Docker / Process / Debug | v0.1 已有强 Core 但没有产品入口；先验证 Workspace → EffectiveConfig → Runtime → MCP 的真实日常闭环 | Accepted |
| CLI 是 Control Plane 的薄客户端，不承载 resolver/runtime 业务逻辑 | 未来 Desktop/codexprov4/API 都应复用同一 application service，避免 CLI 成为第二套架构 | Accepted |
| v0.2 先提供 foreground MCP serve，不引入后台 daemon | 跨进程 start/stop 会强迫提前解决守护进程、IPC、持久状态；当前 vertical slice 只需要真实可连接 endpoint | Accepted |
| Configured MCP activation 必须显式解析 Enabled/EnvRefs 并报告真实 health | 配置存在不等于 runtime healthy；secret 只应在启动边界解析，不能写回配置或普通状态 | Accepted |
| v0.3 优先 Persistent Control Plane / Runtime Lifecycle，Agents/GSD 顺延到 v0.4 | v0.2 已证明 Host/MCP session 与 CLI process lifetime 绑定；若先做 Agent orchestration，会把 run ownership 错绑在前台进程并在 daemon 化时返工 | Accepted |
| v0.3 daemon 只建立 local ownership，不提前实现通用 Process Manager/Docker/Agent | 当前真实 vertical slice 是跨进程 Control Plane 生命周期；保持 small vertical slices，避免 daemon milestone 膨胀 | Accepted |
| Runtime lifecycle 使用 desired state / observed state，而不是持久化内存 runtime/session object | listener、ClientSession、Go pointer 无法跨进程恢复；restart 应通过 reconcile 重建真实 observed state | Accepted |
| v0.3 desired runtime state 只持久化 sorted Workspace ID 集合 | 只保存用户运行意图；daemon identity、endpoint、MCP session 和 observed health 都必须由新进程重新建立 | Accepted |
| daemon stop 保留 desired state，显式 runtime stop 才移除 desired Workspace | clean restart 应恢复持续运行的 Workspace；用户显式停止则必须跨 restart 保持 stopped | Accepted |
| crash recovery 通过 health probe + heartbeat lease stale reclaim 建立新 owner | crash 后旧 metadata/endpoint 不可信；reclaim 必须有 bounded wait，且旧 owner heartbeat 不得 touch 新 owner lease | Accepted |
| Phase 15 Agent Run 是 daemon observed state，不做 desired-state persistence/resume | running goroutine/context/executor 不能跨 daemon process 恢复；后续 GSD 恢复应依赖 `.planning/` checkpoint 而不是伪恢复旧执行对象 | Accepted |
| Phase 15 production executor 明确命名为 `lifecycle`，只验证 ownership/status/cancel | 在真实 Planner/Executor backend 尚未设计前不伪装 AI 能力；Executor contract 已被 production/tests 真实使用，Phase 16 可在同一边界接入 | Accepted |
| v0.5 从 Docker/Process/Debug 改为 Multi-Task Development Environments | 当前最强真实痛点是同一工程多任务并行导致 dirty checkout 混杂；先把 Environment/worktree 隔离产品化，再扩展环境工具 | Accepted |
| ADM 不内置新的 AI Agent 产品 | ChatGPT/Codex/Claude Code/OpenCode/WorkBuddy/CodexPro 已成熟；ADM 的职责是给这些 Agent 提供可靠开发环境 | Accepted |
| Environment 创建默认取当前 checkout HEAD，允许显式 base | 日常通常从 dev，紧急修复可能从 master；不把某个分支写死在 Core | Accepted |
| `include_changes=true` 复制 staged/unstaged/untracked，但排除 ignored | 解决真实的“当前改动需要带入隔离任务”场景，同时避免复制 node_modules、cache、secret/local runtime 文件等 ignored 内容 | Accepted |
| Environment 默认不自动 merge/commit/rebase | 最终 review、commit message 与冲突处理需要人/Agent 显式决策；ADM 只提供事实、能力和安全执行 | Accepted |
| 一个 Environment 默认一个 writer | 解决任务隔离的主要问题，避免在单一 worktree 内重新引入多 Agent 并发写冲突 | Accepted |
| stale Environment 不自动删除；dirty/unpushed destroy 默认拒绝 | 自动清理可能丢失工作；只提供状态/警告，潜在数据丢失必须显式 force | Accepted |
| Agent-facing 长期入口是一个 MCP Gateway，而不是一 Environment 一 MCP 配置 | 用户新增任务只创建 Environment，不应该重新配置 Agent/MCP；由 environment_id 做安全路由 | Accepted |
| codexpro-plus 稳定版不承载 ADM 实验改造 | 它已作为 CodexPro 进程 Manager 发布并验证；未来 UI/Manager 集成如需要先 fork/独立实验，不影响原功能 | Accepted |

## Evolution Rules

每个 Phase 结束时：

1. 对照 Exit Criteria 验证，不满足则不标记完成。
2. 完成的 Active requirement 移入 Validated，并标明 Phase。
3. 新发现需求加入 Active，不偷偷塞进当前 Phase。
4. 被否决或延期的需求移入 Out of Scope，并记录原因。
5. 重要架构决策更新到 Key Decisions。
6. 更新 `.planning/STATE.md` 当前状态和下一步。
7. 若范围发生变化，先调整 ROADMAP，再开始下一个 Phase。

---
*Created: 2026-09-04*