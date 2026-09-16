# `adm-desktop`（v1.1）

`adm-desktop` 是 ADM 的 Wails 桌面管理端。它是**人类管理 surface**，不是第二套 Core，也不是 Agent Gateway 的替代品。

Desktop 通过所选 ADM Base URL 的 `/admin/mcp` 读取和修改同一套 Workspace、Environment、exec allowlist、MCP、Skill、Memory、retention 等数据。连接失败不会直接读写本地 `state.json`。

完整产品用法见 [USER_GUIDE.md](USER_GUIDE.md)，内部机制见 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 1. 启动与发布产物

v1.1 Windows Release：

```text
adm-desktop-v1.2.0-windows-amd64.exe
```

源码构建必须通过 Wails：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop.ps1 -clean -trimpath
```

默认输出：

```text
dist\adm-desktop-windows-amd64.exe
```

不要使用：

```text
go build ./cmd/ai-dev-manager-desktop
```

作为可运行 Desktop artifact；普通 `go build` 缺少 Wails runtime 所需 build tags/assets pipeline。

## 2. Desktop 与 Gateway 的关系

Desktop 不是 Gateway 本身。

```text
adm-desktop
   |
   +-- GET /healthz
   |
   +-- /admin/mcp  -> ADM management

Agent
   |
   +-- /mcp        -> development tools
```

Desktop 可以帮助启动/停止**本机 loopback** Gateway，但管理数据仍来自 Admin MCP。

## 3. Connection Profiles

Desktop 支持保存多个 ADM connection profile：

```text
~/.config/adm/desktop-connections.json
```

一个 profile 主要定义 ADM Base URL 和相关本地启动偏好。

常见 URL：

```text
http://127.0.0.1:43137
http://127.0.0.1:8001
```

切换 profile 时 Desktop 会清理旧 scope UI 数据，并使用 connection generation/stale-response guard，避免旧连接的慢请求回来后覆盖新连接页面。

## 4. Dashboard / Connection 状态

Dashboard 用于快速看：

- 当前 ADM connection；
- Gateway running/stopped/incompatible；
- Workspace/Environment 概况；
- MCP/Skill/Runtime 重要状态；
- 当前选择的 Workspace/Environment context。

Desktop 会先检查 `/healthz`，再建立 `/admin/mcp` management backend。

如果连接失败，页面应显示 disconnected/error，而不是假装读取本机 state。

## 5. Workspace 管理

Desktop 可以：

- 列出 Workspace；
- 查看 Workspace；
- 添加现有目录；
- rename；
- remove registration；
- 对大 Workspace 执行 bounded discovery；
- 从 discovery 结果显式选择 Environment root。

Workspace add/remove 都是 metadata operation：remove 不删除真实目录；有 Environment 引用时会被 Core 拒绝。

Discovery 是 metadata-only，不读项目正文，不自动创建 Environment。

## 6. Environment 管理

Desktop 可以展示和管理：

- stable Environment ID；
- Workspace relation；
- root；
- Writer；
- capability facts；
- MCP/Skill selections；
- private Memory count；
- retention：durable/temporary；
- temporary owner/expiry/provenance；
- cleanup blockers。

rename/remove 继续遵循 Core 规则：不移动/删除项目目录。

## 7. Temporary Environment

Desktop v1.1 可以对已有 temporary Environment 做生命周期管理：

- status；
- promote；
- cleanup preview；
- confirm execute cleanup。

关键行为：

- preview 是 read-only；
- execute 前服务端 fresh safety recheck；
- UI 不暴露 temporary cleanup force；
- ordinary root cleanup 不删项目目录；
- managed-worktree cleanup 保留 generated branch；
- active Writer/process/run/vfrun/MCP、dirty/unpublished worktree 等会阻止 cleanup。

## 8. Runtime / Diagnostics

Environment diagnostics 区域可展示：

- capability report；
- Writer facts；
- verifier definitions；
- process/run observations；
- managed worktree facts；
- unavailable capability reason。

这些视图应区分：

- persisted desired state；
- 当前 Gateway owner runtime observation。

Gateway restart 后 owner-local process/run/MCP observation 重新开始，不应被 Desktop 当成 persistent history。

## 9. Exec Allowlist 与 Denial Observations

Desktop 可以管理允许执行的 executable，并显示 Runtime 最近拒绝的 executable observations。

Allowlist 决定 exec/process/run/verifier/stdio MCP 等本机 executable 是否可以启动。

Denial observation 只保留轻量信息，例如：

- executable；
- denial count；
- first/last blocked time；
- last Environment/surface；
- reason。

不会保存 command args/stdout/stderr。

管理员可以根据这些事实决定是否 allow，而不是让 Agent 自己扩权。

## 10. MCP 页面

Desktop MCP 管理包括：

- global MCP definitions；
- HTTP / stdio config；
- default include；
- import preview/apply；
- global probe；
- Environment selection；
- Environment owner-runtime inspect/refresh；
- health/diagnostic badges；
- text 或 `/pattern/flags` 搜索；
- filtered bulk default/Environment enable actions。

### Global probe 与 Environment observation

Desktop 的 global probe 不依赖当前 Environment；切换 Environment 不应使同一个 unchanged definition 的 global probe result 失效。

Definition 或 connection 改变时旧结果应视为 stale。

### Secret 显示

MCP secret 以 reference requirement 方式管理。UI 不应该显示/持久化原始 credential literal。

## 11. Skill / Skill Sources 页面

Desktop Skill 管理区分：

- Skills catalog；
- Skill Sources；
- global structural availability；
- Environment-specific enablement/availability。

Source 可以：

- add；
- edit root/support roots/default；
- explicit refresh；
- remove。

编辑 source **不会隐式 refresh**。

Skill/Sources 页面切换和 filter 是 UI state，不应读取 Skill instructions、执行 Skill 或产生 Agent prompt injection。

## 12. Memory 页面

Desktop 明确区分：

- Global Memory；
- 当前 Environment private Memory。

进入/刷新 Memory 页面时可以显式加载 human management value，但 management snapshot/Environment summary 不会把 private values 混进去。

写入/删除永远指定 scope；不会自动 Global <-> Environment promote/sync。

## 13. Gateway / System 管理

只有**本机 loopback HTTP root URL**允许 Desktop 使用本地 start/stop lifecycle controls。

Desktop 不会通过远端 URL 停止服务器，也不会为了“修复连接”杀死未知监听进程。

Start 只有在 `/healthz` ready 后才报告成功。

## 14. 托盘与关闭行为（Windows）

Windows：

- 关闭主窗口默认隐藏到 tray，不直接退出整个 app；
- tray 可显示/隐藏窗口；
- tray 可切换 autostart；
- tray `退出` 才真正终止 Desktop；
- `--autostart` 启动时默认隐藏。

Autostart 使用 HKCU Run：

```text
value name: adm-desktop
command: <current desktop exe> --autostart
```

## 15. “启动 ADM 服务” Profile 选项

连接 profile 可以配置 Desktop 启动时尝试启动对应本机 ADM service/Gateway。该能力只适用于明确的 loopback local profile，不应变成远端 process control。

## 16. 图标与 Wails assets

- `assets/icons/ai-dev-manager-app.png`：exe/app icon source；
- `assets/icons/ai-dev-manager-tray.png`：tray source；
- `assets/icons/ai-dev-manager-window.png`：window branding source。

Wails frontend build 会执行 icon preparation script，以处理 Windows tray transparent padding/尺寸。

## 17. Desktop 不做什么

Desktop 不应该：

- 直接修改 `state.json` 作为正常 mode；
- 建立第二套 Workspace/Environment/MCP/Skill 数据；
- 绕过 Admin MCP validation；
- 为连接失败 silently fallback；
- 给 Agent 自动开放 executable；
- 自动删除 temporary resources；
- 自动调用 MCP business tools；
- 自动读 Skill instructions；
- 自动把 Memory 注入 Agent prompt；
- 替代 `/mcp` Agent development surface。

## 18. 相关文档

- [完整用户手册](USER_GUIDE.md)
- [工作机制与架构](ARCHITECTURE.md)
- [CLI 参考](cli.md)
- [MCP / Skill / Memory](catalog-memory.md)
- [打包与发布](packaging.md)
