# ADM 安装与开机自启（Kit 集成）

> v1.4.0-rc.3 已发布 Windows Setup、Portable 与 Linux CLI .deb；本开发分支现将 Go 依赖和 Windows/Linux 打包工具统一到 **Wails Desktop Kit v0.11.3**。这不会修改既有 RC 发布资产；新版本仍需重新运行对应发布 CI。

## Windows Desktop

从 Release 下载适合的文件：

- `adm-desktop-<version>-windows-amd64-setup.exe`：**正常安装版**，双击安装，开始菜单中显示“AI Dev Manager”。
- `adm-desktop-<version>-windows-amd64.exe`：免安装 Portable。
- `adm-desktop-<version>-windows-amd64.zip`：便于携带的 Portable ZIP。
- `SHA256SUMS-<version>.txt`：Release 文件 SHA256 总校验清单。

本分支后续 Setup 使用 Kit v0.11.3 的 NSIS 安装功能（已发布的 v1.4.0-rc.3 使用 Kit v0.11.0）：默认 **当前用户安装**、无需管理员权限；固定卸载标识 `com.wanstu.adm-desktop`，支持覆盖安装、卸载与 `/S` 静默执行。**卸载不会删除** `~/.config/adm` 中的业务数据或 Gateway 配置。安装版会被 Kit Updater 识别，但 ADM 应用内的自动更新 UI 尚未接入。

本地只打包现有已构建的 Wails EXE（需要 NSIS）：

```powershell
./scripts/build-desktop.ps1 -clean -trimpath -Version v1.4.0-rc.3 -o adm-desktop-v1.4.0-rc.3-windows-amd64.exe
./scripts/package-windows.ps1 -InputPath dist/adm-desktop-v1.4.0-rc.3-windows-amd64.exe -Version v1.4.0-rc.3
```

## Linux 无桌面服务器

**无图形界面首选** `adm-cli-<version>-linux-amd64.deb`；它通过 Kit 的 Linux Packaging 生成，不依赖 Wails、GTK 或 WebKit。arm64 使用 `linux-arm64.deb`。

首次安装及启用 systemd 自启动：

```bash
sudo apt install ./adm-cli-v1.4.0-rc.3-linux-amd64.deb
sudo adm gateway install --remote --user admin --port 8001
adm gateway service status
```

`--user` 改成实际运行 Gateway 的 Linux 用户。第二条命令使用 ADM 已有的 **受控 systemd 安装流程**：初始化 Admin/Agent 双 Key、写入 `adm-gateway.service`、启动并启用开机自启，默认绑定 `0.0.0.0` 且白名单关闭。首次输出的新 Key 必须保存好。

**已有服务的服务器不要照抄首次安装命令。** 先执行 `systemctl status adm adm-gateway` 和 `command -v adm`，检查当前进程/服务路径。如果已有 `adm.service` 或其他程序监听 8001，不要再创建一个 `adm-gateway.service`；应先确认旧服务的安装方式、数据目录和迁移方案，防止双服务争抢端口。

日常管理：

```bash
adm gateway service status
sudo adm gateway service restart
sudo adm gateway service stop
sudo adm gateway service start
sudo adm gateway service disable
sudo adm gateway service enable
```

升级：先安装新版 CLI .deb，随后运行 `sudo adm gateway install --remote` 来保留已有用户、监听端口、Key 和已设置的白名单，并重新配置和启动 Gateway。**仅升级 .deb 不代表 Gateway 进程立即切换到新版本**。

`adm-cli` 和现有含桌面程序的 `adm` Debian 包都带 `/usr/bin/adm`，因此它们**不能一起安装**；有图形桌面时保留原 `adm-<version>-linux-amd64.deb`，无图形服务器只装 `adm-cli`。新 CLI 包**不会在安装时擅自启动未初始化的公开 Gateway**；只有显式执行 `adm gateway install --remote` 才初始化 Key 和服务，避免覆盖已有 systemd 单元。

### 为什么不把 Kit 的 `--systemd` 直接加入现有 `.deb`？

Kit 的 `--systemd` 会生成 `/lib/systemd/system` 服务并在首次安装时自动启用。ADM 已经通过 `adm gateway install --remote` 管理 `/etc/systemd/system/adm-gateway.service` 和用户密钥；直接生成第二个服务可能冲突、读取不同数据目录，或绕开 Gateway 初始化。因此此轮仅用 Kit 提供安装包能力，**继续使用 ADM 原有的自启动管理**，未来必须完成安全迁移后才能替换服务管理器。
