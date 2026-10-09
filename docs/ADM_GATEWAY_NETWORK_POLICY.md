# ADM Gateway 网络访问策略（2026-10-08）

状态：**v1.4.0-rc.2 已于 2026-10-09 发布**（commit `9fbb520`，跨平台标签 CI `37869585885` 全绿，共 14 个 Release 文件及 SHA256SUMS）。本轮修复旧 `--listen` 启动参数：现在只能指定端口，真正监听 IP 始终为 `0.0.0.0`。本地 Go 全量/Vet、Chrome 3 窗口回归（762+39 项）、Windows 非回环网卡 `192.168.56.1` 真实 TCP/Host/来源 IP/伪造转发头验证通过。**未从第二台 LAN 设备实测**，稳定版 `v1.4.0` 尚未发布。项目为 AI Dev Manager，使用现有 LADM Environment，不创建 worktree；不覆盖已有未提交文件。

## 目标和范围

- 面向用户的「监听 IP」配置移除：服务监听所有 IPv4 网络接口 `0.0.0.0`，只配置端口。首次远程安装默认端口 8001；已有端口在升级时保持，不强行改端口。
- 提供「访问白名单」总开关，**默认关闭**。关闭表示不按域名和客户端 IP 过滤，不表示关闭认证。
- 同一设置页分开编辑「允许的访问域名/目标 IP」和「允许的客户端来源 IP/CIDR」。可以只配置任意一种；两个列表均为空时，不允许启用，避免形成无意义开关。
- 打开后，各非空列表分别校验；同时配置时需要同时通过两类检查。
- 管理端 Admin API Key、Agent API Key 和 Web 管理身份验证保持原要求，不能因为白名单关闭而跳过鉴权。公开接口不得因默认全网监听而变成未授权管理入口。
- 当前历史的 `AllowedHosts` 仅校验 HTTP Host 请求头，不代表来源 IP；独立新增 ClientIP 校验，严禁把 Host、X-Forwarded-For 或 X-Real-IP 直接当成真实来源。
- IP 白名单以直连 TCP peer `RemoteAddr` 判断；反向代理部署如需识别用户来源，必须另行设置可信代理链，绝不能默认信任外部 Forwarded 头。默认可配置反代 IP/在反代层控制来源。
- 支持 IPv4、IPv6 单地址和 CIDR；域名白名单仅做 Host 匹配（去端口、规范化），不把 DNS 解析结果当客户端来源 IP，也不支持任意伪造的宽松后缀匹配。
- 保存白名单、启用/关闭必须由已授权管理客户端执行，错误配置返回明确中文信息；启用前展示当前访问目标及远端锁定风险。
- 兼容已有白名单：升级前已经实际启用的旧 Host 限制不得在升级时默默变成全放开；应做显式迁移/兼容判断并覆盖回归测试。对新安装默认关闭。

## 修改位置

- `internal/model/types.go`：网络访问配置（开关、Host、Client IP）。
- `internal/app/gateway_access.go`：验证、持久化、状态与鉴权就绪条件。
- `internal/gateway/access.go`：请求准入与禁止伪造客户端地址。
- `internal/gateway/server.go`、`internal/gateway/web.go`、`internal/management/`、`internal/desktop/`：管理接口。
- `cmd/ai-dev-manager-desktop/frontend/`：只显示端口与白名单设置，保留诊断页中的实际 Listen 只读显示。
- `internal/cli/gateway_setup.go`、`internal/cli/gateway_service.go` 与 Gateway 启动路径：面向用户移除监听 IP 参数，仅保留端口并使用 `0.0.0.0`。
- 更新 `docs/REMOTE_ACCESS.md`、`docs/cli.md`。

## 验收（正向与反向）

1. 新安装默认 `0.0.0.0:<port>`，关闭白名单时正确 Key 可从其他 Host/IP 访问，不正确/缺少 Key 一律拒绝。
2. 域名白名单启用后：允许的 Host 可访问，其他 Host 拒绝；Host 大小写/尾点/端口正确规范化。
3. 来源 IP 白名单启用后：允许 IP/CIDR 可访问，其他来源拒绝；伪造 `X-Forwarded-For`、`Forwarded`、`X-Real-IP` 不得绕过。
4. 同时配置域名和来源 IP 时采用 AND；各自留空时跳过对应过滤；启用空规则拒绝。
5. 启用和关闭白名单可热更新，旧的明确 Host 白名单升级不意外失效。
6. 端口修改仍有效，不提供自由指定监听 IP；旧安装和 localhost/CI 受控测试场景按约定处理，不造成大面积回归。
7. Go 单测、`go test ./... -count=1`、`go vet ./...`、Desktop 实际浏览器界面检查以及 Windows 实机访问测试均通过后，才能声明完成。

## 执行顺序

在当前已存在的后台索引任务与 MCP/Skills 批量功能核对收口之后，以「Gateway 默认监听 + 可选白名单」独立开发/验收/提交，不与 Windows 版本信息脚本及 go.mod 未提交改动混合。
