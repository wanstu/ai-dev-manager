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

例如：

```powershell
$env:ADM_V2_ADMIN_API_KEY = "admin-替换成你的长随机密钥"
$env:ADM_V2_AGENT_API_KEY = "agent-替换成另一把长随机密钥"

.\adm.exe gateway access set-admin-key
.\adm.exe gateway access set-agent-key
.\adm.exe gateway access set-hosts --hosts "101.37.171.174"

.\adm.exe gateway start --listen 0.0.0.0:43137 -d
```

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

推荐使用环境变量，避免把 Key 留在 shell history：

```powershell
$env:ADM_V2_ADMIN_API_KEY = "你的 Admin Key"
.\adm.exe gateway access set-admin-key
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

推荐：

```powershell
$env:ADM_V2_AGENT_API_KEY = "你的 Agent Key"
.\adm.exe gateway access set-agent-key
```

也可以：

```powershell
.\adm.exe gateway access set-agent-key --key "你的 Agent Key"
```

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
$env:ADM_V2_ADMIN_API_KEY = "你的 Admin Key"
$env:ADM_V2_AGENT_API_KEY = "你的 Agent Key"

.\adm.exe gateway access set-admin-key
.\adm.exe gateway access set-agent-key
.\adm.exe gateway access set-hosts --hosts "101.37.171.174,adm.example.com"

.\adm.exe gateway access status
.\adm.exe gateway start --listen 0.0.0.0:43137 -d
```

### Host 不固定

```powershell
$env:ADM_V2_ADMIN_API_KEY = "你的 Admin Key"
$env:ADM_V2_AGENT_API_KEY = "你的 Agent Key"

.\adm.exe gateway access set-admin-key
.\adm.exe gateway access set-agent-key
.\adm.exe gateway access set-hosts --hosts "*"

.\adm.exe gateway start --listen 0.0.0.0:43137 -d
```

---

## 11. 远程 ADM CLI 怎么连接

ADM CLI 使用的是 `/admin/mcp`，所以只需要 **Admin Key**。

客户端 PowerShell：

```powershell
$env:ADM_V2_URL = "http://101.37.171.174:43137"
$env:ADM_V2_ADMIN_API_KEY = "服务端配置的 Admin Key"

.\adm.exe workspace list
.\adm.exe environment list
.\adm.exe mcp list
```

也可以只给某条命令指定 URL：

```powershell
$env:ADM_V2_ADMIN_API_KEY = "服务端配置的 Admin Key"

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

CLI **不会使用 Agent Key** 连接管理面。

---

## 12. Desktop 怎么连接远程 ADM

Desktop 也是管理面，因此连接 profile 只保存 **Admin API Key**。

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
$env:ADM_V2_ADMIN_API_KEY = "..."
$env:ADM_V2_AGENT_API_KEY = "..."

.\adm.exe gateway access set-admin-key
.\adm.exe gateway access set-agent-key
.\adm.exe gateway access set-hosts --hosts "*"
.\adm.exe gateway start --listen 0.0.0.0:43137 -d
```

---

## 18. 安全建议

如果使用 `*`：

- Host 检查被放宽。
- Admin/Agent API Key 仍是必要安全边界。
- 建议同时通过云安全组、防火墙或 VPN 限制 43137/反代端口。
- Admin Key 权限高于 Agent Key，应单独保存。
- 不要把 Admin Key 提供给普通 Agent。
- 两把 Key 都不要直接提交到 Git。
