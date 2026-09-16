# adm

ADM（AI Dev Manager）是本地 AI 开发 Gateway / 管理控制面。它让 Agent 只在你明确登记的目录中开发软件，并把文件访问、Writer、允许执行命令、MCP、Skill、Memory、Verifier、长期进程、异步 Run 和临时 Environment 放在同一套可检查的权限/生命周期模型里。

**Workspace 和 Environment 不是后台服务；真正运行的是 Gateway。Git 不是前置条件。**

当前稳定版本：**v1.2.0**。

## 最快开始

Windows 示例：

```powershell
$adm = 'C:\tools\adm\adm-v1.2.0-windows-amd64.exe'

& $adm gateway start --detach
& $adm workspace add --path D:\projects --name projects
& $adm environment create --workspace-id WS_ID --name main --root D:\projects\project-a
& $adm exec allow --executable git
& $adm exec allow --executable go
& $adm environment context --environment-id ENV_ID
```

Agent MCP 默认连接：

```text
http://127.0.0.1:43137/mcp
```

管理面：

```text
http://127.0.0.1:43137/admin/mcp
```

详细步骤见 [docs/quickstart.md](docs/quickstart.md)。

## 核心概念

- **Workspace**：显式登记给 ADM 的本地目录；可以完全不是 Git repo。
- **Environment**：位于 Workspace 内、以目录为 root 的持久开发上下文；保存 MCP/Skill selection、private Memory、Verifier、Writer/retention 等状态。
- **Gateway**：真正运行的 MCP/HTTP 服务。
- **Runtime Owner**：当前 Gateway 生命周期内持有 MCP session、process、Run、async verifier observation 的 owner。
- **Writer lease**：同一个 physical root 的单写者、有期限 mutation authority；默认 TTL 5 分钟。
- **Exec allowlist**：Agent/Runtime 只能启动显式允许的 executable。
- **MCP catalog**：全局 desired definitions；Environment 只保存启用的 stable MCP IDs。
- **Skill Source/catalog**：从显式 root 发现真实 `SKILL.md`；Environment 只保存启用的 Skill IDs。
- **Memory**：Global 与 Environment-private 两个显式 scope；不会自动合并。
- **Verifier**：Environment-scoped structured test/lint/build/custom 定义。
- **Process / Run / Verifier Run**：Gateway owner-local Runtime resources，不跨 Gateway restart 伪恢复。
- **Temporary Environment**：带 owner + TTL retention 的普通 `env_`；cleanup 默认 preview、fresh recheck、无 force。

## Desktop

Windows Desktop 发布名：

```text
adm-desktop-v1.2.0-windows-amd64.exe
```

Desktop 通过所选 ADM Base URL 的 `/admin/mcp` 管理 ADM；连接失败不会直接回退读写本机 `state.json`。

源码构建必须使用 Wails：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop.ps1 -clean -trimpath
```

不要把普通 `go build ./cmd/ai-dev-manager-desktop` 当成可运行 Desktop artifact。

Desktop 连接 profiles 位于：

```text
~/.config/adm/desktop-connections.json
```

详见 [docs/desktop.md](docs/desktop.md)。

## 从源码构建 CLI

```powershell
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -trimpath -o dist\adm-windows-amd64.exe ./cmd/ai-dev-manager
```

## 文档

完整文档入口：[docs/index.md](docs/index.md)

最重要的四份：

- [完整用户手册](docs/USER_GUIDE.md) — 每个功能如何使用，以及实际工作流。
- [CLI 完整参考](docs/cli.md) — v1.1 管理命令、flags、副作用边界。
- [Agent Gateway / MCP Tool 参考](docs/AGENT_GATEWAY.md) — Agent 工具参数、Writer 要求、Admin/Agent surface 区别。
- [工作机制与架构](docs/ARCHITECTURE.md) — Workspace/Environment/Writer/Exec/MCP/Skill/Memory/Runtime/Retention 的内部机制。

专题：

- [快速开始](docs/quickstart.md)
- [MCP、Skill 与 Memory](docs/catalog-memory.md)
- [Desktop 管理端](docs/desktop.md)
- [打包与 GitHub Actions](docs/packaging.md)
- [产品语义合同](docs/PRODUCT_CONTRACT.md)

## 产品边界

ADM 提供本机开发能力、授权、生命周期、路由和诊断；Agent/GSD/其他 orchestrator 负责任务规划和编排。

ADM 不把 Git/Docker/Verifier/MCP/Skill 变成普通开发的全局 prerequisite，也不内建 Planner/Executor/Reviewer、GSD phase advance、自动 merge/push 或 temporary auto-GC。
