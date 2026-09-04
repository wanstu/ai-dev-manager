# Phase 7 Context — Control Plane Composition & Introspection

## Milestone

v0.2 — Usable Control Plane

## Goal

把 v0.1 已完成但彼此分层的 Workspace Registry、ConfigService、Native Runtime、Runtime Adapter 和 MCP Host 组合成一个正式的 application/control-plane boundary。上层调用方只需要 Workspace ID，不应知道如何手工拼装底层组件。

## Locked Decisions

1. Control Plane 与 Runtime 继续分离；Control Plane 负责 resolve/composition/lifecycle，Runtime 只消费 EffectiveConfig。
2. CLI 将是 Control Plane 的薄客户端，因此本 Phase 不实现 CLI。
3. 当前只构造 `native` runtime；Workspace 声明未知 RuntimeID 时返回结构化 unsupported 错误，不能静默回退 Native。
4. 空 RuntimeID 视为当前默认 `native`，用于兼容 v0.1 已持久化 Workspace。
5. MCP Host 继续一 instance → 一 Workspace Runtime，继续 loopback-only。
6. Introspection 只返回可安全展示的配置/来源/能力，不解析或展开 EnvRefs 对应的 secret value。
7. Runtime Override 仍由调用方按次传入内存，不持久化。
8. 不引入后台 daemon、IPC、Docker、Agent、外部 Runtime Adapter 实现。

## Existing Building Blocks

- `internal/config.Store`
- `internal/workspace.Registry`
- `internal/workspace.ConfigService`
- `internal/runtime.Native`
- `internal/adapter/runtimeadapter.NativeAdapter`
- `internal/host.Manager`

当前缺口不是这些组件的能力，而是没有正式 composition root：测试代码必须自己 new Store/Registry/ConfigService/Native/Adapter/Host。

## API Shape

新增 `internal/controlplane`，第一版 Service 应至少承担：

```text
New(root)
Registry access / Workspace lookup
Resolve(workspaceID, runtimeOverride)
Inspect(workspaceID, runtimeOverride)
BuildRuntime(workspaceID, runtimeOverride)
StartMCP(instanceID, workspaceID, runtimeOverride, addr)
GetMCP / ListMCP / StopMCP / StopAll
```

具体函数签名允许实现时按 Go 习惯调整，但职责不能漂移到 CLI 或 MCP adapter。

## Introspection

Snapshot 至少包含：

```text
Workspace:
  id/path/profile/runtime

Runtime:
  selected runtime id
  effective policy mode/source
  capabilities

Effective config:
  MCP id + enabled + source + enabled_source + transport/url/command metadata
  Skill id + enabled + source + enabled_source + path
  Verifier id + kind + enabled + source + enabled_source + executable/cwd
```

Env/EnvRefs 的实际 value 不应进入 snapshot；EnvRefs 可以只展示引用名或计数，若没有真实需求优先不展示值。

## Verification Strategy

必须使用真实持久化 Store + Registry temp root，不只 mock：

1. 注册 temp workspace。
2. Global/Profile/Project 配置形成不同来源。
3. Control Plane inspect 验证来源与 policy/capabilities。
4. Control Plane StartMCP 后用官方 MCP client 连接并 read 文件。
5. 两 workspace 同时启动，验证跨 workspace 读取仍被拒绝。
6. unknown RuntimeID 明确失败。

## Exit Boundary

Phase 7 完成后，上层已经可以通过一个 Service 使用现有 Core，但还没有用户可执行命令。Phase 8 才添加 `cmd/ai-dev-manager`。
