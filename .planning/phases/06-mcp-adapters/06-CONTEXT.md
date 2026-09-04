# Phase 6 Context — MCP Host & External Runtime Adapter

## Phase Goal

让 ai-dev-manager 的 Core 真正可被 MCP client 调用，同时保持协议与 Runtime/Core 解耦；支持一个 Workspace 一个独立 MCP HTTP instance，并提供 stdio 入口；定义稳定 Runtime Adapter contract，为 CodexPro / DevSpace / 其他外部 runtime 后续接入提供边界。

## Requirements

- RUN-03
- COMPAT-02
- COMPAT-03
- COMPAT-04

## Locked Decisions

1. MCP 协议层使用官方 `github.com/modelcontextprotocol/go-sdk/mcp`，依赖只进入 adapter/host 层，不进入 model/config/runtime Core。
2. 目标协议以当前 2026-07-28 为基线；Streamable HTTP 使用 stateless handler。
3. 同时支持：
   - stdio transport：给本机 MCP client / agent。
   - Streamable HTTP：给独立 Workspace server instance，未来可由 HTTPS tunnel/reverse proxy 暴露给 ChatGPT。
4. MCP tool 集合根据 Runtime Adapter 的 capabilities 动态暴露；不存在的 capability 不注册对应 tool。
5. MCP tool handler 不重新实现安全逻辑，只调用 Native/Adapter；Workspace containment、policy、ToolPaths、Git/worktree/verifier 仍由 Runtime 强制。
6. Core 不 import MCP SDK。Native 通过 `NativeAdapter` 映射到一个协议无关的 Runtime Adapter contract。
7. External Runtime Adapter contract 采用 capability + generic invoke + status，避免要求外部 runtime 实现 Native 的全部 typed methods。
8. 一个 HTTP Instance 固定绑定一个 Runtime Adapter/Workspace，不在单 endpoint 内通过用户参数切换 Workspace；隔离发生在 server instance 层。
9. Lifecycle Manager 管理 instance id、workspace id、transport、address/endpoint、state、start/stop；不持久化 PID/临时 session 到 Project config。
10. HTTP 默认只监听 loopback。公开 HTTPS/tunnel/auth 属于后续控制面部署能力，不在本 Phase 自动开放网络。
11. MCP 返回结构化结果，并在错误时使用 tool error，不把 Runtime 内部配置/凭据放进错误正文。
12. CodexPro / DevSpace 本 Phase只写 compatibility contract + fake external adapter acceptance，不读取/修改 `codexprov4`。

## Runtime Adapter Contract

建议 `internal/adapter/runtimeadapter`：

```go
type Status struct {
    ID           string
    WorkspaceID  string
    State        string
    Capabilities []string
}

type Runtime interface {
    ID() string
    WorkspaceID() string
    Capabilities() []string
    Status(context.Context) Status
    Invoke(context.Context, string, map[string]any) (any, error)
}
```

`NativeAdapter` 把 capability 名映射到 Native typed API。

Phase 6 必须真实使用这个接口：MCP adapter 只依赖此接口，而不直接依赖 `*runtime.Native`。

## MCP Tool Surface

最小映射：

```text
files.tree        -> tree
files.read        -> read
files.write       -> write
files.edit        -> edit
search.text       -> search
shell.exec        -> exec
git.status        -> git_status
git.diff          -> git_diff
git.branch        -> git_branch
git.worktree      -> git_worktrees / git_worktree_create / git_worktree_remove
verify.run        -> run_verifier / run_verifiers
```

另提供只读：

```text
runtime_info
```

`runtime_info` 返回 workspace id / runtime id / capability，不返回本地敏感配置。

## MCP Adapter

建议：

```text
internal/adapter/mcpserver/
├── server.go
├── tools.go
├── stdio.go
├── http.go
└── *_test.go
```

- `New(adapter Runtime) *mcp.Server`
- `RunStdio(ctx, adapter)`
- `NewHTTPHandler(adapter) http.Handler`

HTTP handler 使用 stateless Streamable HTTP。

## Lifecycle Manager

建议：

```text
internal/host/
├── manager.go
└── manager_test.go
```

最小：

```go
StartHTTP(instanceID string, adapter Runtime, addr string) (Instance, error)
Stop(ctx context.Context, instanceID string) error
Get(instanceID string) (Instance, bool)
List() []Instance
```

规则：
- `addr=""` 默认 `127.0.0.1:0`。
- 非 loopback address 默认拒绝，避免无意公网暴露。
- 每个 instance 独立 listener + MCP server + adapter。
- endpoint 为 `http://<actual-addr>/mcp`。
- Stop idempotent。

## Acceptance Tests

### In-memory MCP

用官方 SDK `NewInMemoryTransports`：
1. Native Adapter -> MCP Server。
2. client ListTools。
3. ReadOnly runtime 只看到 read/search/tree，不看到 write/exec/git。
4. Call read/search 返回真实 Workspace A 内容。
5. Standard runtime 能 call edit/git/verifier。

### HTTP instances

1. 创建 Workspace A/B Native runtimes。
2. Manager 启动两个 `127.0.0.1:0` instance。
3. 官方 Streamable HTTP client 分别连接 endpoint A/B。
4. A read 只能看到 A；B read 只能看到 B。
5. 两 endpoint 地址不同。
6. Stop A 后 B 仍可调用。

### External Adapter

实现测试 fake adapter：
- capability `external.echo`。
- MCP server 动态暴露映射/或 adapter generic invoke acceptance。
- status 可映射。

本 Phase不要求真的启动 DevSpace/CodexPro。

## Compatibility Contract

文档 `docs/runtime-adapter-contract.md` 至少描述：
- runtime identity/workspace identity
- capability naming
- invoke input/output/error contract
- lifecycle/status
- CodexPro adapter 需要映射哪些现有能力
- DevSpace adapter 需要映射哪些能力
- 配置继承仍属于 ai-dev-manager Control Plane，不委托给外部 runtime

## Exit Criteria

- 官方 MCP client 能通过 in-memory transport调用 Native tools。
- 官方 Streamable HTTP client 能调用一个真实 Workspace Runtime。
- 两个 Workspace 可同时启动两个隔离 HTTP MCP instances。
- Stop 一个 instance 不影响另一个。
- stdio entry 能编译并有 transport-boundary test/contract。
- MCP adapter 只依赖 Runtime Adapter interface，不直接耦合 Config Store/Workspace Registry。
- External fake runtime 能通过统一 adapter contract 被 host/status 层处理。
- CodexPro/DevSpace compatibility contract 已文档化，不修改 codexprov4。
- `go test ./...`、`go vet ./...` 全绿。
