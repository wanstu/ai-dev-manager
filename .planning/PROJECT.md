# ai-dev-manager

## What This Is

一个独立于 CodexPro / DevSpace 的本地 AI Coding Core。它负责统一管理 Workspace、Runtime、MCP、Skills、Agents、执行能力与验证流程；上层可以是桌面 UI、CLI、ChatGPT MCP、CodexPro v4 或其他客户端，下层可以是 Native Runtime、DevSpace、Codex CLI、Claude Code、OpenCode 或其他外部 Runtime。

## Core Value

**让多个 AI Coding Workspace 在隔离运行的同时，可靠继承共享能力，并允许项目级私有扩展；所有执行都可控、可验证、可追踪。**

## Product Positioning

不是另一个“文件读写 MCP Server”，而是：

> Local AI Development Workspace & Runtime Core

它解决的是本地 AI 开发环境的控制面与执行面统一问题。

## Relationship With Existing Projects

- `codexprov4`：继续独立开发和日常使用；本项目不修改、不依赖它。未来可作为本项目的 Desktop / Control Plane 客户端或 Runtime Adapter。
- CodexPro：借鉴 workspace containment、safe execution、structured file/search/edit、handoff / `.ai-bridge` 等优点。
- DevSpace：借鉴 worktree、runtime、skills、agents、execution contract 等成熟思路。
- 本项目不 fork CodexPro 或 DevSpace，不把两边代码直接揉在一起；优先借鉴设计，并通过 Adapter / Capability 接口保持兼容空间。

## Current Milestone: v0.1 Core Foundation

**Goal:** 建立可测试的核心领域模型和配置继承引擎，形成第一条“Global → Profile → Project → Runtime Override → EffectiveConfig”的闭环，为后续 MCP、Skill、Runtime、Execution 提供稳定基础。

## Requirements

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

### Active

#### Configuration

(Core resolver + v1 JSON persistence are validated; future configuration extensions are tracked by the capability phases that need them.)

#### Workspace

(Workspace registration, isolation and managed worktree requirements are validated through Phase 5.)

#### MCP / Skills

(MCP Catalog and Skill discovery requirements are validated through Phase 3.)

#### Agents

(Agent/Subagent orchestration is deferred beyond v0.1; see Out of Scope.)

#### Runtime & Execution

(Runtime capability, multi-instance hosting and future extension interfaces are validated through Phase 6.)

#### Verification

(Structured verifier declaration, execution and traceability are validated through Phase 5.)

#### Compatibility

(Runtime Adapter compatibility boundaries are validated/documented through Phase 6.)

## Out of Scope — v0.1

- **AGENT-01** Agent / Subagent Registry 与 orchestration — 延后到 v0.3 Agents / GSD Runtime，不在 v0.1 创建空抽象。
- 桌面 UI / 托盘 — `codexprov4` 当前负责用户日常使用；新 Core 先不做 UI。
- 完整 DevSpace 兼容层 — 先稳定自身接口，避免被外部实现反向绑架架构。
- Docker 结构化 API — v0.1 只预留 capability，后续再实现。
- Debugger / DAP 深度集成 — 后续阶段。
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
| v0.1 HTTP Host 默认且仅允许 loopback listen | 防止本地开发 Runtime 被无意暴露；公网 HTTPS/tunnel/auth 后续由 Control Plane 显式管理 | Accepted |
| MCP Host 只依赖协议无关 Runtime Adapter interface，不依赖 Native/Config Store/Workspace Registry | CodexPro、DevSpace、其他外部 runtime 可以通过 adapter 接入而不重写 Core | Accepted |
| 未知 external capability 只通过 runtime_info 展示，不自动生成可执行 MCP tool | 防止 capability 名称自动变成未经设计/审核的远程执行入口 | Accepted |

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