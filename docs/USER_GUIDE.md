# ADM v1.1 用户手册

本文面向实际使用 ADM 的人：你可以通过 Desktop 管理 ADM，通过 `adm` CLI 做自动化配置，也可以把 Agent 连接到 ADM Gateway，让 Agent 在明确授权的本地目录中完成真实开发工作。

如果你想了解实现内部为什么这样设计，请继续阅读 [ARCHITECTURE.md](ARCHITECTURE.md)。如果你正在接入 Agent/MCP 客户端，请看 [AGENT_GATEWAY.md](AGENT_GATEWAY.md)。完整 CLI 参数见 [cli.md](cli.md)。

## 1. ADM 是什么

ADM（AI Dev Manager）是本机 AI 开发控制面。它把“Agent 可以访问哪些目录、可以执行哪些程序、可以使用哪些 MCP/Skill、当前有哪些 Runtime 资源”集中到一个可检查、可管理的边界中。

ADM 的核心原则是：

- **Workspace 是显式登记的本地目录**，不是 Git 仓库的别名。
- **Environment 是一个持久开发上下文**，不是服务、容器或 Git 分支。
- **Gateway 才是真正运行的服务进程**。
- **读操作和写操作分权**：读通常不需要 Writer；文件写入、命令执行等 mutation 需要 Writer lease。
- **命令执行必须经过 ADM execution policy**：Strict 默认依赖 allowlist；命令黑名单始终优先，并且 Full Authorization 也不能绕过。
- **MCP 和 Skill 是全局定义，Environment 只保存启用选择**。
- **Git、Verifier、MCP、Skill、Worktree 都是可选能力**；缺一个能力只阻止依赖它的操作。
- **ADM 不负责 Planner/Executor/Reviewer、GSD phase 推进或任务编排**。ADM 提供能力、权限、生命周期和诊断，Agent 自己决定任务流程。

## 2. 三个使用入口

ADM v1.1 有三个主要入口，但它们共用同一套 Core 语义和持久状态。

| 入口 | 用途 | 默认路径/地址 |
|---|---|---|
| `adm` CLI | 管理、脚本化、Gateway 生命周期 | 默认管理 `http://127.0.0.1:43137` |
| `adm-desktop` | 人类可视化管理 | 连接所选 ADM Base URL 的 `/admin/mcp` |
| Agent MCP Gateway | Agent 文件/Runtime/MCP/Skill 能力 | `http://127.0.0.1:43137/mcp` |

HTTP Gateway 同时提供：

- `/mcp`：Agent 面，暴露开发所需能力，不暴露大部分管理写操作。
- `/admin/mcp`：管理面，供 CLI/Desktop 管理 Workspace、Environment、catalog、Memory 等。
- `/healthz`：健康检查。

正常 CLI 的 `workspace` / `environment` / `exec` / `mcp` / `skill` / `memory` 命令都通过 `/admin/mcp`。如果管理目标不可达，CLI 会失败，不会偷偷回退到本地 `state.json`。

`gateway`、`doctor`、`state` 是明确的本机 bootstrap/offline/recovery 命令，因此它们是例外。

## 3. 安装与启动

GitHub Release 提供：

- `adm-v1.2.0-windows-amd64.exe` / `adm-v1.2.0-windows-arm64.exe`
- `adm-v1.2.0-linux-amd64` / `adm-v1.2.0-linux-arm64`
- `adm-v1.2.0-darwin-amd64` / `adm-v1.2.0-darwin-arm64`
- `adm-desktop-v1.2.0-windows-amd64.exe`
- `adm-desktop-v1.2.0-darwin-universal.zip`
- `adm-desktop-v1.2.0-linux-amd64`
- `SHA256SUMS-v1.2.0.txt`

Windows PowerShell 示例：

```powershell
$adm = 'C:\tools\adm\adm-v1.2.0-windows-amd64.exe'
& $adm -h
& $adm doctor
```

启动本机 Gateway：

```powershell
& $adm gateway start --detach
& $adm gateway status
```

默认 Base URL：

```text
http://127.0.0.1:43137
```

可以选择不同的 ADM：

```powershell
& $adm --adm-url http://127.0.0.1:8001 workspace list

$env:ADM_V2_URL = 'http://127.0.0.1:8001'
& $adm workspace list
```

CLI 与 Desktop 都会读取 `~/.config/adm/.env`，也会读取**当前可执行文件同目录**的 `.env`。优先级为：**已有进程环境变量 > 应用目录 `.env` > `~/.config/adm/.env`**。用户级文件适合保存常用连接，应用目录文件适合便携版或单个安装实例覆盖。

远程 ADM 使用双 API Key：服务端 Remote access 分别设置 Admin Key 与 Agent Key；Desktop/CLI 作为客户端访问 `/admin/mcp` 时使用同一把 Admin Key（CLI 优先通过 `ADM_ADMIN_API_KEY` 提供；旧 `ADM_V2_ADMIN_API_KEY` 仅兼容读取），Agent 访问 `/mcp` 时使用 Agent Key。两种面都支持 `Authorization: Bearer <Key>` 或 `X-ADM-API-Key: <Key>`。远程监听还需要 Host/IP 白名单；白名单 `*` 表示不限制 Host，但不会关闭双 Key 鉴权。完整配置见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

> 非 loopback 监听必须先配置 Host allowlist、Admin Key 和 Agent Key。`/admin/mcp` 与 `/mcp` 使用不同 Key，不能互换；如果经公网暴露，仍建议由 HTTPS 反向代理提供 TLS。

## 4. Workspace：ADM 的目录授权边界

### 4.1 Workspace 是什么

Workspace 是一个你明确允许 ADM 使用的现有目录。它可以是：

- Git 仓库；
- 包含多个项目的目录；
- 完全没有 `.git` 的普通目录；
- 空目录。

Git 不是 Workspace 的前置条件。

### 4.2 登记、查看、重命名和移除

```powershell
& $adm workspace add --path D:\projects --name projects
& $adm workspace list
& $adm workspace inspect --workspace-id WS_ID
& $adm workspace rename --workspace-id WS_ID --name projects-main
& $adm workspace remove --workspace-id WS_ID
```

行为边界：

- rename 只改 ADM 元数据，不移动真实目录。
- remove 只删除 ADM 注册记录，不删除目录或文件。
- 只要仍有 Environment 引用 Workspace，remove 就会被拒绝。

### 4.3 大目录项目发现

当一个 Workspace 包含很多项目时，可以做 bounded metadata-only discovery：

```powershell
& $adm workspace discover `
  --workspace-id WS_ID `
  --query project-a `
  --max-depth 4 `
  --max-entries 2000 `
  --max-candidates 50
```

它只读取目录 metadata/项目标记，不读取项目文件正文，也不会自动创建 Environment。预算参数为 `0` 时由 Core 使用默认值。

适合这种目录：

```text
D:\projects
├── p1
├── p2
├── p3
└── archive
```

Agent 或人可以先 discovery，再显式决定应创建/使用哪个 Environment。

## 5. Environment：持久开发上下文

### 5.1 Environment 不是容器或分支

Environment 由稳定 `env_...` ID 标识，核心内容包括：

- 所属 Workspace；
- Environment root；
- 显示名称；
- Writer lease；
- 已启用的 MCP IDs；
- 已启用的 Skill IDs；
- Environment-private Memory；
- Verifier 定义；
- retention/lifecycle 元数据。

它不要求 Git、Docker、Worktree 或 Verifier。

### 5.2 创建持久 Environment

整个 Workspace 作为 root：

```powershell
& $adm environment create --workspace-id WS_ID --name main
```

指定 Workspace 内的子目录：

```powershell
& $adm environment create `
  --workspace-id WS_ID `
  --name project-a `
  --root D:\projects\project-a
```

Environment root 必须是已存在目录，并且必须位于 Workspace 内。

### 5.3 查看 Environment

```powershell
& $adm environment list
& $adm environment inspect --environment-id ENV_ID
```

`list` 是轻量摘要；`inspect` 会补充：

- Workspace 关系；
- capability facts；
- 已解析 MCP/Skill；
- 已删除但仍被 Environment 引用的 unresolved IDs；
- private Memory **条目数**。

它不会把 private Memory 的值直接展开。

### 5.4 Rename / Remove

```powershell
& $adm environment rename --environment-id ENV_ID --name review
& $adm environment remove --environment-id ENV_ID
```

rename 不移动 root、不改 Memory、不改 MCP/Skill 选择、不释放 Writer。

ordinary Environment remove 只删除 ADM 上下文记录，不删除真实目录或文件；active Writer 会阻止 remove。由 ADM managed worktree 支撑的 Environment 必须走 managed-worktree destroy 安全路径，不能绕过。

## 6. Writer lease：为什么 Agent 写文件前要“拿锁”

### 6.1 目的

多个 Environment 可能最终指向同一个 physical root。为了防止两个 Agent 同时写同一目录，ADM 对 physical root 实施单 writer lease。

读操作不要求 Writer；mutation 要求 matching writer owner。

### 6.2 使用

```powershell
& $adm environment writer acquire `
  --environment-id ENV_ID `
  --owner agent-session-123

& $adm environment writer heartbeat `
  --environment-id ENV_ID `
  --owner agent-session-123

& $adm environment writer release `
  --environment-id ENV_ID `
  --owner agent-session-123
```

人工恢复时可以：

```powershell
& $adm environment writer release --environment-id ENV_ID --force
```

`--force` 是恢复工具，不应作为正常工作流。

### 6.3 Lease 工作机制

默认 lease TTL 是 **5 分钟**。

- acquire 获取或同 owner 续租；
- heartbeat 只续租；
- 成功 mutation 会更新活动并续租；
- 长时间受管执行会保持 Writer 活性；
- lease 过期后不再阻塞 physical root；
- 同一 physical root 上的另一个 Environment 也不能绕过这个限制。

因此 Writer 是“有期限的写权限”，不是永久锁。

## 7. 文件能力：Agent 最基础的开发循环

Agent 面提供：

- `tree`：列目录；
- `read`：读文本；
- `search`：literal 文本搜索；
- `write`：写文本；
- `edit`：精确文本替换；
- `delete`：删除一个文件。

`tree` / `read` / `search` 是 read-only，不需要 Writer。

`write` / `edit` / `delete` 必须提供 matching `writer_owner`。

所有路径都受 Environment root containment 约束。Agent 不能通过 `..`、软链接等方式把权限扩展到注册 root 外。

`delete` 不提供递归目录删除；这是刻意的安全边界。

一个典型 Agent 流程：

```text
1. environment_injection_plan(env_id)
2. environment_context_bundle(env_id)
3. tree/read/search
4. environment_writer_acquire(env_id, owner)
4. write/edit/delete
5. exec 或 verifier
6. inspect result
7. 继续 edit/verify
8. environment_writer_release
```

## 8. Exec allowlist：允许执行哪些本机命令

### 8.1 管理 allowlist

```powershell
& $adm exec allow --executable go
& $adm exec allow --executable git
& $adm exec allow --executable node
& $adm exec list
& $adm exec remove --executable git

# 永久优先拒绝某个 executable
& $adm exec blacklist add --executable pwsh
& $adm exec blacklist list
& $adm exec blacklist remove --executable pwsh
```

这不是 shell 字符串白名单，而是 executable 权限边界。Agent 调用 `exec` / process / run / verifier / stdio MCP 时，最终都要经过相关 executable 检查。

Execution policy 的优先级是：**command blacklist → reserved ADM command policy → allowlist / Full Authorization**。加入黑名单时同名 executable 会从 allowlist 移除；移出黑名单不会自动重新加入 allowlist。Full Authorization 允许未列入 allowlist 的 executable，但**永远不能执行黑名单中的 executable**。

### 8.2 Agent 执行命令

Agent 工具 `exec` 需要：

- `environment_id`；
- `writer_owner`；
- `executable`；
- 参数数组；
- 可选 Environment-relative `cwd`；
- 可选 timeout/output budget。

示意：

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-123",
  "executable": "go",
  "args": ["test", "./..."],
  "cwd": "project-a",
  "timeout_ms": 120000
}
```

工作目录必须留在 Environment root 中。默认 Strict 模式下，allowlist 中不存在的 executable 会被明确拒绝；Full Authorization 可放行未列入 allowlist 的 executable，但命令黑名单仍会明确拒绝。**Full 模式下 allowlist 为空不代表不可执行**：Capability Report / Context Bundle 会将 `shell.exec` 标记为 available，并通过 Full bypass 审计记录未列入 allowlist 的执行；如果某个命令仍失败，应继续区分 PATH/可执行文件不存在等运行环境问题。

### 8.3 Exec denial observations

Gateway/Admin 管理面还会记录轻量的 executable authorization observations，包括 Strict allowlist 拒绝、command blacklist 拒绝与 Full Authorization bypass；包含次数、最近时间、来源 Environment 等，但不会持久化命令参数或 stdout/stderr。Desktop 可以用这些观察帮助人决定是否调整 allowlist / blacklist。

## 9. MCP：全局定义、Environment 授权、运行时观察是三件不同的事

### 9.1 MCP 数据模型

ADM 的 MCP 分三层：

1. **Global MCP definition**：连接配置、transport、secret refs、health policy。
2. **Environment selection**：某个 Environment 是否启用该 MCP ID。
3. **Gateway owner-local observation**：当前进程中的连接/健康/工具 inventory/recovery 状态。

不要把“已配置”误解为“该 Environment 已授权”，也不要把“上次 healthy”误解为持久事实。

### 9.2 支持的 transport

v1.1 支持：

- `streamable-http`
- `stdio`

HTTP 示例：

```powershell
& $adm mcp add `
  --name my-http-mcp `
  --transport streamable-http `
  --endpoint http://127.0.0.1:9000/mcp
```

带 header 引用：

```powershell
& $adm mcp add `
  --name private-http `
  --transport streamable-http `
  --endpoint https://example.invalid/mcp `
  --auth-mode headers `
  --header-refs-json '{"Authorization":"${MY_MCP_AUTH}"}'
```

stdio 示例：

```powershell
& $adm exec allow --executable my-mcp-server

& $adm mcp add `
  --name local-helper `
  --transport stdio `
  --executable my-mcp-server `
  --args-json '["serve","--stdio"]' `
  --env-refs-json '{"API_TOKEN":"${MY_MCP_TOKEN}"}'
```

stdio executable 仍然受 ADM executable allowlist 约束。

### 9.3 Secret/reference 机制

MCP catalog 持久化的是引用，例如：

```text
${MY_MCP_TOKEN}
```

而不是 token literal。真正的环境值在 activation 边界解析。正常 list/status/inventory/error 不应该泄露 secret 值。

### 9.4 Default 与 Environment enablement

```powershell
& $adm mcp set-default --id MCP_ID --enabled true
```

只影响**之后新建的 Environment**。不会批量修改现有 Environment。

针对现有 Environment：

```powershell
& $adm environment mcp enable --environment-id ENV_ID --mcp-id MCP_ID
& $adm environment mcp disable --environment-id ENV_ID --mcp-id MCP_ID
```

禁用后，Agent 对这个 Environment 的 MCP tools/call 授权应立即失效。

### 9.5 四种 MCP 诊断不要混用

#### `mcp probe`：全局 transient probe

```powershell
& $adm mcp probe --id MCP_ID
```

用途：验证全局 definition 能否解析/连接，不需要 Environment。

它不会：

- 启用 MCP；
- 改 Environment selection；
- 调用业务 tool；
- 持久化 health；
- 建立自动 reconnect 生命周期。

stdio probe 使用 ADM 临时工作目录，因此不要假定 project-relative cwd。

#### `mcp status`：Environment 一次性状态探测

```powershell
& $adm mcp status --id MCP_ID --environment-id ENV_ID
```

会先应用 Environment selection，再返回 configured / disabled / healthy / error 等状态。

#### `mcp inspect`：被动观察

```powershell
& $adm mcp inspect --id MCP_ID --environment-id ENV_ID
```

读取脱敏 desired config + 当前 Gateway owner 已经拥有的 runtime observation/tool inventory。

**不会连接、Ping、refresh。**

#### `mcp refresh`：主动刷新 owner runtime

```powershell
& $adm mcp refresh --id MCP_ID --environment-id ENV_ID
```

显式丢弃旧 session/observation，重新 connect、Ping、refresh bounded tool inventory。

它不会调用业务工具，也不会修改 catalog 或 Environment selection。

### 9.6 MCP health/reconnect policy

定义可以配置：

- health check enabled；
- check interval；
- probe timeout；
- auto reconnect；
- reconnect interval。

自动 reconnect 默认关闭。开启后采用固定间隔，不做指数退避，也不会自动 replay 一次失败的业务 tool call。

### 9.7 外部配置导入

支持 JSON/JSONC preview/apply：

```powershell
& $adm mcp import-preview --file .\mcp.json
& $adm mcp import-apply --file .\mcp.json
```

也可使用：

```powershell
Get-Content .\mcp.json -Raw | & $adm mcp import-preview --stdin
```

或者显式 inline：

```powershell
& $adm mcp import-preview --json-or-jsonc $content
```

三种 content source **必须且只能选一个**。

支持的格式包括 generic `mcpServers`、OpenCode、WorkBuddy/CodeBuddy、Codex plugin、Claude Code、MCPHub 等。preview 不落 catalog；apply 会重新解析并原子写入选中定义。导入不会修改已有 Environment selection。credential literal 会被转换为引用需求，不持久化原始 secret blob。

## 10. Skill：ADM 管的是真实 Skill artifact，不是复制一段提示词

### 10.1 Skill 模型

Skill 来自显式配置的 source root。ADM 扫描真实 `SKILL.md`，并记录：

- Skill stable ID；
- source；
- artifact path；
- support roots；
- default include；
- availability 状态。

ADM 不把 Skill 解释为内部任务工作流；Agent 显式读取 Skill 后自己遵循它。

### 10.2 推荐的 source 生命周期

登记 source，但不立即 refresh：

```powershell
& $adm skill source-add `
  --root C:\Users\you\.config\opencode\skills `
  --support-root C:\Users\you\.config\opencode\shared `
  --default
```

查看：

```powershell
& $adm skill source-list
```

修改 source 配置：

```powershell
& $adm skill source-update `
  --id SKILL_SOURCE_ID `
  --root C:\skills `
  --support-root C:\skills-support
```

**update 不会自动 refresh。** 这是为了让配置变更和 filesystem discovery 的副作用分离。

显式 refresh：

```powershell
& $adm skill source-refresh --id SKILL_SOURCE_ID
```

refresh 按 source 原子更新该 source 拥有的 Skill snapshot；失败不会把旧的有效 snapshot 半更新成残缺状态。

删除 source：

```powershell
& $adm skill source-remove --id SKILL_SOURCE_ID
```

source-owned Skills 会一起移除，但已有 Environment 中的 ID 引用不会被偷偷重写，会成为可诊断的 unresolved selection。

### 10.3 `skill add` 是便利入口

```powershell
& $adm skill add --root C:\skills --support-root C:\shared --default
```

这是“登记 source + refresh 一次”的便利方式。需要更可控的生命周期时，优先用 `source-add` / `source-refresh`。

### 10.4 Global availability 与 Environment availability

全局结构可用性：

```powershell
& $adm skill availability
```

这里检查 source/artifact/support root 等结构事实，不看 Environment enablement，也不会读 Skill 指令正文。

Environment 视角：

```powershell
& $adm environment skill list --environment-id ENV_ID
& $adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID
```

这里才会出现 enabled / disabled / artifact_missing 等 Environment-specific 状态。

### 10.5 Environment 选择

```powershell
& $adm skill set-default --id SKILL_ID --enabled true

& $adm environment skill enable --environment-id ENV_ID --skill-id SKILL_ID
& $adm environment skill disable --environment-id ENV_ID --skill-id SKILL_ID
```

和 MCP 一样，default 只影响以后创建的 Environment。

### 10.6 Agent 如何读取 Skill

Agent 面提供：

- `environment_skill_list`
- `environment_skill_inspect`
- `environment_skill_files`
- `environment_skill_read`

只有该 Environment 已启用的 Skill 才能读取内容。`environment_skill_read` 只能访问 `SKILL.md` artifact 范围和明确授权的 support roots，不能借 Skill 跳出到任意主机路径。

## 11. Memory：Global 与 Environment-private 明确分域

### 11.1 Global Memory

跨 Environment 共享：

```powershell
& $adm memory global list
& $adm memory global read --key machine
& $adm memory global write --key machine --value windows
& $adm memory global delete --key machine
```

### 11.2 Environment-private Memory

只属于一个 Environment：

```powershell
& $adm memory environment list --environment-id ENV_ID
& $adm memory environment read --environment-id ENV_ID --key task
& $adm memory environment write --environment-id ENV_ID --key task --value 'private context'
& $adm memory environment delete --environment-id ENV_ID --key task
```

ADM 不会自动在两个 scope 之间 copy/promote/merge。

Environment list/inspect/context 默认只暴露安全 count/facts，不自动塞入 private Memory 值。Agent 若需要值，必须显式调用 Memory read。

## 12. Verifier：结构化 test/lint/build/custom 定义

### 12.1 定义 Verifier

```powershell
& $adm exec allow --executable go

& $adm environment verifier add `
  --environment-id ENV_ID `
  --kind test `
  --name 'go test' `
  --executable go `
  --arg test `
  --arg ./... `
  --timeout-seconds 300
```

定义 Verifier **不会自动把 executable 加入 allowlist**。

查看/删除：

```powershell
& $adm environment verifier list --environment-id ENV_ID
& $adm environment verifier remove --environment-id ENV_ID --verifier-id VF_ID
```

### 12.2 同步与异步执行

Agent 面支持同步：

```text
environment_verifier_run
```

调用会等验证结束。适合短 verifier。

长测试/构建优先：

```text
environment_verifier_run_start
environment_verifier_run_list
environment_verifier_run_status
environment_verifier_run_cancel
```

异步 verifier 使用独立的 owner-local `vfrun_...` 身份。它复用同一套：

- verifier definition；
- exec execution policy（allowlist / command blacklist / authorization mode）；
- cwd containment；
- configured timeout；
- Writer 权限；
- structured verifier classification。

`list/status` 是只读；start/cancel 需要 matching Writer。运行观察不会持久化，Gateway restart 后不会恢复旧 `vfrun_`。

## 13. 长运行开发服务：Process

`process_start` 用于 dev server 这类持续运行进程，例如：

```text
npm run dev
python -m http.server
my-api-server serve
```

Agent 可：

- `process_start`
- `process_list`
- `process_status`
- `process_logs`
- `process_stop`

start/stop 需要 matching Writer；list/status/logs 是观察操作。

Process 由当前 persistent Gateway runtime owner 持有，使用 `proc_...` ID，不允许 Agent 拿任意 OS PID 做通用进程管理。

ADM 保留 bounded stdout/stderr tail，并可报告该 owned process 的 listening ports。Gateway shutdown 会清理 active process；restart 后不会恢复旧 process observation。

## 14. 异步单命令：Run

`run_...` 适合“一个可能很久的命令，但不属于 structured verifier”的场景。

工具：

- `run_start`
- `run_list`
- `run_status`
- `run_cancel`

Run 状态区分：

- `running`
- `succeeded`
- `failed`
- `canceled`

它仍然只是**单命令 Runtime resource**，不是 ADM task。它不会做 Planner/Executor/Reviewer、自动重试、自动 Git 合并、phase advance 等编排。

Run 同样是 owner-local observation；Gateway restart 后不会 resume/resurrect。

## 15. Environment capability report 与 context bundle

### 15.1 Capability report

```powershell
& $adm environment capability-report --environment-id ENV_ID
```

返回 canonical capability facts，解释哪些能力 available/unavailable 以及原因，例如：

- files；
- shell/exec；
- Git；
- managed worktree；
- MCP；
- Skill；
- verifier；
- process/run observations。

有 Gateway runtime owner 时，报告可以加入**已有** owner-local observation，但生成报告本身不会主动 reconnect/probe MCP、调用业务 tool 或运行 verifier。

### 15.2 MCP / Skill injection plan

```powershell
& $adm environment injection-plan --environment-id ENV_ID
```

Injection plan 是 Environment 对 Agent 暴露 MCP / Skill 的 canonical passive 决策结果。它只返回**已经选中的**能力，并明确区分：

- `selected`：该能力确实被 Environment 或 Workspace 选择；
- `selection_sources`：`environment`、`workspace`，或两者同时存在；
- `default_include_in_environment`：当前 catalog default flag，仅作为事实，不伪装成历史 selection 来源；
- `injectable`：当前 passive capability 检查是否允许进入下一步 runtime/read；
- `state / reason_code / message`：不可注入时的结构化原因；
- `next_action`：Agent 下一步应调用的显式工具。

MCP 的 ready 项通常返回 `environment_mcp_tools`，Skill 的 ready 项通常返回 `environment_skill_read`；blocked 项分别指向 inspect 工具。

**selection 不等于 injection。** 例如 Workspace 继承了一个 MCP，但 secret reference 未解析，则它仍然是 selected，同时 `injectable=false`。生成 injection plan 不会主动连接 MCP、读取 Skill 正文、获取 Writer 或扩大权限。

### 15.3 Context bundle

```powershell
& $adm environment context --environment-id ENV_ID
```

它把 Agent 启动工作时最常用的安全信息压成一个 bounded bundle：

- stable Environment/Workspace identity；
- root 和 bounded tree digest；
- capability facts；
- MCP 安全摘要及已有 tool-name observation；
- Skill 安全摘要；
- verifier 摘要；
- operation guidance；
- omissions/truncation evidence。

它刻意**不做**：

- 隐式“当前 Environment”；
- 读 Global/private Memory 值；
- 注入完整 Skill instructions；
- 主动连接/probe MCP；
- 执行 Git/verifier/process/run；
- 获取 Writer；
- 扩大任何权限。

可以限制 scope/budget：

```powershell
& $adm environment context `
  --environment-id ENV_ID `
  --path project-a `
  --max-depth 3 `
  --max-entries 500 `
  --max-digest-entries 80 `
  --max-output-bytes 65536
```

## 16. Temporary Environment：短生命周期上下文

Temporary Environment 仍然是普通稳定 `env_...`，只是 retention 为 temporary。ADM 不因此得到任何 task orchestration 权限。

### 16.1 existing_root 模式

```powershell
& $adm environment temporary create `
  --workspace-id WS_ID `
  --name task-123 `
  --owner-id agent-task-123 `
  --ttl-seconds 3600 `
  --mode existing_root `
  --root D:\projects\project-a
```

不写 mode 时服务端默认 `existing_root`。这个模式只创建 ADM Environment metadata，不创建/复制/删除真实项目目录。

### 16.2 managed_worktree 模式

```powershell
& $adm environment temporary create `
  --workspace-id WS_ID `
  --name task-isolated `
  --owner-id agent-task-124 `
  --ttl-seconds 7200 `
  --mode managed_worktree `
  --base-ref HEAD
```

只有这个模式需要 Git/worktree capability。目标目录和 branch 名由 ADM 生成，caller 不能任意选择。

### 16.3 Status、Promote、Cleanup

```powershell
& $adm environment temporary status --environment-id ENV_ID
```

status 只读，不要求 owner，不拿 Writer，会显示 retention/expiry/cleanup blockers。

保留工作：

```powershell
& $adm environment temporary promote `
  --environment-id ENV_ID `
  --owner-id agent-task-123
```

promote 只把 retention 变 durable，不移动文件、不改 ENV ID、不 merge/push Git。

cleanup 默认 preview：

```powershell
& $adm environment temporary cleanup `
  --environment-id ENV_ID `
  --owner-id agent-task-123
```

确认后执行：

```powershell
& $adm environment temporary cleanup `
  --environment-id ENV_ID `
  --owner-id agent-task-123 `
  --execute
```

Temporary workflow **没有 force**。

cleanup 会 fresh recheck，常见 blocker 包括：

- 尚未过期；
- owner 不匹配；
- active Writer；
- active MCP operation/session；
- active process；
- active `run_`；
- active `vfrun_`；
- managed worktree dirty；
- managed worktree 有 unpublished commits；
- managed root identity/tamper 不确定。

ordinary existing_root cleanup 只删 ADM Environment/private context，不删项目目录/文件。

managed-worktree cleanup 会走现有 worktree destroy safety；成功移除 ADM-owned worktree root，但保留 generated branch。

## 17. Git 与 managed worktree

Agent 面有可选：

- `git_status`
- `git_diff`
- `git_branch`

非 Git Environment 调这些工具会局部失败，但 `read/search/write/exec` 等其他能力继续可用。

managed worktree 是可选 isolation：

- source 可以是 Git top-level Workspace，或 root 本身是 Git top-level 的现有 Environment；
- Agent 必须提供业务分支片段，例如 `fix/check-ref`；ADM 自动应用已配置的 branch prefix，prefix 会规范化为以 `/` 结尾；
- ADM 始终选择 worktree destination，Agent 不能传入任意路径；
- branch fragment 只接受简单 ASCII Git-safe 字符，并再次执行 Git ref 校验；
- 可显式选择是否迁移 source checkout 的未提交变动；ignored 文件不会迁移，冲突会让创建整体回滚；
- source checkout 不切 branch、不 reset、不 pull；
- managed worktree 不提供 reset/sync/reuse/switch-branch；新任务应从新的 base_ref 创建新的 managed worktree；
- routed access 前会重新验证 managed root identity；
- destroy 需 matching Writer；
- dirty/unpublished 默认拒绝；
- generic managed destroy 有 force 恢复路径，但 branch 始终保留；
- **temporary cleanup 不暴露 force**。
## 18. Desktop 怎么用

`adm-desktop` 是管理面，不是 Agent Gateway 的替代品。

它通过 `/admin/mcp` 管理当前连接，可视化：

- Dashboard / ADM connection；
- Workspace / Environment；
- Runtime / retention；
- MCP；
- Skill / Skill Sources；
- Global / Environment Memory；
- Exec allowlist、command blacklist、authorization mode 与 denial observations；
- Gateway/system/diagnostics；
- temporary Environment lifecycle 操作。

Desktop 可以保存多个 ADM connection profile。切换连接时会清空旧 scope 的数据并防止旧请求结果混入新连接。

只有 loopback HTTP root URL 可以由 Desktop 执行本地 Gateway start/stop。连接失败时 Desktop 不会直接回退读写 `state.json`。

Windows / macOS / Linux 的托盘、autostart 和平台构建说明见 [desktop.md](desktop.md)。

## 19. 哪些状态会持久化，哪些不会

### 持久化 desired state

通常包括：

- Workspace；
- Environment；
- Writer lease 元数据（过期后视为无效）；
- exec execution policy（allowlist / command blacklist / authorization mode）；
- MCP definitions；
- Skill sources/catalog snapshot；
- Environment MCP/Skill selections；
- Memory；
- Verifier definitions；
- managed-worktree identity；
- retention metadata；
- lightweight exec denial observations。

### Gateway owner-local observation，不持久化

通常包括：

- MCP live session/health/tool inventory；
- `proc_` process status/log tail/ports；
- `run_` async command observations；
- `vfrun_` async verifier observations/results。

所以 Gateway restart 后 persisted configuration 仍在，但这些 live observations 不会被“猜测恢复”。

## 20. 常见误解

### “Environment 是不是一个 Git branch？”

不是。它只是一个以目录为 root 的开发上下文。Git 是可选工具。

### “把 MCP 加到 catalog 后 Agent 就能用了？”

不一定。还必须为对应 Environment enable，而且 activation 配置/secret/executable 必须可用。

### “Skill source-update 会立刻扫描文件吗？”

不会。update 只改 source 配置，refresh 必须显式执行。

### “default=true 会把所有已有 Environment 打开吗？”

不会。default 只影响未来新建 Environment。

### “Writer 是不是整个 ADM 只能有一个？”

不是。限制是同一个 physical root 只能有一个 active writer；不同 root 可以各自拥有 writer。

### “exec allow go 后，Agent 能执行任意 shell 字符串吗？”

不是。ADM 授权 executable，仍然检查 Environment、Writer、cwd containment 等 Runtime 约束。若 executable 在 command blacklist 中，即使开启 Full Authorization 也会被拒绝。

### “temporary 到期会自动删除吗？”

不会。ADM 没有自动 GC。到期只是 cleanup eligibility 的一个条件，执行 cleanup 前仍要 current safety evidence。

### “Agent context 会自动拿到我的 Memory 和完整 Skill 内容吗？”

不会。context bundle 只提供安全摘要。Memory value 和 Skill content 必须显式读取。

## 21. 推荐的日常配置

最小本地开发：

```powershell
# 1. Gateway
& $adm gateway start --detach

# 2. Workspace / Environment
& $adm workspace add --path D:\projects --name projects
& $adm environment create --workspace-id WS_ID --name main --root D:\projects\project-a

# 3. 常用 executable
& $adm exec allow --executable git
& $adm exec allow --executable go
& $adm exec allow --executable node

# 4. 可选 MCP / Skill
& $adm mcp list
& $adm skill source-list

# 5. 检查上下文
& $adm environment inspect --environment-id ENV_ID
& $adm environment capability-report --environment-id ENV_ID
& $adm environment context --environment-id ENV_ID
```

然后把 Agent MCP 客户端连接到：

```text
http://127.0.0.1:43137/mcp
```

让 Agent 始终显式使用稳定 `environment_id`；需要 mutation 时使用一个稳定的 `writer_owner`。

## 22. 继续阅读

- [CLI 完整参考](cli.md)
- [Agent Gateway / MCP Tool 参考](AGENT_GATEWAY.md)
- [工作机制与架构](ARCHITECTURE.md)
- [Desktop](desktop.md)
- [打包与发布](packaging.md)
- [产品语义合同](PRODUCT_CONTRACT.md)
