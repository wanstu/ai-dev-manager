# Phase 10 Context — Local Daemon & Control API

## Milestone

v0.3 — Persistent Control Plane / Runtime Lifecycle

## Goal

把 v0.2 的 `controlplane.Service` 从“每次 CLI invocation 临时创建的对象”推进为一个由本地 daemon 长期持有的唯一 Control Plane owner。Phase 10 只验证跨进程 lifecycle/control boundary；Workspace Runtime/MCP 的长期 start/status/stop ownership 留给 Phase 11。

## Why This Phase Exists

当前 CLI 每次调用都会 `controlplane.New(...)`，`host.Manager.instances` 与 `mcp.Activator.sessions` 都只存在于该 Go process 内。`mcp activate` 还会在命令返回前主动 Stop。这个结构已经足够验证 v0.2 的真实 activation/probe，但无法成为 Agent、UI 或长期 MCP supervision 的稳定 owner。

因此 Phase 10 首先建立：

```text
CLI A: start ─┐
CLI B: status ├─> Local Control API ─> one daemon process ─> one controlplane.Service
CLI C: stop  ─┘
```

## Locked Decisions

1. daemon 是 Control Plane 的 owner，不在 CLI 里建立第二套 resolver/runtime 业务层。
2. Phase 10 control endpoint 仅 local/loopback；不做公网 HTTPS、tunnel、remote auth。
3. 同一 config root 同时只允许一个健康 daemon owner。
4. discovery metadata 可以持久化 PID、instance ID、control address、started_at 等可恢复数据；不得持久化 Go pointer、listener、Runtime object、MCP ClientSession。
5. daemon restart/reconcile desired Workspace runtime state 属于 Phase 12；Phase 10 只要求 daemon 自身 start/status/stop。
6. Phase 10 不把 existing foreground `serve` / `mcp activate` 强行重写为 daemon RPC；它们的 runtime ownership 迁移属于 Phase 11。
7. local control protocol 应保持实现最小、机器可测。优先标准库；不要为了第一版引入大型 RPC framework。
8. daemon shutdown 必须 graceful：收到 control stop 或 OS cancellation 后关闭 Control Plane，再移除当前 instance 的 discovery metadata。
9. stale metadata 需要可识别并可修复，不能让一次 crash 永久阻止下一次 `start`。
10. Windows 是当前真实 acceptance 环境，但领域/API 不应无必要绑定 Windows-only service manager。

## Minimal Surface

```text
ai-dev-manager start
ai-dev-manager status
ai-dev-manager stop
```

允许存在仅供内部 child process 使用的 hidden daemon-run mode，但它不是第二套产品 API。

最小 status 至少包含：

```text
instance_id
pid
control_endpoint
state
started_at
```

## Security Boundary

- control listener 必须 loopback-only。
- metadata/status 不输出 config Env values、resolved EnvRefs 或其他 secret。
- control API 第一版只暴露 daemon health/stop 等明确方法，不做 arbitrary command execution。
- discovery/state 文件放在 config root 下的 app-owned runtime area，而不是 Workspace 可编辑区域。

## Verification Strategy

1. 单元/集成测试验证 metadata read/write/stale cleanup 与 loopback validation。
2. process-level test 使用当前 test binary 的 helper mode 或构建后的 real binary，证明 child daemon 跨调用存活。
3. start 后创建新的 CLI invocation 执行 status，必须命中同一 instance ID/PID。
4. stop 后进程退出且 metadata 被清理。
5. 已运行时重复 start 返回现有 healthy daemon 或结构化 already-running 结果，不创建第二 owner。
6. 全量 `go test ./...`、`go vet ./...`、build gate。

## Exit Boundary

Phase 10 通过后自动进入 Phase 11；不得在 Phase 10 顺手迁移 Agent/GSD、Docker、generic Process Manager 或 persistent Workspace runtime semantics。
