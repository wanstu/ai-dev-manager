# ADM v1.1 快速开始

本页只给最短可用路径。完整功能说明见 [USER_GUIDE.md](USER_GUIDE.md)。

## 1. 获取发布产物

GitHub Release v1.2.0 提供：

```text
adm-v1.2.0-windows-amd64.exe
adm-v1.2.0-linux-amd64
adm-v1.2.0-darwin-amd64
adm-desktop-v1.2.0-windows-amd64.exe
SHA256SUMS-v1.2.0.txt
```

Windows 示例：

```powershell
$adm = 'C:\tools\adm\adm-v1.2.0-windows-amd64.exe'
& $adm -h
& $adm doctor
```

如果从源码构建 CLI：

```powershell
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -trimpath -o dist\adm-windows-amd64.exe ./cmd/ai-dev-manager
$adm = '.\dist\adm-windows-amd64.exe'
```

## 2. 启动 Gateway

Gateway 是真正运行的 MCP/HTTP 服务。Workspace 和 Environment 是持久记录，不是后台服务。

```powershell
& $adm gateway start --detach
& $adm gateway status
```

默认地址：

```text
Base URL   http://127.0.0.1:43137
Agent MCP  http://127.0.0.1:43137/mcp
Admin MCP  http://127.0.0.1:43137/admin/mcp
Health     http://127.0.0.1:43137/healthz
```

选择另一套 ADM：

```powershell
& $adm --adm-url http://127.0.0.1:8001 workspace list
```

或者：

```powershell
$env:ADM_V2_URL = 'http://127.0.0.1:8001'
& $adm workspace list
```

也可以把常用连接写入 `~/.config/adm/.env`：

```dotenv
ADM_V2_URL=http://127.0.0.1:8001
# 远程管理连接时再配置：
# ADM_V2_ADMIN_API_KEY=你的AdminKey
```

CLI/Desktop 还会读取可执行文件同目录 `.env`，优先级为：**进程环境 > 应用目录 `.env` > `~/.config/adm/.env`**。

正常管理连接失败不会 fallback 到本地 writable state。

## 3. 登记 Workspace

```powershell
& $adm workspace add --path D:\projects --name projects
& $adm workspace list
```

Workspace 是 ADM 被允许使用的现有目录。Git 不是前置条件。

如果 Workspace 很大，可以先发现项目：

```powershell
& $adm workspace discover --workspace-id WS_ID --query project-a
```

Discovery 是 bounded metadata-only，不读文件正文、不自动创建 Environment。

## 4. 创建 Environment

使用整个 Workspace：

```powershell
& $adm environment create --workspace-id WS_ID --name main
```

只授权一个项目子目录：

```powershell
& $adm environment create `
  --workspace-id WS_ID `
  --name project-a `
  --root D:\projects\project-a
```

查看：

```powershell
& $adm environment inspect --environment-id ENV_ID
& $adm environment capability-report --environment-id ENV_ID
& $adm environment context --environment-id ENV_ID
```

`environment context` 是 Agent onboarding 推荐入口：返回 bounded root/tree/capability/MCP/Skill/verifier summary，但不会自动读取 Memory values/完整 Skill instructions，也不会主动执行东西。

## 5. 允许常用 executable

Agent 不能任意启动本机程序，先配置 allowlist：

```powershell
& $adm exec allow --executable git
& $adm exec allow --executable go
& $adm exec allow --executable node
& $adm exec list
```

只允许你实际需要的 executable。执行时仍然需要 Environment + matching Writer + cwd containment。

## 6. 可选：配置 MCP

HTTP MCP：

```powershell
& $adm mcp add `
  --name local-tools `
  --transport streamable-http `
  --endpoint http://127.0.0.1:9000/mcp
```

启用到已有 Environment：

```powershell
& $adm environment mcp enable --environment-id ENV_ID --mcp-id MCP_ID
```

诊断：

```powershell
& $adm mcp probe --id MCP_ID
& $adm mcp inspect --id MCP_ID --environment-id ENV_ID
& $adm mcp refresh --id MCP_ID --environment-id ENV_ID
```

`probe` 是 global transient；`inspect` 是 passive；`refresh` 才主动 reconnect/Ping/inventory refresh。

## 7. 可选：配置 Skill

推荐 source 生命周期：

```powershell
& $adm skill source-add `
  --root C:\Users\you\.config\opencode\skills `
  --support-root C:\Users\you\.config\opencode\shared

& $adm skill source-list
& $adm skill source-refresh --id SKILL_SOURCE_ID
& $adm skill list
& $adm skill availability
```

启用到 Environment：

```powershell
& $adm environment skill enable --environment-id ENV_ID --skill-id SKILL_ID
& $adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID
```

Skill source config update 不会隐式 refresh。

## 8. Agent 连接

把 MCP client 指向：

```text
http://127.0.0.1:43137/mcp
```

推荐 Agent 工作流：

```text
environment_context_bundle
-> tree/read/search
-> environment_writer_acquire
-> write/edit/delete/exec/verifier/process/run
-> environment_writer_release
```

ADM 不存在隐藏 current Environment；Agent 应始终显式传 `environment_id`。

## 9. Desktop

Windows 可以直接使用 release 中的：

```text
adm-desktop-v1.2.0-windows-amd64.exe
```

Desktop 通过 `/admin/mcp` 管理所选 ADM，不会连接失败后 direct-write `state.json`。

如果从源码构建，必须使用 Wails build script：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop.ps1 -clean -trimpath
```

不要把普通 `go build ./cmd/ai-dev-manager-desktop` 当作可运行 Desktop artifact。

## 10. 下一步

- [完整用户手册](USER_GUIDE.md)
- [CLI 完整参考](cli.md)
- [Agent Gateway / MCP tools](AGENT_GATEWAY.md)
- [工作机制与架构](ARCHITECTURE.md)
- [Desktop 管理](desktop.md)
