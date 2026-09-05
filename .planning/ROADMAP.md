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

## Milestone v0.3 — Persistent Control Plane / Runtime Lifecycle ✅ Completed 2026-09-04

**Goal:** 给 v0.2 已验证的 Control Plane 一个稳定、唯一、长期存在的本地 ownership boundary，使 CLI 退出后 Workspace Runtime / MCP 生命周期仍可被同一个本地服务管理，并为后续 Agent/GSD orchestration 提供正确的跨进程生命周期基础。

## Phase 10 — Local Daemon & Control API ✅ Completed 2026-09-04

**Goal:** 建立最小本地 daemon 与跨进程 Control API，让多个独立 CLI invocation 操作同一个长期 Control Plane 实例。

**Requirements:** LIFE-01, LIFE-02, LIFE-03

**Scope:**
- `ai-dev-manager start/status/stop` 最小 daemon lifecycle。
- daemon 进程拥有一个长期 `controlplane.Service`。
- local-only control endpoint；继续禁止非 loopback 暴露。
- 最小 daemon discovery/state metadata，只保存可恢复的标识和 endpoint，不序列化 Go runtime/session object。
- CLI lifecycle 命令作为 thin client，不复制 Control Plane 业务逻辑。
- graceful stop 与 stale daemon metadata 清理。

**Exit criteria:**
1. 一个 CLI 启动 daemon 后退出，daemon 仍存活。
2. 第二个独立 CLI 可以 `status` 并证明连接到同一 daemon instance。
3. 第三个独立 CLI 可以 `stop`，daemon 优雅退出并清理 discovery metadata。
4. 同一 config root 不允许两个健康 daemon 同时成为 owner。
5. control endpoint 只监听 loopback，状态输出不泄露 secret。
6. `go test ./...`、`go vet ./...` 与真实 binary lifecycle acceptance 通过。

---

## Phase 11 — Persistent Workspace Runtime Ownership ✅ Completed 2026-09-04

**Goal:** 把 v0.2 进程内 Host/MCP activation 生命周期迁移到 daemon ownership，使 Workspace Runtime 与 configured MCP session 可以跨 CLI invocation 保持运行。

**Requirements:** LIFE-04, LIFE-05, LIFE-06

**Scope:**
- daemon-owned Workspace runtime instance registry。
- desired state / observed state 分离。
- runtime start/status/stop 控制面。
- MCP Host 与 configured MCP Activator 由 daemon 中的长期 Control Plane 持有。
- Workspace A/B 独立生命周期与失败隔离。
- owned child-process cleanup primitive 仅服务 daemon/runtime/MCP，不扩展成通用 Process Manager 产品能力。

**Exit criteria:**
1. `runtime start --workspace A` 后启动命令退出，A 仍为 running。
2. 新 CLI invocation 能查询同一个 observed runtime/MCP 状态。
3. Workspace A/B 可同时运行且 stop A 不影响 B。
4. daemon stop 会关闭 MCP sessions、HTTP hosts 与 owned children。
5. 不持久化 ClientSession、listener、Go pointer 等不可恢复对象。

---

## Phase 12 — Restart Reconciliation & Lifecycle Acceptance ✅ Completed 2026-09-04

**Goal:** 验证 daemon 崩溃/重启后的 desired-state reconciliation，并完成 v0.3 跨进程生命周期闭环。

**Requirements:** LIFE-07, LIFE-08

**Scope:**
- 持久化最小 desired runtime state。
- daemon startup reconciliation。
- stale observed state / dead process metadata 修复。
- clean restart 与 crash-restart acceptance。
- 多 Workspace + MCP endpoint 的真实 binary acceptance。

**Exit criteria:**
1. daemon restart 后能从 desired state 重建应该运行的 Workspace runtime/MCP，而不是尝试恢复旧内存对象。
2. crash 后 stale metadata 不会永久阻止下一次 daemon 启动。
3. restart 后 runtime identity/status 可解释，旧 endpoint/session 不被误报 healthy。
4. 两个 Workspace 的 desired/observed state 仍隔离。
5. v0.3 全量测试、vet、build 与 Windows binary acceptance 通过。

---

## Milestone v0.3.1 — Usability Acceptance ✅ Completed 2026-09-04

**Goal:** 把 v0.3 已验证的 persistent runtime 从底层验收接口提升为可日常使用的本地工具，同时修复 Docker client 无法连接 loopback-only Workspace MCP Host 的实际验收问题。

## Phase 13 — Docker-reachable Workspace MCP ✅ Completed 2026-09-04

**Goal:** 默认保持 loopback-only，但允许用户显式把某个 Workspace MCP 暴露给 Docker，并在 daemon restart 后保持该 desired bind intent。

**Requirements:** USE-01

**Exit criteria:**
1. 默认 runtime 仍绑定 `127.0.0.1:0`。
2. `runtime start --workspace X --docker` 可绑定可从 Docker host gateway 访问的地址，并返回 `host.docker.internal` client endpoint。
3. daemon Control API 仍严格 loopback-only。
4. desired-state 向后兼容 v1；Docker/custom listen intent 可跨 daemon restart 恢复。
5. tests/vet/build 通过。

---

## Phase 14 — Daily CLI UX ✅ Completed 2026-09-04

**Goal:** 保留底层 workspace/runtime 命令，同时提供不要求用户手工管理 Workspace ID/daemon 顺序的日常命令。

**Requirements:** USE-02

**Scope:**
- `up [path|workspace-id]`：自动注册/复用 Workspace、确保 daemon running、启动 runtime。
- `down [path|workspace-id]`：显式停止 Workspace runtime。
- `ps`：汇总 Workspace + runtime 状态。
- `ctl start|status|stop|restart|shutdown`：集中管理 daemon；`shutdown` 清除 desired runtimes 后停 daemon。
- `up --docker` 直接输出 Docker MCP endpoint。

**Exit criteria:**
1. 新用户对一个未注册目录执行一次 `up` 即可得到 running runtime/MCP endpoint。
2. `down` 不要求记 Workspace ID。
3. `ctl restart` 保留 desired runtime；`ctl shutdown` 不会在下一次 start 时恢复 runtime。
4. 旧 `start/status/stop/workspace/runtime` 命令保持兼容。
5. full tests/vet/build 与真实 CLI acceptance 通过。

---

## Milestone v0.4 — Agents / GSD Runtime ✅ Completed 2026-09-05

**Goal:** 在 persistent daemon ownership 上建立真实 Agent Run 生命周期，再逐步接入 Planner / Executor / Reviewer、GSD phase execution 与 multi-worktree orchestration。

## Phase 15 — Agent Run Lifecycle ✅ Completed 2026-09-05

**Goal:** daemon 长期拥有 Agent Run；CLI 退出后 Run 仍可 list/status/cancel，并明确 daemon restart 不恢复旧 observed Run。

**Requirements:** ARUN-01..03

**Scope:**
- daemon-owned Agent Manager 与稳定 `run_` identity。
- running/completed/cancelled/error 状态机与时间信息。
- loopback Control API：run/list/status/cancel。
- `agent run/list/status/cancel` CLI。
- Phase 15 production `lifecycle` executor，只验证 ownership/cancel，不声称已有 LLM execution。

**Exit criteria:**
1. `agent run .` 的启动 CLI 退出后，独立 CLI 仍能查询同一个 running Run。
2. `agent list/status/cancel` 跨进程工作并保持 Workspace identity。
3. cancel 幂等，两个 Workspace Run 不串状态。
4. daemon stop/restart 后旧 Run 不被误报 running。
5. unit/process acceptance + fmt/test/vet/build 全通过。

---

## Phase 16 — Planner / Executor / Reviewer ✅ Completed 2026-09-05
- 通过 Phase 15 Executor contract 接入第一条真实 `verify` workflow。
- 明确 plan → execute → verify → review 状态与失败回路。
- reviewer fail 与 orchestration error 分离；Run status 保留 plan/steps/review 审计轨迹。

## Phase 17 — GSD Phase Executor ✅ Completed 2026-09-05
- 读取 `.planning/PROJECT.md` / `STATE.md` / `CONTEXT.md` / `PLAN.md`。
- 通过受控 Execution Spec 执行 Runtime operations、verifier/reviewer。
- review=pass 后只推进到预先存在且身份明确的下一 Plan；缺失/歧义时 blocked，不猜测。

## Phase 18 — Parallel Agents / Worktrees ✅ Completed 2026-09-05
- `parallel-verify` parent Run 在 2–8 个 managed Git worktree 上并发执行 verify lanes。
- derived Runtime 复用 base EffectiveConfig，但 root/identity observed-only 且不写 Workspace Registry。
- parent review 聚合 lane pass/fail；默认清理 worktree，`--keep-worktrees` 显式保留；不自动 merge/删除 branch。

## Milestone v0.5 — Multi-Task Development Environments ✅ Completed 2026-09-05

**Goal:** 解决同一工程同时开发多个需求时 dirty checkout 相互污染的问题。把 Phase 18 已验证的 managed worktree + derived Runtime 从临时 lane 提升为持久的一等 Environment；先让 Environment Core 可靠可用，不内置新的 AI Agent，也不以 UI 驱动架构。

### Phase 19 — Persistent Environment Lifecycle ✅ Completed 2026-09-05
- 稳定 `env_` identity 与持久 Environment Registry。
- Environment 绑定 base Workspace、name、base ref/base commit、branch、managed worktree path 与 lifecycle state。
- 默认从创建时当前 checkout HEAD 建立，允许显式 base branch/tag/commit。
- 默认 branch `adm/<sanitized-name>`；已有 branch 明确冲突，不偷偷生成新 branch。
- daemon restart 后重新发现/重建 Environment Runtime context，不持久化内存 Runtime object。
- CLI / Control API 已形成 create/list/inspect/destroy 最小闭环；真实 Git + daemon restart acceptance 通过。

### Phase 20 — Dirty Change Transfer & Safe Destruction ✅ Completed 2026-09-05
- main checkout dirty 时默认从 HEAD 创建并明确报告未带入的修改。
- `include_changes=true` 已验证复制 staged + unstaged + untracked，保留 partially-staged 语义并排除 ignored 文件；binary patch 通过真实 Git acceptance。
- destroy 已验证对 dirty / unpushed work 默认拒绝，显式 force 才允许潜在数据丢失；pushed+clean 可普通 destroy。
- Environment 不自动 commit、merge、squash、cherry-pick、push 或删除 branch；跨进程 daemon restart acceptance 通过。

### Phase 21 — Environment State / Writer Guard / Base Facts ✅ Completed 2026-09-05
- Environment activity/stale 已作为纯状态事实；7 天 inactivity 只提示，不自动清理或释放 writer。
- 持久 single-writer lease 已验证：同 owner renew、第二 owner conflict、显式 release/force release，并跨 daemon restart 保留。
- inspect 已报告 base ahead/behind/divergence/base_moved、dirty、upstream、branch/worktree、activity/writer 等事实。
- 明显 divergence 或 behind>=10 时最多返回一条非指令性确认 hint，不返回强制 rebase/merge 指令。
- base 不自动同步；rebase/merge/push/commit 仍属于 Agent/用户显式开发决策；真实 Git + cross-process CLI + full gate 全部通过。

## Milestone v0.6 — Agent MCP Gateway

**Goal:** 让外部 Agent 只配置一个长期稳定的 ADM MCP Gateway，就能发现 Project/Environment、创建与管理 Environment，并通过 `environment_id` 把开发操作安全路由到对应 validated Runtime；不要求每个任务新增 MCP 配置，也不把 ADM 变成内置 Agent。

### Phase 22 — Gateway Host & Discovery ✅ Completed 2026-09-05
- 独立 Gateway MCP server 已实现，现有 per-Workspace Direct MCP contract 保持不变并通过回归测试。
- daemon-owned Gateway listener 首次可从 loopback `:0` 选择端口并持久化实际地址；daemon restart 后真实 MCP client 复用完全相同 endpoint。
- `gateway up/status/down` lifecycle 已实现；默认 loopback，non-loopback 被拒绝。
- discovery tools `gateway_info` / `workspace_list` / `environment_list` / `environment_inspect` 已通过 typed MCP + cross-process acceptance。
- persisted port 被占用时 Gateway 保持 desired=true/error 且不静默换端口；full fmt/test/vet/build gate 通过。

### Phase 23 — Environment Lifecycle MCP ✅ Completed 2026-09-05
- Gateway 已增加 `environment_create` / `environment_destroy` / writer acquire/release，并通过官方 MCP client 真实调用。
- create 保留 v0.5 base/include-changes/branch 语义；destroy 继续服从 dirty/unpushed/active-writer guard，force 仍需显式请求且 branch 保留。
- domain failure 已通过 `isError=true` + typed structured error code/message/environment_id/facts/warnings/hints 返回，不把开发建议伪装成 required action。
- Gateway 不暴露 raw `git_worktree_create/remove`；writer conflict、daemon restart persistence、unsafe destroy、duplicate branch 等真实 lifecycle acceptance 与 full gate 全部通过。

### Phase 24 — Routed Read Tools ✅ Completed 2026-09-05
- `tree/read/search/git_status/git_diff/git_branch` 已通过显式 `environment_id` 路由到 validated derived Runtime。
- 每次调用重新验证 managed worktree identity并重建 derived Runtime，不信任持久 path；missing worktree真实测试通过。
- Gateway 保持统一 tool surface，在具体 Environment 上二次 capability check；缺能力返回 structured `capability_missing`。
- read-only 调用不需要 writer、不刷新开发 activity；双 Environment隔离与 daemon restart 后同 endpoint routed read均通过，full gate全绿。

### Phase 25 — Writer-safe Mutation & Verification ✅ Completed 2026-09-05
- `write/edit/exec/run_verifier/run_verifiers` 已通过 `environment_id + writer_owner` 路由。
- mutation/active operation 在 Runtime Invoke 前强制校验当前 writer；错误 owner 不触达 Runtime，成功调用才续租 writer `last_seen_at` 并 touch Environment activity。
- Manager 以 RWMutex 保证 mutation 与 writer release/destroy 的原子边界，同时允许不同 Environment 并发 mutation；race tests 已通过。
- 真实双 Environment / 双 writer / daemon restart / MCP client acceptance 已验证同一 Project 多任务并行隔离，structured exec policy 与 verifier pass/fail 语义保持正确。
- 普通 `go test ./...` 已改为无人值守、不启动真实 TCP listener；真实 network acceptance 通过固定 daemon/test executable 路径完整运行并 5/5 PASS。
- v0.6 已完成并停在 milestone boundary；remote exposure、Process/Docker/UI 留给后续 milestone。

### v0.7 — Development Environment Capabilities
- 根据真实使用补 Process / dev server / logs / ports / HTTP verification。
- Docker structured capability 仅在它能改善 Environment 开发体验时加入。
- 远端 Agent 连接开发机器上的 ADM MCP 属于 MCP exposure/auth 问题，不引入无必要的 SSH Remote Runtime。
- Debug / DAP 在真实需求证明后再进入。

### v0.8 — Human Experience / Manager
- 先改善 CLI，再按真实需求决定 TUI/Desktop UI。
- `codexpro-plus` 稳定版保持原 CodexPro 进程管理功能；若验证 ADM Manager，使用 fork/独立实验版本。
- Agent Manager 保留为可选 orchestration capability，不成为前期产品中心。
- CodexPro / DevSpace / 其他 runtime compatibility 继续通过 Adapter 演进。

## Requirement Traceability

| Requirement Group | Primary Phase |
|---|---|
| CONF-* | 1-2 |
| WORK-01 | 1-2 |
| WORK-02 | 4 |
| WORK-03 | 5 |
| MCP-* | 3 |
| SKILL-* | 2-3 |
| AGENT-01 | v0.4 Agents / GSD Runtime |
| RUN-01 | 1 |
| RUN-02..03 | 4, 6 |
| EXEC-* | 4-5 |
| SEC-01 | 4 |
| VERIFY-* | 5 |
| COMPAT-* | 1, 6 |
| CTRL-* | 7 |
| CLI-* | 8 |
| MCPA-* | 9 |
| LIFE-01..03 | 10 |
| LIFE-04..06 | 11 |
| LIFE-07..08 | 12 |
| ENV-01..02, ENV-04..05, ENV-09, UX-01 foundation | 19 |
| ENV-03, ENV-07 | 20 |
| ENV-06, ENV-08, UX-01 completion | 21 |
| GW-01..02, GW-08 | 22 |
| GW-03, GW-06..07 | 23 |
| GW-04 | 24 |
| GW-05 | 25 |

## GSD Development Rules

- 当前 Phase 只实现当前 Phase 的退出标准；新想法进入 Active / Later，不插队。
- 每个 Phase 开工前创建 `CONTEXT.md` 和至少一个可执行 `PLAN.md`。
- Plan 必须拆成可独立验证的小任务，并明确测试/验证命令。
- Phase 完成必须更新 STATE、Requirement 状态和关键决策。
- 任何“先做了再说”的跨层抽象都必须先回答：它是否服务当前 vertical slice？
