# Phase 11 Context — Persistent Workspace Runtime Ownership

## Milestone

v0.3 — Persistent Control Plane / Runtime Lifecycle

## Goal

把 v0.2 已有的 `host.Manager` 与 configured MCP `Activator` 从“某次 CLI process 内的能力”真正放到 Phase 10 daemon 的长期 `controlplane.Service` 之下。一个 Workspace runtime 在 daemon 生命周期内拥有稳定的 observed state、MCP Host endpoint 与 configured MCP sessions；新的 CLI invocation 只通过 local Control API 查询/控制它。

## Phase Boundary

Phase 11 解决：

```text
CLI
 ↓ local control API
Daemon
 ↓
Runtime Owner
 ├─ Workspace A
 │   ├─ Native Runtime Adapter
 │   ├─ MCP Host endpoint
 │   └─ configured MCP sessions
 └─ Workspace B
     ├─ Native Runtime Adapter
     ├─ MCP Host endpoint
     └─ configured MCP sessions
```

Phase 11 **不解决 daemon restart 后恢复**。desired state 的磁盘持久化和 startup reconciliation 属于 Phase 12。

## Locked Decisions

1. daemon 中现有的那个 `controlplane.Service` 必须是 Runtime Owner 唯一使用的 Control Plane；不得为每个 runtime command 再 `controlplane.New()`。
2. Workspace runtime instance key 固定以 Workspace ID 为核心；一个 Workspace 在同一个 daemon 中最多一个 active owned runtime instance。
3. `runtime start` 的最小真实含义是：
   - 从 Control Plane 构造该 Workspace Runtime；
   - 启动一个 daemon-owned loopback MCP Host instance；
   - activation/probe 该 Workspace explicitly-enabled configured MCPs；
   - 只有这些步骤形成一致 observed state 后才报告 `running`。
4. configured MCP activation 失败不能留下伪 `running` runtime；第一版优先 rollback 已启动 Host/session，并报告结构化 `error`。
5. Runtime Owner 维护内存中的 desired/observed 分离：`desired_running` 表示当前 daemon 希望该 Workspace 运行；`state` 表示 observed `starting/running/stopping/stopped/error`。
6. Phase 11 desired state 只在 daemon 内存存在；**不写磁盘**。Phase 12 再持久化并 reconcile。
7. `runtime status` 必须来自 daemon observed state，并结合现有 Control Plane Host/MCP status；不能由 CLI 重新 BuildRuntime 后猜测“running”。
8. `runtime stop` 关闭该 Workspace configured MCP sessions 与 daemon-owned MCP Host，不影响其他 Workspace。
9. daemon shutdown 必须 stop all owned runtimes，并最终调用 Control Plane `StopAll`，确保没有遗留 Host/session。
10. Existing foreground `serve` 与 one-shot `mcp activate` 可以暂时保留兼容，但新的 persistent lifecycle surface 必须走 daemon Runtime Owner。
11. 不新增通用 Process Manager、开发服务器 lifecycle、Docker、Agent executor。

## Runtime Status Shape

至少包含：

```text
workspace_id
runtime_id
desired_running
state
mcp_host:
  instance_id
  endpoint
configured_mcps:
  id
  health
  tool_names
error (safe, optional)
```

状态不得包含 Env values 或 resolved EnvRefs values。

## CLI Surface

```text
ai-dev-manager runtime start  --workspace <id>
ai-dev-manager runtime status --workspace <id>
ai-dev-manager runtime stop   --workspace <id>
```

可增加 `runtime list`，仅在它直接服务多 Workspace acceptance 且实现很小的情况下。

## Verification Strategy

1. Runtime Owner 单元/集成测试：start idempotency、stop independence、activation rollback。
2. daemon control API 测试：runtime start/status/stop 都操作 daemon 中同一 owner。
3. 跨进程 CLI acceptance：daemon start → runtime start → CLI exits → new CLI runtime status 仍 running。
4. 两个真实 temp Workspace 同时 start，得到不同 MCP endpoint；stop A 后 B 仍 running/可连接。
5. configured MCP status 来自 daemon retained Activator session，而不是每个 CLI invocation 新 probe 后立即关闭。
6. daemon stop 后所有 runtime Host/session 清理。

## Exit Boundary

Phase 11 通过后自动进入 Phase 12。Phase 12 才允许持久化 `desired_running` 并验证 daemon restart reconciliation。
