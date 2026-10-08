# ADM 加密凭据与 MCP 密钥库

## 配置方式（Desktop / Web 管理页面）

1. 打开 **MCP 管理 → 加密密钥库**，输入密钥名称（例：`github-token`）及 Token，选择**加密保存**。
2. 新建或修改 MCP：选 HTTP → Header Auth，将 Header 写为 `Authorization=Bearer ${secret:github-token}`；或在 stdio 的环境引用中写入 `TOKEN=${secret:github-token}`。
3. 在 MCP 界面刷新/探测配置，确认连接。仅管理员可以新增、替换、列出名称或删除密钥。密钥内容**永不通过列表或普通配置读回**。密钥更新会使使用它的 MCP 长连接失效，下次连接使用新值。
4. 未配置、名称拼写错误或密钥损坏，MCP 显示 `unresolved_secret_reference` 而不是默默发送空凭据。

## 兼容性与数据存储

- 旧式 `${MCP_TOKEN}` 环境变量引用继续有效，但新配置优先使用 ADM 自有密钥库。
- 凭据加密保存于状态文件旁边的 `state.json.secrets.json`；普通 `state.json` 的 MCP 定义仅保存引用名称。
- **Windows**：用当前用户的 DPAPI 加密密文，没有另存的应用主密钥。不同 Windows 用户/账号无法直接解密。备份/迁移到其他账号前应重新输入凭据。
- **Linux 等非 Windows 系统**：使用 AES-256-GCM 加密，随机主密钥另存于 `state.json.secrets.json.key`（权限 0600）。两文件应一起备份并严控文件和目录权限；这只是静态加密，**无法抵御已取得服务进程账号或 root 权限的人**。
- **Desktop 已存的管理连接 API Key**：新版本首次读取旧配置时，会迁移明文到连接配置路径对应的加密密钥库，随后重写连接配置以删除明文字段。已有备份、副本、日志和系统快照不会自动清除，曾以明文保存过的关键 Token 建议主动轮换。
- 加密不能替代最小权限。运行时 MCP 子进程/HTTP 传输仍会使用解密后的密钥；请始终使用 HTTPS 访问远程 ADM Admin 管理面。**如果把 Agent 与 Admin 运行在同一个拥有系统高权限的账号下，并给予 Agent 不受限的本机命令执行能力，DPAPI/文件加密都不能提供进程级的完全隔离。** 严格安全边界需要独立服务账号与主机 ACL。
- Secret 管理入口仅在 Admin MCP / 管理页面。Agent MCP 不提供 `secret_set`、`secret_list` 或 `secret_delete`。涉及凭据的 MCP 配置应使用引用，不应把明文放进 URL、Git 仓库、命令参数、普通 Memory 或聊天记录。

## 版本迁移与后续事项

这是新能力，不会强制更改用户当前 MCP 环境变量配置，也不会迁移/轮换现有 Token。
管理入口首次登录需通过与旧版一样的 Admin 访问控制；如连接的是旧版 ADM Gateway，会提示不支持密钥库。
