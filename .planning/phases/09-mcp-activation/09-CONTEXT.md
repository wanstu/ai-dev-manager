# Phase 9 Context — Configured MCP Activation & Health

## Milestone

v0.2 — Usable Control Plane

## Goal

把 Phase 3 的 Effective MCP Catalog 从“只解析配置、health=unprobed”推进到真实运行生命周期：Control Plane 可以按 Workspace 的 EffectiveConfig 启动/连接 explicitly-enabled MCP、完成 initialize/list-tools 健康探测、报告结构化状态，并安全关闭。

## Locked Decisions

1. 只有 `Enabled != nil && *Enabled == true` 的 MCP 才会 activation；nil 继续表示未明确启用，false 表示 disabled。
2. Project disable/override 必须继续遵循 EffectiveConfig；activation 不重新实现继承规则。
3. 第一版支持标准 `stdio` 与 `streamable-http` client transport。
4. stdio 必须使用 executable + argv，不经过 `cmd /c`、PowerShell 或 shell 字符串解析。
5. 当前 `MCPDefinition` 缺少 stdio argv；Phase 9 允许向兼容 JSON schema 增加 `Args []string`。nil 表示继承，非 nil（包括空 slice）表示该层显式替换 argv。
6. EnvRefs 语义固定为 `child_env_name -> host_env_reference_name`。实际值只在 activation 边界解析，不写回配置、snapshot、普通 status/error。
7. stdio child 不默认继承全部宿主环境；只继承运行所需的最小基础环境变量，再叠加显式 `Env` 与 `EnvRefs`，避免绕过 EnvRefs 暴露宿主凭据。
8. MCP health 至少区分 `unprobed / disabled / starting / healthy / error / stopped`。
9. healthy 的最低证明是 MCP client Connect 成功并完成 `ListTools`；保留 tool names 作为 capability/health metadata，不执行未知工具。
10. 一个 Workspace 的 MCP activation/session 不得复用于另一个 Workspace；key 必须包含 Workspace ID + MCP ID。
11. Control Plane 负责从 Workspace ID resolve EffectiveConfig 并驱动 MCP activation manager；CLI 只增加 `mcp status` / `mcp activate` / `mcp stop` 的薄入口。
12. v0.2 不实现后台 daemon；CLI 单次 `mcp activate` 仅适合在同一进程生命周期内使用。真实长期外部 MCP supervision 可在后续 daemon/process milestone 扩展。

## SDK Grounding

项目固定 `github.com/modelcontextprotocol/go-sdk v1.7.0`。官方 v1.7 API 的 stdio client 使用 `mcp.CommandTransport{Command: exec.Command(...)}`，Streamable HTTP client 使用 `mcp.StreamableClientTransport{Endpoint: ...}`。Phase 9 只使用这些官方 transport，不自定义 MCP wire protocol。

## Runtime Shape

建议在现有 `internal/mcp` package 增加 activation manager：

```text
EffectiveConfig
  ↓
mcp.Registry.FromEffective
  ↓
Enabled entries only
  ↓
Activation Manager
  ├── stdio: exec.Command + CommandTransport
  └── streamable-http: StreamableClientTransport
  ↓
Client.Connect
  ↓
ListTools
  ↓
healthy status + retained ClientSession
```

Control Plane 持有该 manager，并暴露：

```text
ActivateConfiguredMCPs(workspaceID)
MCPStatuses(workspaceID)
StopConfiguredMCP(workspaceID, mcpID)
StopConfiguredMCPs(workspaceID)
```

具体 Go API 可按测试最小化。

## Status Safety

状态可以显示：

```text
workspace_id
mcp_id
transport
health
source
enabled_source
tool_names
error_kind / safe error message
```

不得显示：

```text
Env value
EnvRefs resolved value
完整 child environment
```

## Verification Strategy

1. 增加 MCP `Args` resolver/store round-trip + override tests。
2. 使用当前 test binary 自己提供一个 hidden stdio MCP fixture mode，避免依赖外部 npm/python/server。
3. Global enabled stdio MCP 被 Workspace A/B 各自 activation，两个 session key 隔离。
4. Project A disable 后 A status=disabled/不启动，B 仍 healthy。
5. fixture 通过 EnvRef 读取一个测试环境值并仅返回是否存在，不返回实际值；missing EnvRef 必须结构化失败。
6. Streamable HTTP 使用项目现有 MCP HTTP server fixture 做至少一个健康连接测试。
7. CLI status/activation 输出不包含 env value。

## Exit Boundary

Phase 9 通过后 v0.2 Milestone 完成。Milestone 边界必须停止 auto-advance，等待 v0.3 规划，不自动进入 Agents/GSD Runtime。
