# ADM 远程访问、Host 白名单与双 API Key

ADM 的远程 HTTP Gateway 同时暴露两个 MCP surface：

- `/admin/mcp`：人类管理面，供 Desktop 和 ADM CLI 使用。
- `/mcp`：Agent 开发面，供 ChatGPT/Codex/其他 MCP Agent 使用。

这两个 surface 使用**不同的 API Key**：

- **Admin API Key**：只能认证 `/admin/mcp`，也用于远程 shutdown 管理请求。
- **Agent API Key**：只能认证 `/mcp`。
- Admin Key 不能拿去访问 `/mcp`。
- Agent Key 不能拿去访问 `/admin/mcp`。

两类请求都可以使用同一个 Header 名：

```text
X-ADM-API-Key: <对应 surface 的 Key>
```

也支持：

```text
Authorization: Bearer <对应 surface 的 Key>
```

服务端根据请求 URL 决定应该验证 Admin Key 还是 Agent Key。

---

## 1. 先理解三个不同概念

### 监听地址

```text
--listen 0.0.0.0:43137
```

表示 ADM 在所有网卡上监听。

它**不表示白名单允许所有 Host**。

### Host/IP 白名单

```text
adm gateway access set-hosts --hosts "101.37.171.174,adm.example.com"
```

表示允许这些 HTTP Host 进入远程 Gateway。

端口不用写。

### Host 通配符

```text
*
```

是 ADM 明确定义的 Host 通配符，表示：

> 不限制 Host/IP，但 Admin Key 和 Agent Key 鉴权仍然强制。

因此：

```text
0.0.0.0
```

不是白名单通配符。

真正的“不限制 Host”必须配置：

```text
*
```

---

## 2. 远程 Gateway 的启用条件

ADM 允许监听非 loopback 地址之前，必须同时具备：

1. 至少一个 Host/IP 白名单项，或者 `*`
2. Admin API Key
3. Agent API Key

推荐直接使用部署工作流：

```powershell
.\adm.exe gateway setup --remote --listen 0.0.0.0:8001 --hosts "101.37.171.174"
.\adm.exe gateway start --listen 0.0.0.0:8001 -d
```

首次 setup 会自动生成缺失的 Admin/Agent Key，服务端只保存 hash，明文只在本次输出显示。再次执行 setup 默认同时保留已有 Host policy 和双 Key；只有显式 `--hosts` 才替换白名单，显式 `--rotate-keys` 才轮换 Key。

Linux/systemd 部署更推荐直接使用一键工作流：

```bash
sudo adm gateway install --remote --user admin --port 8001 --hosts '*'
```

首次安装需要 `--user`。后续升级 ADM 二进制后建议先查看计划，再重复执行：

```bash
sudo adm gateway install --remote --dry-run
sudo adm gateway install --remote
```

`--dry-run` 不写 state、不生成或轮换 Key，也不会修改/restart systemd unit；输出会明确 Key 的 preserve/generate/rotate 动作、`service_enabled` 和将发生的 service 配置变化。

如果现有 unit 是 ADM managed service，省略 user/listen/hosts 会保留原值，双 Key 与 systemd Enabled 状态也默认保留；显式 `--rotate-keys` 才轮换 Key，需要改变开机启动时显式使用 `gateway service enable/disable`。ADM 不会覆盖没有 managed marker 的同名 systemd unit。升级中的 unit/ready 失败会尝试恢复旧 service 配置与原 Gateway access 状态。

常规生命周期不需要再手工调用 `systemctl`：`gateway service start/stop/restart/enable/disable` 已覆盖这些操作。start/restart 会先检查 unit 指向的 Gateway state 是否满足远程启动条件；所有变更命令只允许操作 ADM managed unit。


如果缺少其中任意一项，ADM 会拒绝远程监听。

---

## 3. 生成两把 Key

Windows PowerShell 示例：

```powershell
$adminKey = "adm-admin-" + [guid]::NewGuid().ToString("N")
$agentKey = "adm-agent-" + [guid]::NewGuid().ToString("N")

$adminKey
$agentKey
```

请分别保存。

不要使用同一个值作为 Admin Key 和 Agent Key。

ADM 服务端持久化的是 Key 的 SHA-256 hash，不会通过状态接口回显原始 Key。

每把 Key 至少需要 16 个字符。

---

## 4. CLI：设置 Admin Key

普通初始化建议使用 `gateway setup --remote`。如确实需要手工指定服务端 Admin Key，必须显式传入；服务端不再从 `.env` 隐式读取：

```powershell
.\adm.exe gateway access set-admin-key --key "你的 Admin Key"
```

也可以显式传：

```powershell
.\adm.exe gateway access set-admin-key --key "你的 Admin Key"
```

清除：

```powershell
.\adm.exe gateway access clear-admin-key
```

---

## 5. CLI：设置 Agent Key

普通初始化建议由 `gateway setup --remote` 自动生成。主动换 Key 时优先使用：

```powershell
.\adm.exe gateway access rotate-agent-key
```

如确实需要手工指定服务端 Agent Key，必须显式传入：

```powershell
.\adm.exe gateway access set-agent-key --key "你的 Agent Key"
```

服务端不会从 `.env` 隐式读取 Agent Key。

清除：

```powershell
.\adm.exe gateway access clear-agent-key
```

---

## 6. CLI：查看当前远程访问状态

```powershell
.\adm.exe gateway access status
```

状态只显示：

- Host 白名单
- Admin Key 是否已配置
- Agent Key 是否已配置

不会返回原始 Key。

类似：

```json
{
  "allowed_hosts": [
    "101.37.171.174"
  ],
  "admin_api_key_configured": true,
  "agent_api_key_configured": true
}
```

---

## 7. 严格 Host/IP 白名单

固定服务器 IP：

```powershell
.\adm.exe gateway access set-hosts --hosts "101.37.171.174"
```

同时允许域名：

```powershell
.\adm.exe gateway access set-hosts --hosts "101.37.171.174,adm.example.com"
```

如果客户端访问：

```text
http://101.37.171.174:43137
```

白名单只配置：

```text
101.37.171.174
```

不要写端口。

---

## 8. 不限制 Host/IP，只依赖双 Key

如果服务器有动态域名、多层反代或 Host 不固定：

```powershell
.\adm.exe gateway access set-hosts --hosts "*"
```

此时：

- 任意 Host 可以进入认证阶段。
- `/admin/mcp` 仍必须提供正确 Admin Key。
- `/mcp` 仍必须提供正确 Agent Key。
- 两把 Key 不能互换。

如果白名单同时包含：

```text
101.37.171.174
*
adm.example.com
```

ADM 会规范化为：

```text
*
```

---

## 9. 为什么 0.0.0.0 不是“不限制”

下面是**监听配置**：

```powershell
.\adm.exe gateway start --listen 0.0.0.0:43137 -d
```

意思是：

> 在本机所有网卡的 43137 端口监听。

下面是**访问白名单配置**：

```powershell
.\adm.exe gateway access set-hosts --hosts "0.0.0.0"
```

意思只是：

> 允许 HTTP Host 字面值为 0.0.0.0。

它不会匹配：

```text
101.37.171.174
adm.example.com
other.example.net
```

如果你要“不限制 Host”，使用：

```powershell
.\adm.exe gateway access set-hosts --hosts "*"
```

---

## 10. 推荐服务端完整配置

### 固定公网 IP / 域名

```powershell
.\adm.exe gateway setup --remote --listen 0.0.0.0:8001 --hosts "101.37.171.174,adm.example.com"
.\adm.exe gateway start --listen 0.0.0.0:8001 -d
```

### Host 不固定

```powershell
.\adm.exe gateway setup --remote --listen 0.0.0.0:8001 --hosts "*"
.\adm.exe gateway start --listen 0.0.0.0:8001 -d
```

已有 Key 默认保留；显式轮换：

```powershell
.\adm.exe gateway access rotate-admin-key
.\adm.exe gateway access rotate-agent-key
# 或同时轮换
.\adm.exe gateway access rotate --all
```

---

## 11. 远程 ADM CLI 怎么连接

ADM CLI 使用的是 `/admin/mcp`，所以只需要 **Admin Key**。这里使用的是**客户端 Admin Key**，值必须与远端 ADM 服务端 Remote access 中已经配置的 Admin Key 完全一致。

ADM CLI 会读取 `~/.config/adm/.env` 和可执行文件同目录 `.env`；优先级是 **进程环境 > 应用目录 `.env` > 用户级 `.env`**。`.env` 只用于客户端连接凭据，不再决定服务端保存的 Key hash。客户端 Admin Key 使用 `ADM_ADMIN_API_KEY`；旧 `ADM_V2_ADMIN_API_KEY` 暂时兼容读取。因此可以先写用户级配置：

```dotenv
ADM_V2_URL=http://101.37.171.174:43137
ADM_ADMIN_API_KEY=服务端配置的AdminKey
```

也可以直接使用客户端 PowerShell：

```powershell
$env:ADM_V2_URL = "http://101.37.171.174:43137"
$env:ADM_ADMIN_API_KEY = "服务端配置的 Admin Key"

.\adm.exe workspace list
.\adm.exe environment list
.\adm.exe mcp list
```

也可以只给某条命令指定 URL：

```powershell
$env:ADM_ADMIN_API_KEY = "服务端配置的 Admin Key"

.\adm.exe --adm-url "http://101.37.171.174:43137" workspace list
```

CLI 会向：

```text
http://101.37.171.174:43137/admin/mcp
```

发送：

```text
X-ADM-API-Key: <Admin Key>
```

服务端同时接受标准 Bearer 形式：

```text
Authorization: Bearer <Admin Key>
```

二选一即可。CLI **不会使用 Agent Key** 连接管理面。

---

## 12. Desktop 怎么连接远程 ADM

Desktop 也是管理面，因此连接 profile 只保存 **Admin API Key**。它与 Remote access 页面里的“服务端 Admin Key”是同一份凭据的客户端副本，但存储语义不同：服务端只持久化 Key hash；Desktop connection profile 需要保存客户端原始 Key 才能继续发起认证请求，UI 不会回显原值。

连接配置：

```text
ADM Base URL:
http://101.37.171.174:43137

Admin API Key:
服务端配置的 Admin Key
```

Desktop 连接的是：

```text
/admin/mcp
```

所以 Agent Key 不需要写进 Desktop connection profile。

Desktop connection profile 保存在：

```text
~/.config/adm/desktop-connections.json
```

连接编辑弹窗会显示 Admin Key **已保存 / 未配置** 状态，但不会把原始 Key 回填到输入框。留空保存时保留原 Key，输入新值则替换。

在 Desktop 的“Remote access”设置区，可以分别管理：

- Host/IP 白名单
- Admin API Key
- Agent API Key

其中设置 Admin Key 时，Desktop 会同步更新当前 connection profile，避免 Admin Key 轮换后自己失去管理连接。

---

## 13. Agent MCP 怎么配置

Agent 连接：

```text
http://101.37.171.174:43137/mcp
```

必须提供 **Agent API Key**。

例如 MCP 客户端支持 Header 配置：

```json
{
  "url": "http://101.37.171.174:43137/mcp",
  "headers": {
    "X-ADM-API-Key": "${ADM_V2_AGENT_API_KEY}"
  }
}
```

环境变量：

```powershell
$env:ADM_V2_AGENT_API_KEY = "服务端配置的 Agent Key"
```

也可以使用：

```text
Authorization: Bearer <Agent Key>
```

不要在 Agent 配置中使用 Admin Key。

---

## 14. 两把 Key 互换会发生什么

假设：

```text
Admin Key = admin-secret-...
Agent Key = agent-secret-...
```

访问：

```text
/admin/mcp
```

使用：

```text
X-ADM-API-Key: agent-secret-...
```

结果：

```text
403 Forbidden
```

同样，访问：

```text
/mcp
```

但发送 Admin Key，也会返回 403。

---

## 15. Nginx 反向代理

例如外部入口：

```text
http://101.37.171.174:8221
```

Nginx：

```nginx
location / {
    proxy_pass http://127.0.0.1:43137;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

推荐保留原 Host：

```nginx
proxy_set_header Host $host;
```

如果使用严格白名单：

```powershell
.\adm.exe gateway access set-hosts --hosts "101.37.171.174"
```

ADM 看到的 Host 必须匹配。

如果 Nginx 场景下 Host 很复杂，可以使用：

```powershell
.\adm.exe gateway access set-hosts --hosts "*"
```

但这只放开 Host 校验，双 Key 认证仍然存在。

Nginx 不需要根据 URL 注入 Key；Key 应由真正的 Desktop/CLI/Agent 客户端提供并透传。

---

## 16. 本机访问规则

未配置远程 Host 白名单时：

- `localhost`
- `127.0.0.1`
- `::1`

可以继续使用本机直连兼容模式。

一旦配置了远程 Host 白名单：

- `/admin/mcp` 必须携带 Admin Key。
- `/mcp` 必须携带 Agent Key。
- 即使请求最终通过 Nginx 转发到 `127.0.0.1`，也不能绕过 Key。

这样可以防止反向代理把远程请求伪装成本地 Host 后绕过认证。

---

## 17. 配置顺序

推荐始终按这个顺序：

```text
1. 设置 Admin Key
2. 设置 Agent Key
3. 设置 Host 白名单
4. 启动 0.0.0.0 远程监听
```

这样可以避免先打开远程 Host 后，管理连接尚未准备好对应 Key。

PowerShell：

```powershell
.\adm.exe gateway setup --remote --listen 0.0.0.0:8001 --hosts "*"
.\adm.exe gateway start --listen 0.0.0.0:8001 -d
```

---

## 18. 远程诊断

已能连接 `/admin/mcp` 后，可直接读取服务端运维诊断：

```bash
adm --adm-url http://SERVER:8001 gateway diagnostics
```

Desktop 的 **ADM 连接 → Gateway 诊断** 显示同一类信息，并可一键复制诊断报告。内容包括运行用户、监听地址、实际 state/config 路径、远程访问 readiness 和 Linux systemd 状态；不会包含 Admin/Agent Key 原文。

这组信息只通过 Admin MCP 返回，不加入公开 `/healthz`。

---

## 19. 安全建议

如果使用 `*`：

- Host 检查被放宽。
- Admin/Agent API Key 仍是必要安全边界。
- 建议同时通过云安全组、防火墙或 VPN 限制 43137/反代端口。
- Admin Key 权限高于 Agent Key，应单独保存。
- 不要把 Admin Key 提供给普通 Agent。
- 两把 Key 都不要直接提交到 Git。
- `~/.config/adm/.env` 和 `~/.config/adm/desktop-connections.json` 可能包含客户端 Admin Key 原文；Linux/macOS 上应保持仅当前用户可读（例如 `chmod 600`）。服务端 Remote access 状态本身只保存 Key hash。
