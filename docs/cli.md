# `adm` CLI 完整参考（v1.1）

`adm` 是 ADM 的命令行管理端和本机 Gateway 启动器。

- 正常 `workspace` / `environment` / `exec` / `mcp` / `skill` / `memory` 管理通过所选 ADM 的 `/admin/mcp`。
- `gateway` / `doctor` / `state` 是明确的本机 bootstrap/offline/recovery 命令。
- 成功的正常管理命令使用 pretty JSON stdout，适合脚本读取；错误返回非零退出码。

概念与使用场景见 [USER_GUIDE.md](USER_GUIDE.md)。

## 1. 选择 ADM 管理目标

默认：

```text
http://127.0.0.1:43137
```

命令行覆盖：

```powershell
adm --adm-url http://127.0.0.1:8001 workspace list
```

环境变量覆盖：

```powershell
$env:ADM_V2_URL = 'http://127.0.0.1:8001'
adm workspace list
```

CLI 会同时读取用户级 `~/.config/adm/.env` 和 executable 同目录 `.env`。优先级固定为：**已有进程环境变量 > executable 同目录 `.env` > `~/.config/adm/.env`**。因此可以把常用远程连接写在用户级 `.env`，再用某个便携版/安装实例同目录的 `.env` 做局部覆盖。

例如 `~/.config/adm/.env`：

```dotenv
ADM_V2_URL=http://101.37.171.174:43137
ADM_V2_ADMIN_API_KEY=你的客户端AdminKey
```

这里的 `ADM_V2_ADMIN_API_KEY` 是 **CLI 作为客户端连接 `/admin/mcp` 时使用的 Key**；它必须与远端 ADM 服务端 Remote access 中配置的 Admin Key 一致。

管理目标不可达时不会 fallback 到本地 writable `state.json`。

## 2. 总帮助

```powershell
adm -h
adm workspace -h
adm environment -h
adm environment temporary -h
adm environment writer -h
adm environment verifier -h
adm environment mcp -h
adm environment skill -h
adm exec -h
adm mcp -h
adm skill -h
adm memory -h
adm gateway -h
adm doctor -h
adm state -h
```

`environment` 可简写为 `env`。

# Workspace

## `workspace add`

```text
adm workspace add --path PATH [--name NAME]
```

登记一个现有本地目录。Git 不是前置条件。

示例：

```powershell
adm workspace add --path D:\projects --name projects
```

## `workspace list`

```text
adm workspace list
```

列出所有 registered Workspace。

## `workspace inspect`

```text
adm workspace inspect --workspace-id WS_ID
```

按 stable ID 查看一个 Workspace。

## `workspace discover`

```text
adm workspace discover --workspace-id WS_ID \
  [--path REL] [--query TEXT] \
  [--max-depth N] [--max-entries N] [--max-candidates N] \
  [--max-digest-entries N] [--max-output-bytes N]
```

执行 bounded metadata-only 项目发现。

特点：

- 不读取文件正文；
- 不创建 Environment；
- 不要求 Writer/Git/MCP/Skill/verifier；
- budget 为 `0` 时使用 Core 默认值。

## `workspace rename`

```text
adm workspace rename --workspace-id WS_ID --name NAME
```

只改显示名称，不移动/重命名真实目录。

## `workspace remove`

```text
adm workspace remove --workspace-id WS_ID
```

只删除 ADM registration，不删除目录/文件。仍有 Environment 引用时拒绝。

# Environment

## `environment create`

```text
adm environment create --workspace-id WS_ID --name NAME [--root PATH]
```

创建 durable Environment。

`--root` 省略时默认使用 Workspace root；显式 root 必须是 Workspace 内的已存在目录。

## `environment list`

```text
adm environment list
```

返回 lightweight summaries。

## `environment inspect`

```text
adm environment inspect --environment-id ENV_ID
```

返回 Workspace relation、capability facts、resolved/unresolved MCP/Skill selections、private Memory count 等，不展开 private values。

## `environment capability-report`

别名：`environment capabilities`。

```text
adm environment capability-report --environment-id ENV_ID
```

返回 canonical `CapabilityReport`。正常 CLI 通过 Admin MCP/Gateway 路径，因此可包含已有 runtime owner observation，但不会主动 probe/connect/execute。

## `environment injection-plan`

```text
adm environment injection-plan --environment-id ENV_ID
```

返回 canonical passive MCP / Skill injection plan。它只包含当前 Environment 最终 selected 的能力，并返回 `selection_sources`、`default_include_in_environment`、`injectable`、`state/reason_code` 与 `next_action`。

该命令不会连接 MCP、读取 Skill 正文、获取 Writer 或修改 selection。Workspace inheritance 会明确标记为 `workspace`，因此 Agent 可以区分“Environment 显式启用”和“Workspace 继承”。

## `environment context`

```text
adm environment context --environment-id ENV_ID \
  [--path REL] \
  [--max-depth N] [--max-entries N] \
  [--max-digest-entries N] [--max-output-bytes N]
```

返回 canonical bounded Environment Context Bundle。

不会：

- 隐式选择 Environment；
- 读取 Memory values；
- 读取完整 Skill content；
- 主动 MCP probe；
- 执行 verifier/process/run；
- acquire Writer。

## `environment tree-digest`

```text
adm environment tree-digest --environment-id ENV_ID \
  [--path REL] \
  [--max-depth N] [--max-entries N] [--max-candidates N] \
  [--max-digest-entries N] [--max-output-bytes N]
```

读取 Environment root 下 bounded directory metadata 摘要，不扩展到 Workspace sibling。

## `environment rename`

```text
adm environment rename --environment-id ENV_ID --name NAME
```

只改 metadata display name。

## `environment remove`

```text
adm environment remove --environment-id ENV_ID
```

只删 ordinary Environment ADM record，不删 root/project files。active Writer 阻止 remove；managed worktree Environment 不能绕过 managed destroy safety。

# Temporary Environment

## `environment temporary create`

```text
adm environment temporary create \
  --workspace-id WS_ID --name NAME \
  --owner-id OWNER --ttl-seconds N \
  [--session-id ID] [--run-id ID] \
  [--mode existing_root|managed_worktree] \
  [--root PATH] [--base-ref REF]
```

规则：

- `owner-id` 必须非空；
- TTL 必须为正数；
- mode 省略时服务端默认 `existing_root`；
- `root` 只用于 `existing_root`；
- `base-ref` 只用于 `managed_worktree`；
- session/run 仅 provenance，不启动/绑定任务。

示例：

```powershell
adm environment temporary create `
  --workspace-id ws_xxx `
  --name task-42 `
  --owner-id agent-task-42 `
  --ttl-seconds 3600 `
  --mode existing_root `
  --root D:\projects\app
```

## `environment temporary status`

```text
adm environment temporary status --environment-id ENV_ID
```

只读查看 retention/expiry/current cleanup blockers。无需 lifecycle owner，不获取 Writer。

## `environment temporary promote`

```text
adm environment temporary promote --environment-id ENV_ID --owner-id OWNER
```

matching lifecycle owner 才能 promote。只把 retention 改 durable，不移动目录、不 merge/push Git、不改 Environment ID。

## `environment temporary cleanup`

Preview：

```text
adm environment temporary cleanup --environment-id ENV_ID --owner-id OWNER
```

Execute：

```text
adm environment temporary cleanup --environment-id ENV_ID --owner-id OWNER --execute
```

默认 preview。`--execute` 触发 fresh safety recheck。**没有 force。**

ordinary existing root cleanup 不删除项目目录/文件；managed worktree cleanup 保留 generated branch。

# Environment Writer

## `environment writer acquire`

```text
adm environment writer acquire --environment-id ENV_ID --owner OWNER
```

获取或续租 Writer。

## `environment writer heartbeat`

```text
adm environment writer heartbeat --environment-id ENV_ID --owner OWNER
```

只续租。

## `environment writer release`

正常：

```text
adm environment writer release --environment-id ENV_ID --owner OWNER
```

人工恢复：

```text
adm environment writer release --environment-id ENV_ID --force
```

Writer 默认 TTL 为 5 分钟；force 是 recovery fallback，不是常规流程。

# Environment MCP selection

## Enable

```text
adm environment mcp enable --environment-id ENV_ID --mcp-id MCP_ID
```

## Disable

```text
adm environment mcp disable --environment-id ENV_ID --mcp-id MCP_ID
```

只修改该 Environment selection，不修改 global definition/default，也不影响其他 Environment。

# Environment Skill

## Enable / Disable

```text
adm environment skill enable --environment-id ENV_ID --skill-id SKILL_ID
adm environment skill disable --environment-id ENV_ID --skill-id SKILL_ID
```

## `environment skill list`

```text
adm environment skill list --environment-id ENV_ID
```

查看该 Environment 的 Skill availability，不读取 Skill instructions。

## `environment skill inspect`

```text
adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID
```

查看单个 Skill 的 enabled/disabled/broken availability 和结构事实。

# Environment Verifier definitions

## `environment verifier add`

```text
adm environment verifier add \
  --environment-id ENV_ID \
  --kind test|lint|build|custom \
  --executable NAME_OR_PATH \
  [--name NAME] \
  [--arg ARG ...] \
  [--cwd RELATIVE_PATH] \
  [--timeout-seconds N] \
  [--enabled=true|false]
```

`--arg` 可重复。

注意：定义 verifier 不会自动允许 executable。需要另行 `adm exec allow`。

示例：

```powershell
adm exec allow --executable go
adm environment verifier add `
  --environment-id env_xxx `
  --kind test `
  --executable go `
  --arg test `
  --arg ./... `
  --timeout-seconds 300
```

## `environment verifier list`

```text
adm environment verifier list --environment-id ENV_ID
```

## `environment verifier remove`

```text
adm environment verifier remove --environment-id ENV_ID --verifier-id VF_ID
```

CLI 当前负责定义管理；实际 verifier run 是 Agent Gateway Runtime tool，见 [AGENT_GATEWAY.md](AGENT_GATEWAY.md)。

# Exec Allowlist

## `exec allow`

```text
adm exec allow --executable NAME_OR_PATH
```

允许一个 executable 用于 Environment Runtime execution。

## `exec remove`

```text
adm exec remove --executable NAME_OR_PATH
```

立即移除；后续执行按新 allowlist 判定。

## `exec list`

```text
adm exec list
```

列当前 allowlist。

# MCP Catalog

## `mcp add` — Streamable HTTP

```text
adm mcp add --name NAME \
  --transport streamable-http \
  --endpoint URL \
  [--auth-mode none|headers] \
  [--header-refs-json JSON] \
  [--health-check-enabled] \
  [--check-interval-seconds N] \
  [--probe-timeout-seconds N] \
  [--auto-reconnect] \
  [--reconnect-interval-seconds N] \
  [--default]
```

示例：

```powershell
adm mcp add `
  --name api-tools `
  --transport streamable-http `
  --endpoint http://127.0.0.1:9000/mcp
```

Header secret 必须使用环境引用，例如：

```powershell
adm mcp add `
  --name private-api `
  --transport streamable-http `
  --endpoint https://example.invalid/mcp `
  --auth-mode headers `
  --header-refs-json '{"Authorization":"${PRIVATE_API_AUTH}"}'
```

## `mcp add` — stdio

```text
adm mcp add --name NAME \
  --transport stdio \
  --executable PATH \
  [--args-json JSON_ARRAY] \
  [--env-refs-json JSON_OBJECT] \
  [health policy flags] \
  [--default]
```

stdio executable 需要 exec allowlist。

## `mcp list`

```text
adm mcp list
```

列 global MCP desired definitions，不等于 Environment authorization 或 live health。

## `mcp set-default`

```text
adm mcp set-default --id MCP_ID --enabled true|false
```

只影响未来新建 Environment。

## `mcp remove`

```text
adm mcp remove --id MCP_ID
```

删除 global entry。已有 Environment selection ID 不会被静默重写，可表现为 unresolved reference。

# MCP Import

## `mcp import-preview`

```text
adm mcp import-preview \
  (--json-or-jsonc CONTENT | --file PATH | --stdin) \
  [--format auto|generic-mcpservers|opencode|workbuddy|codex-plugin|claude-code|mcphub] \
  [--source-scope SCOPE] \
  [--default]
```

三种内容来源必须且只能提供一个。

Preview 不修改 catalog/Environment selection，不持久化 raw blob。

## `mcp import-apply`

```text
adm mcp import-apply \
  (--json-or-jsonc CONTENT | --file PATH | --stdin) \
  [--format FORMAT] \
  [--selected-names A,B] \
  [--conflict-policy error|skip|update_by_name] \
  [--source-scope SCOPE] \
  [--default]
```

Apply 重新解析并原子写入 selected global definitions，不修改现有 Environment selections。

# MCP Diagnostics

## `mcp probe`

```text
adm mcp probe --id MCP_ID
```

Global transient configuration/connection probe，无 Environment。

不会 enable、不会持久化 health、不会调 business tool。

## `mcp status`

```text
adm mcp status --id MCP_ID --environment-id ENV_ID
```

Environment-aware 一次性 configured/disabled/healthy/error 状态探测。

## `mcp inspect`

```text
adm mcp inspect --id MCP_ID --environment-id ENV_ID
```

**Passive**：desired config + 已有 owner-local observation/inventory。不会连接/Ping/刷新。

## `mcp refresh`

```text
adm mcp refresh --id MCP_ID --environment-id ENV_ID
```

**Active**：discard old session/observation，然后 reconnect/Ping/refresh bounded tool inventory。

不会调用 business tool，也不会修改 desired definition/Environment selection。

# Skill Catalog / Sources

## `skill add`

```text
adm skill add --root PATH [--support-root PATH] [--default]
```

便利入口：从 explicit discovery root 扫描真实 `SKILL.md`。更精细的 source 生命周期建议使用下面的 source commands。

## `skill source-list`

```text
adm skill source-list
```

列 persisted Skill Sources，不扫描主机路径。

## `skill source-add`

```text
adm skill source-add --root PATH [--support-root PATH ...] [--default]
```

登记 source，**不 refresh**。`--support-root` 可重复。

## `skill source-update`

```text
adm skill source-update --id SOURCE_ID --root PATH \
  [--support-root PATH ...] \
  [--default=true|false]
```

更新 source config，**不 refresh**。

省略 `--default` 时保留当前值。

## `skill source-refresh`

```text
adm skill source-refresh --id SOURCE_ID
```

显式原子 refresh 一个 source snapshot。

## `skill source-remove`

```text
adm skill source-remove --id SOURCE_ID
```

移除 source 和 source-owned discovered Skills；Environment selection ID 不静默改写。

## `skill list`

```text
adm skill list
```

列 catalog inventory。**不是 availability**。

## `skill availability`

```text
adm skill availability
```

列 global structural availability，不参考某个 Environment enablement，不读取 Skill instructions。

## `skill set-default`

```text
adm skill set-default --id SKILL_ID --enabled true|false
```

只影响未来 Environment。

## `skill remove`

```text
adm skill remove --id SKILL_ID
```

删除 global Skill entry。

# Memory

## Global Memory

### List

```text
adm memory global list
```

### Read

```text
adm memory global read --key KEY
```

### Write

```text
adm memory global write --key KEY --value VALUE
```

`VALUE` 可以为空字符串，但必须显式提供 `--value`。

### Delete

```text
adm memory global delete --key KEY
```

## Environment-private Memory

### List

```text
adm memory environment list --environment-id ENV_ID
```

### Read

```text
adm memory environment read --environment-id ENV_ID --key KEY
```

### Write

```text
adm memory environment write --environment-id ENV_ID --key KEY --value VALUE
```

### Delete

```text
adm memory environment delete --environment-id ENV_ID --key KEY
```

Global 与 private scope 永远显式，不自动 copy/promote/merge。

# Gateway

## `gateway start`

```text
adm [--adm-url URL] gateway start [--listen HOST:PORT] [-d|--detach]
```

不写 `--listen` 时从当前 ADM Base URL 推导 loopback listen。

前台模式阻塞当前 terminal，Ctrl+C 停止。

后台：

```powershell
adm gateway start --detach
```

命令在 `/healthz` ready 后才返回。

### 远程监听与双 API Key

远程监听前必须同时配置 Host/IP 白名单、Admin API Key 和 Agent API Key：

```powershell
$env:ADM_V2_ADMIN_API_KEY = 'your-admin-key'
$env:ADM_V2_AGENT_API_KEY = 'your-agent-key'
adm gateway access set-admin-key
adm gateway access set-agent-key
adm gateway access set-hosts --hosts '101.37.171.174,adm.example.com'
adm gateway start --listen 0.0.0.0:43137 -d
```

`/admin/mcp` 只接受 Admin Key；`/mcp` 只接受 Agent Key，两把 Key 不能互换。

白名单支持 `*`：

```powershell
adm gateway access set-hosts --hosts '*'
```

`*` 表示不限制 Host/IP，但双 Key 鉴权仍然强制；`0.0.0.0` 不是白名单通配符，它只表示监听所有网卡时常用的 listen 地址。完整说明见 [REMOTE_ACCESS.md](REMOTE_ACCESS.md)。

## `gateway status`

```text
adm [--adm-url URL] gateway status [--listen HOST:PORT]
```

显示状态、MCP URL、PID、version、Runtime Owner。

`status` 可以检查自定义端口或远端 health；远端仅是查看，不获得 stop 权限。

## `gateway logs status`

```text
adm gateway logs status
```

显示 ADM 持久日志目录、当前单文件轮转阈值、每个日志级别的 rotated-file 保留数量。该命令不读取或输出日志正文，也不会返回 secret/token 等字段。

## `gateway stop`

```text
adm [--adm-url URL] gateway stop [--listen HOST:PORT]
```

只停止本机 loopback Gateway，不通过远端 URL 发 shutdown。

## `gateway restart`

```text
adm [--adm-url URL] gateway restart [--listen HOST:PORT]
```

停止本机旧 Gateway 后在当前 terminal 启动新 Gateway。

## `gateway stdio`

```text
adm gateway stdio
```

只供 MCP client 以 stdin/stdout transport 自动启动。人工 terminal 运行会被拒绝/提示。

# Doctor / State

## `doctor`

```text
adm doctor
```

本机诊断：

- 当前 executable；
- state path；
- 默认 Gateway 状态；
- Workspace；
- Environment / Writer；
- exec allowlist。

这是本机 recovery 入口，不等同于远端 Admin MCP management。

## `state path`

```text
adm state path
```

打印当前本机 ADM persistent state 文件路径。

# 脚本化注意事项

## JSON stdout

正常管理命令成功结果为 JSON，因此 PowerShell 可以：

```powershell
$envs = adm environment list | ConvertFrom-Json
$envs | Select-Object id,name,root
```

## 不要解析 human help 当 API

`-h`、`doctor`、Gateway lifecycle 输出是人类文本。自动化的数据接口应使用正常 JSON management commands 或 Admin MCP。

## Stable ID 优先

不要通过 name 猜目标；脚本应保存/传递 `ws_...`、`env_...`、`mcp_...`、`skill_...` stable IDs。

## 不要用 shell cwd 推断 Environment

CLI 不提供 hidden current Environment；这也是安全特性。

# 相关文档

- [完整用户手册](USER_GUIDE.md)
- [Agent Gateway 工具参考](AGENT_GATEWAY.md)
- [工作机制与架构](ARCHITECTURE.md)
- [Desktop](desktop.md)
- [产品合同](PRODUCT_CONTRACT.md)
