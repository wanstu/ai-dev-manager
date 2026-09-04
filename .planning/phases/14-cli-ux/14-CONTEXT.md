# Phase 14 Context — Daily CLI UX

## Goal

把 v0.3/v0.3.1 的底层 lifecycle 命令包装成日常可用入口，减少用户手工维护 config-root、Workspace ID 与 daemon/runtime 启动顺序的负担。

## Locked Decisions

1. 保留现有 `start/status/stop/workspace/runtime/...` 命令兼容性；新命令是高层 convenience layer。
2. `up [target]`：target 可是 Workspace ID 或目录路径；省略时默认当前目录。未注册目录自动注册，daemon 未运行自动启动，然后启动 Runtime。
3. `up --docker` 复用 Phase 13 Docker exposure；`up --listen` 复用显式 listen。
4. `down [target]`：解析 Workspace 后执行显式 runtime stop，因此 desired state 变为 stopped；不停止 daemon。
5. `ps`：列出所有已注册 Workspace，并合并 daemon observed runtime 状态；daemon 未运行时也能显示 Workspace 为 stopped/unobserved。
6. `ctl start|status|stop|restart|shutdown`：
   - stop：停止 daemon，但保留 desired runtime，未来 start 会恢复。
   - restart：stop + start，保留 desired runtime。
   - shutdown：显式停止全部 desired runtimes，再停止 daemon；未来 start 不自动恢复。
7. `ctl` 管理的 daemon Control API 仍 loopback-only。
8. 不引入第二个 executable（例如 admctl）；保持单个 `ai-dev-manager.exe`。

## UX Target

```text
ai-dev-manager up --docker D:\projects\foo
ai-dev-manager ps
ai-dev-manager down D:\projects\foo
ai-dev-manager ctl restart
ai-dev-manager ctl shutdown
```

## Acceptance

- unregistered path `up` one-shot flow。
- existing path second `up` reuses Workspace ID。
- `down` by path works without Workspace ID。
- `ps` works daemon running/stopped。
- `ctl restart` preserves desired runtime。
- `ctl shutdown` clears desired runtime and leaves daemon stopped。
- old lifecycle commands remain green。
