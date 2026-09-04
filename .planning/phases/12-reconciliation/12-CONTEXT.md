# Phase 12 Context — Restart Reconciliation & Lifecycle Acceptance

## Milestone

v0.3 — Persistent Control Plane / Runtime Lifecycle

## Goal

完成 v0.3 最后一个 vertical slice：把 Phase 11 daemon 内存中的 `desired_running` 提升为最小可持久化状态，并在 daemon clean restart / crash restart 后重新构造 Workspace Runtime、MCP Host 与 configured MCP sessions。任何旧 listener、endpoint、ClientSession 或 observed runtime state 都视为失效，绝不从磁盘“恢复”。

## Why This Phase Exists

Phase 10/11 已验证：

```text
CLI exits
  ↓
daemon remains
  ↓
Workspace Runtime / MCP Host / configured MCP session remain owned
```

但 daemon 本身退出后，RuntimeOwner.entries、host.Manager.instances 与 Activator.sessions 都会消失。Phase 12 要把“用户希望哪些 Workspace 持续运行”与“当前进程真实观察到什么”分开：

```text
Persisted Desired State
  workspace A = running
  workspace B = running
          ↓ daemon start
Fresh Control Plane / RuntimeOwner
          ↓ reconcile
new Runtime A + new MCP endpoint/session
new Runtime B + new MCP endpoint/session
```

## Locked Decisions

1. 持久化对象只包含最小 desired state：当前需要运行的 Workspace ID 集合（schema/version 可最小化）。
2. 不持久化 `RuntimeStatus` observed fields、MCP Host endpoint、listener address、PID、ClientSession、tool probe session 或 Go object。
3. `runtime start` 先把 Workspace 标记为 desired-running，再尝试建立 observed runtime；如果启动失败，desired=true 保留，observed=error，允许未来 reconcile 重试。
4. `runtime stop` 先把 Workspace 从 desired-running 中移除，再关闭 Host/session，避免停止过程中 daemon crash 后错误重启该 Workspace。
5. daemon startup 在发布 healthy discovery metadata 前加载 desired state 并执行 reconcile；单个 Workspace reconcile 失败记录为 RuntimeError，但不应让整个 daemon 因一个项目失败而退出。
6. desired state 文件损坏不能静默当作 empty；应返回明确错误，避免无声丢失用户运行意图。
7. clean daemon `stop` 只停止 observed runtime，不清除 desired-running；随后再次 `start` 应恢复之前需要运行的 Workspace。只有显式 `runtime stop` 改变 desired state。
8. crash 后残留的 `daemon.json` / lease 不能永久阻止新 owner。新的 `start` 必须等待当前 owner变 healthy、lease 被释放，或 heartbeat 变 stale；一旦 stale，安全清理并启动新 daemon，而不是单纯等待 readiness timeout 后失败。
9. daemon metadata/status 永远只表示当前 daemon instance；restart 后必须生成新 instance ID/PID。旧 runtime endpoint 不得被新 status 复用或误报 healthy。
10. 同一 Workspace 的 reconciliation 仍复用 Phase 11 RuntimeOwner/Control Plane 路径，不能建立第二套恢复启动逻辑。
11. v0.3 仍不实现通用 service restart policy、Process Manager、Agent recovery、Docker 或 remote supervisor。

## Persistent Shape

建议 config-root app-owned runtime area：

```text
<config-root>/runtime/
  daemon.json              # observed daemon discovery; ephemeral
  daemon.lock              # observed owner lease; ephemeral
  desired-runtimes.json    # persisted user desired state
```

`desired-runtimes.json` 仅需表达：

```json
{
  "version": 1,
  "workspace_ids": ["ws_...", "ws_..."]
}
```

列表必须 deterministic/sorted，保存原子化。

## Restart Semantics

### Clean restart

```text
runtime start A
runtime start B
stop daemon
  → observed hosts/sessions gone
  → desired A/B remain
start daemon
  → reconcile A/B
  → new endpoints
```

### Crash restart

```text
runtime start A
kill daemon process
  → daemon.json + lease may remain
  → old endpoint dead
start daemon
  → wait for lease stale / recover metadata
  → new daemon owner
  → load desired A
  → reconcile A
  → new endpoint, running
```

## Verification Strategy

1. Desired store round-trip, deterministic sorting, atomic replacement, corrupt/version handling。
2. RuntimeOwner start/stop persistence semantics; activation failure keeps desired=true。
3. clean daemon restart test: A/B desired survive, endpoints are rebuilt and remain isolated。
4. crash test: kill daemon child without graceful cleanup; immediate restart eventually succeeds after stale lease recovery within bounded timeout。
5. after crash and before recovery, old runtime endpoint is unreachable; after restart status reports a fresh endpoint, never the old one。
6. explicit `runtime stop A` survives daemon restart: A remains stopped while B is reconciled running。
7. full `go test ./...`, `go vet ./...`, build and real Windows binary acceptance。

## Exit Boundary

Phase 12 success completes v0.3. Because milestone boundary is reached, `workflow.auto_advance=true` must stop before v0.4 Agents/GSD implementation. Update PROJECT/ROADMAP/STATE and leave v0.4 planning as the next explicit milestone decision/work item.
