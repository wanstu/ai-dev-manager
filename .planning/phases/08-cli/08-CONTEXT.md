# Phase 8 Context — CLI & Foreground MCP Serve

## Milestone

v0.2 — Usable Control Plane

## Goal

给 v0.1 Core + Phase 7 Control Plane 增加第一个真正的人类/脚本入口。CLI 只负责参数、输出和进程生命周期；Workspace resolve、Runtime composition、MCP host 必须继续走 `internal/controlplane`。

## Locked Decisions

1. 命令名：`ai-dev-manager`，入口位于 `cmd/ai-dev-manager`。
2. 不引入 Cobra 等 CLI 框架；当前命令规模使用标准库 `flag` 足够，避免增加非必要依赖。
3. 全局 `--config-root` 允许显式覆盖默认用户配置根，用于测试/隔离环境。
4. 全局 `--json` 提供稳定机器输出；默认文本输出保持简洁。
5. Workspace 第一版命令：`workspace add/list/show`。
6. Runtime 检查命令：`inspect --workspace <id>`，输出 Phase 7 Snapshot。
7. MCP 启动命令：`serve --workspace <id>`；前台阻塞，输出 endpoint，接收进程取消/信号后优雅停止。
8. v0.2 不做后台 daemon，因此不提供跨进程 `start/status/stop` 假象。
9. `serve` 的 listen 仍受 Host loopback-only policy；CLI 不绕过。
10. CLI 不 import `internal/config` resolver 或 `internal/runtime.Native` 来重新 composition。

## Command Surface

```text
ai-dev-manager [--config-root PATH] [--json] workspace add --path PATH [--profile ID] [--runtime ID]
ai-dev-manager [--config-root PATH] [--json] workspace list
ai-dev-manager [--config-root PATH] [--json] workspace show <workspace-id>
ai-dev-manager [--config-root PATH] [--json] inspect --workspace <workspace-id>
ai-dev-manager [--config-root PATH] [--json] serve --workspace <workspace-id> [--listen 127.0.0.1:0] [--instance ID]
```

Global flags are intentionally parsed before the command in v0.2.

## Output Contract

JSON mode:

- one JSON object/array for non-blocking commands。
- `serve` starts by writing one JSON line containing instance id/workspace/runtime/endpoint, then blocks until cancellation。

Text mode:

- add/show: concise workspace identity/path/runtime/profile。
- list: one workspace per line。
- inspect: readable JSON is acceptable for v0.2 rather than inventing a second rich formatter。
- serve: print endpoint prominently。

## Testability

`main` should delegate to a testable `run(ctx,args,stdout,stderr)` function. Signal handling belongs only in `main`; tests can cancel context deterministically.

`serve` integration test should:

1. create persisted workspace through CLI run path。
2. start CLI serve in goroutine with cancellable context。
3. read the first endpoint line。
4. connect official MCP client and call `read`。
5. cancel context and assert graceful return。

## Non-goals

- no interactive TUI。
- no config editing commands beyond Workspace registry fields。
- no daemon/service installation。
- no external configured MCP activation; Phase 9 owns that。
