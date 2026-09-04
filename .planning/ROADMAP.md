# Roadmap — ai-dev-manager

## Milestone v0.1 — Core Foundation ✅ Completed 2026-09-04

**Goal:** 先建立稳定、可测试的核心，不做桌面 UI；完成配置继承、Workspace、MCP/Skill Registry、Native Runtime、安全执行和验证的最小闭环。

## Phase 1 — Core Domain & EffectiveConfig ✅ Completed 2026-09-04

**Goal:** 建立核心领域模型和四层配置继承引擎。

**Requirements:** CONF-01..07, WORK-01, RUN-01, COMPAT-01

**Scope:**
- 建立 Go module 与核心 package 结构。
- 定义 Workspace、Profile、RuntimeRef、MCPDefinition、SkillSource、Policy、Verifier 等最小领域模型。
- 定义 ConfigLayer / EffectiveConfig / SourceTrace。
- 实现 `Global → Profile → Project → Runtime Override` merge。
- 实现 enable / disable / override 语义。
- 设计稳定 ID 和冲突规则。
- 配置 resolver 不直接依赖文件系统格式，先从内存模型验证规则。
- 建立 table-driven unit tests。

**Exit criteria:**
1. 同一配置项可明确证明优先级为 Runtime > Project > Profile > Global。
2. MCP / Skill 都能被项目显式禁用继承项。
3. 新增项目私有项不会污染 Global。
4. EffectiveConfig 能返回每个关键项的来源 trace。
5. 核心 package 不依赖 Wails、桌面 UI 或具体 Runtime。
6. `go test ./...` 通过。

---

## Phase 2 — Config Store & Workspace Registry ✅ Completed 2026-09-04

**Goal:** 把 Phase 1 的内存模型落到真实本地配置，并支持多个 Workspace 注册。

**Requirements:** WORK-01, CONF-01..04 persistence, SKILL-01/02 persistence representation

> Planning correction: `WORK-02` 是 Runtime 越界隔离，Phase 2 没有 Runtime，不应在此验证；继续由 Phase 4 负责。

**Scope:**
- 定义 Global / Profile / Project 配置的持久化位置和 schema version。
- 原子读写与损坏配置保护。
- Workspace Registry：稳定 ID、path、profile、runtime、policy。
- Windows 路径规范化与越界检查基础能力。
- 支持全局 Skill 路径与项目 Skill 路径。
- 输出 resolved workspace snapshot，便于后续 Runtime 启动。

**Exit criteria:**
1. 注册两个 Workspace 后重启仍能恢复。
2. 两个 Workspace 可引用同一个 Global Skill/MCP 定义，但 Project override 相互独立。
3. 项目配置损坏不会覆盖 Global 配置。
4. 路径重复/非法配置会返回结构化错误。
5. 配置 migration/version 有最小测试覆盖。

---

## Phase 3 — MCP & Skill Registry ✅ Completed 2026-09-04

**Goal:** 建立真正的共享能力注册表和项目级差异解析。

**Requirements:** MCP-01..05, SKILL-01..04

**Scope:**
- MCP Registry：global/profile/project scopes。
- MCP transport / command / url / env / enable / disable 模型。
- Secret/env 引用只保存引用，不在普通日志中展开敏感值。
- Skill discovery：explicit + configured discovery paths。
- Skill ID 冲突与 project override 规则。
- GSD Skill 与项目 `.planning/` 状态分离。

> `AGENT-01` 延后到 v0.3 Agents / GSD Runtime：在没有真实 agent orchestration vertical slice 前不创建空 Agent 抽象。

**Exit criteria:**
1. 一个 Global MCP 可被两个 Workspace 继承。
2. Workspace A 可禁用该 MCP，Workspace B 不受影响。
3. Workspace A 可增加私有 MCP，B 看不到。
4. Global GSD Skill 无需复制即可出现在多个 Workspace 的 EffectiveSkills 中。
5. Project Skill 可以覆盖同 ID Global Skill，并能追踪来源。

---

## Phase 4 — Native Runtime & Security Policy ✅ Completed 2026-09-04

**Goal:** 建立第一个可运行的 Native Runtime，真正执行文件、搜索、编辑和安全 Shell。

**Requirements:** WORK-02, RUN-02, RUN-03, EXEC-01, EXEC-02 (Shell portion), EXEC-04, SEC-01

**Scope:**
- Runtime interface 与 capability model。
- Native Runtime：tree/read/search/write/edit 等基础文件能力；unified apply-patch 延后到 Phase 5 与 Git/diff 一起决策。
- Workspace containment 和 blocked paths。
- Structured executable+argv executor 与 command policy。
- ReadOnly / WorkspaceWrite / Standard / Full 的第一版执行策略。
- 多 Workspace 独立 Runtime context。
- 结构化 execution result、日志与错误分类。

**Exit criteria:**
1. Runtime A 无法读写 Workspace B 路径。
2. ReadOnly 下写入和危险执行被执行层拒绝。
3. WorkspaceWrite 可以修改项目内文件但不能越界。
4. 两个 Workspace 可并行创建独立 Runtime context。
5. 文件修改与命令执行均产生结构化结果。

---

## Phase 5 — Git & Verification ✅ Completed 2026-09-04

**Goal:** 让 Agent 可以安全地修改代码后自行验证，而不是只“写文件”。

**Requirements:** EXEC-02, VERIFY-01..03, WORK-03

**Scope:**
- Git status/diff/branch 基础结构化能力。
- Git worktree list/create/remove 的第一版领域接口与安全约束。
- Project Verifier 配置：test/lint/build/custom。
- verifier runner、timeout、cwd、exit code、structured output。
- 修改 → diff → verifier 的闭环测试。

**Exit criteria:**
1. 可以获得结构化 git status/diff。
2. 可以声明并运行至少 test + build verifier。
3. verifier 失败能明确指出哪个 verifier、exit code 和摘要。
4. worktree 创建不会默认修改主 checkout。
5. 一个完整示例工程可以完成“修改 → diff → test/build 验证”。

---

## Phase 6 — MCP Host / External Runtime Adapter ✅ Completed 2026-09-04

**Goal:** 让 Core 能真正接入 ChatGPT 和外部 Runtime，同时保持协议与核心解耦。

**Requirements:** RUN-03, COMPAT-02..04

**Scope:**
- MCP Host/Server adapter，把 Native Runtime capabilities 暴露给 MCP client。
- Runtime lifecycle manager：start/stop/status。
- External MCP Runtime adapter 的通用接口。
- CodexPro v4 compatibility contract 草案。
- DevSpace adapter feasibility spike，只验证边界，不要求完整功能覆盖。

**Exit criteria:**
1. ChatGPT/MCP client 能连接一个 Workspace Runtime 并调用基础能力。
2. 两个 Workspace 可启动两个隔离实例。
3. Core 不需要知道调用方是桌面 UI 还是 ChatGPT。
4. External Runtime 能通过 adapter 映射 capability/status。
5. CodexPro v4 接入所需 API 被文档化，不要求此 Phase 修改 codexprov4。

---

## Milestone v0.2 — Usable Control Plane ✅ Completed 2026-09-04

**Goal:** 把 v0.1 Core 组合成真正可启动、可检查、可连接的本地运行时产品入口；CLI 只是第一个 Control Plane client。

## Phase 7 — Control Plane Composition & Introspection ✅ Completed 2026-09-04

**Goal:** 建立 application/control-plane service，以 Workspace ID 作为入口统一组装 EffectiveConfig、Native Runtime、Runtime Adapter 与 MCP Host 生命周期，并提供结构化 introspection。

**Requirements:** CTRL-01..03

**Scope:**
- 新增 `internal/controlplane`，组合 Store / Workspace Registry / ConfigService / Native Runtime / Runtime Adapter / Host Manager。
- 支持按 Workspace ID resolve + build runtime，不让 CLI/MCP client 手工拼装底层组件。
- 提供 workspace/runtime introspection snapshot，暴露 identity、policy、capabilities、effective MCP/Skill/Verifier 与 Source/EnabledSource。
- 提供本进程内 StartMCP/Get/List/StopMCP 生命周期；继续 loopback-only。
- Native 以外的 runtime selection 返回结构化 unsupported error，不伪装成 Native。

**Exit criteria:**
1. 从持久化 Workspace ID 能建立可工作的 Native runtime adapter。
2. introspection 能证明 Project/Profile/Global 来源信息仍正确。
3. Control Plane 启动的 MCP endpoint 可被官方 client 连接并读 workspace 文件。
4. 两个 Workspace 通过同一 Control Plane 仍保持隔离。
5. `go test ./...` 与 `go vet ./...` 通过。

---

## Phase 8 — CLI & Foreground MCP Serve ✅ Completed 2026-09-04

**Goal:** 提供第一个真正的 `ai-dev-manager` 可执行入口，让用户无需测试代码即可注册/查看 Workspace 并启动 MCP endpoint。

**Requirements:** CLI-01..03

**Scope:**
- 新增 `cmd/ai-dev-manager`，仅解析输入/格式化输出并调用 Control Plane / Registry。
- 支持 config root 显式指定，便于测试与多环境使用。
- workspace add/list/show。
- inspect/effective runtime snapshot。
- `serve --workspace <id>` foreground MCP；输出 endpoint，处理 Ctrl+C/termination 后优雅 stop。
- JSON 输出作为稳定机器接口；人类可读输出保持最小。

**Exit criteria:**
1. 可以用真实二进制注册一个 temp workspace，再 list/show。
2. inspect 输出包含 runtime capabilities 与 effective config source 信息。
3. `serve` 启动后官方 MCP client 能连接并调用 read。
4. CLI 不直接 import config resolver 或 Native 的内部实现细节来重复 composition。
5. `go test ./...`、`go vet ./...`、`go build ./cmd/ai-dev-manager` 通过。

---

## Phase 9 — Configured MCP Activation & Health ✅ Completed 2026-09-04

**Goal:** 把 Phase 3 的 MCP Catalog 从“配置可解析”推进到“Enabled MCP 可真实 activation/probe，并可查看 health”的运行闭环。

**Requirements:** MCPA-01..03

**Scope:**
- 为 stdio / Streamable HTTP MCP Definition 建立显式 activation contract；只实现当前测试需要的 transport，不扩展任意 shell。
- EnvRefs 在 activation 边界从宿主环境解析；普通状态/错误不输出 secret value。
- disabled inherited MCP 不 activation；Project override/disable 保持 Workspace 隔离。
- Control Plane 提供 configured MCP status/health；CLI 增加检查入口。
- 失败保持结构化状态，不把 `configured` 当作 `healthy`。

**Exit criteria:**
1. 一个 Global configured MCP 能被两个 Workspace 各自 activation/probe。
2. Workspace A disable 后只阻止 A，Workspace B 仍可 healthy。
3. EnvRefs 缺失返回明确错误且不泄露其他环境值。
4. 至少一个真实 local stdio MCP fixture 完成 initialize/list-tools 或等价健康探测。
5. v0.2 全量 `go test ./...`、`go vet ./...` 通过，并完成 CLI → Workspace → Runtime → MCP endpoint → read/verify 的最终闭环。

---

## Later Milestones

### v0.3 — Agents / GSD Runtime
- Agent / subagent registry。
- GSD phase executor / verifier integration。
- Planner / Executor / Reviewer workflow。
- 多 worktree 多会话隔离。

### v0.4 — Docker / Process / Debug
- Docker ps / compose / logs / inspect。
- Process start/stop/status/logs。
- 开发服务器启动、日志读取、接口验证闭环。
- Debug / DAP feasibility。

### v0.5 — Compatibility
- DevSpace adapter 完善。
- CodexPro / codexprov4 adapter。
- Codex CLI / Claude Code / OpenCode executor adapters。

### v0.6 — UI / Remote Access
- 独立 Desktop UI 是否需要，在此时再决定。
- codexprov4 UI integration。
- HTTPS / tunnel / auth / remote lifecycle。

## Requirement Traceability

| Requirement Group | Primary Phase |
|---|---|
| CONF-* | 1-2 |
| WORK-01 | 1-2 |
| WORK-02 | 4 |
| WORK-03 | 5 |
| MCP-* | 3 |
| SKILL-* | 2-3 |
| AGENT-01 | v0.3 Agents / GSD Runtime |
| RUN-01 | 1 |
| RUN-02..03 | 4, 6 |
| EXEC-* | 4-5 |
| SEC-01 | 4 |
| VERIFY-* | 5 |
| COMPAT-* | 1, 6 |
| CTRL-* | 7 |
| CLI-* | 8 |
| MCPA-* | 9 |

## GSD Development Rules

- 当前 Phase 只实现当前 Phase 的退出标准；新想法进入 Active / Later，不插队。
- 每个 Phase 开工前创建 `CONTEXT.md` 和至少一个可执行 `PLAN.md`。
- Plan 必须拆成可独立验证的小任务，并明确测试/验证命令。
- Phase 完成必须更新 STATE、Requirement 状态和关键决策。
- 任何“先做了再说”的跨层抽象都必须先回答：它是否服务当前 vertical slice？
