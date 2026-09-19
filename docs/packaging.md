# 打包、CI 与 GitHub Release（v1.2）

## 1. 用户可见命名

- CLI：`adm`
- Desktop：`adm-desktop`

Go module path 仍为 `ai-dev-manager-v2`，这是源码内部 import path，不作为用户可见应用名。

## 2. 发布产物

v1.2.0 GitHub Release 产物：

```text
adm-v1.2.0-windows-amd64.exe
adm-v1.2.0-linux-amd64
adm-v1.2.0-darwin-amd64
adm-desktop-v1.2.0-windows-amd64.exe
SHA256SUMS-v1.2.0.txt
```

Release workflow 会为所有上传文件生成 SHA-256 校验清单。

## 3. 本地 CLI build

Windows：

```powershell
New-Item -ItemType Directory -Force -Path dist | Out-Null
go build -trimpath -o dist\adm-windows-amd64.exe ./cmd/ai-dev-manager
```

## 4. Desktop build

Desktop 必须走 Wails：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop.ps1 -clean -trimpath
```

默认：

```text
dist\adm-desktop-windows-amd64.exe
```

不要把：

```text
go build ./cmd/ai-dev-manager-desktop
```

当成可分发 Desktop。它不等价于正式 Wails production artifact。

## 5. 本地版本化打包

仓库保留 `scripts/build-rc.ps1` 用于生成带版本名的本地 Windows CLI/Desktop/checksum bundle。例如：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-rc.ps1 -Version v1.2.0
```

`-Version` 不只用于文件名：它会通过统一的 linker 变量注入 CLI、Desktop 和内嵌 Gateway。发布产物中的 `adm version`、Desktop 窗口标题和 Gateway/Admin MCP metadata 因此使用同一个版本值。普通未注入的源码构建明确显示为 `dev`。

默认输出类似：

```text
dist\adm-v1.2.0-windows-amd64.exe
dist\adm-desktop-v1.2.0-windows-amd64.exe
dist\SHA256SUMS-v1.2.0.txt
```

也可指定目录：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-rc.ps1 -Version v1.2.0 -OutputDir .\dist\release
```

这个脚本是本地打包辅助；正式 GitHub Release 以 tag-triggered CI workflow 产物为准。

## 6. GitHub Actions workflow

ADM Desktop runtime 已使用 Wails Desktop Kit，但发布编排继续保留在 ADM 仓库：ADM 同一 Release 还包含多架构 CLI、Linux `.deb` / `.tar.gz`、统一版本注入和总校验清单，这些超出 Kit 通用三平台 Desktop reusable workflow 的职责。Kit 负责公共桌面基础设施，ADM workflow 负责产品发布图。

`.github/workflows/ci.yml` 在以下情况触发：

- push `master`；
- push `v*` tag；
- pull request；
- manual workflow dispatch。

### Windows gate

第一个 job 在 `windows-latest`：

```text
go test -count=1 ./...
go vet ./...
scripts/build-desktop.ps1 -clean -trimpath
```

Desktop artifact 只有这一步全部通过后才上传。

### CLI cross-build

Windows gate 成功后构建：

```text
linux-amd64
darwin-amd64
windows-amd64
```

CLI 使用：

```text
go build -trimpath ./cmd/ai-dev-manager
```

并上传 GitHub Actions artifacts。

## 7. 普通 branch / PR artifact naming

非 tag 构建：

```text
adm-windows-amd64.exe
adm-linux-amd64
adm-darwin-amd64
adm-desktop-windows-amd64.exe
```

这些是 CI artifacts，不会创建正式 GitHub Release。

## 8. Tag artifact naming

例如 tag `v1.2.0`：

```text
adm-v1.2.0-windows-amd64.exe
adm-v1.2.0-linux-amd64
adm-v1.2.0-darwin-amd64
adm-desktop-v1.2.0-windows-amd64.exe
```

## 9. 自动 GitHub Release

当 ref 是 `refs/tags/v*` 且前置 jobs 成功后，`Publish GitHub Release` job 会：

1. checkout 对应 tag；
2. 下载 Windows Desktop + 三平台 CLI artifacts；
3. 合并到 `dist/`；
4. 生成 `SHA256SUMS-<tag>.txt`；
5. 使用 `softprops/action-gh-release` 创建 GitHub Release；
6. 上传 `dist/*`；
7. 自动生成 release notes。

Tag 带 `-`（例如 `v1.2.0-rc.1`）会被标记为 prerelease；普通 `v1.2.0` 是正式 release。

因此发布流程应当是：

```text
master CI green
-> create/push annotated version tag
-> tag CI reruns tests/vet/builds
-> Publish GitHub Release automatically
```

不要在 tag CI 还没完成时把 release 当作成功。

## 10. v1.1.0 发布事实

`v1.1.0` 已通过 tag-triggered完整 workflow：

- Windows full test PASS；
- `go vet ./...` PASS；
- Wails Windows Desktop build PASS；
- Linux CLI build PASS；
- macOS CLI build PASS；
- Windows CLI build PASS；
- checksum generation PASS；
- GitHub Release publish PASS。

## 11. Desktop icon pipeline

`cmd/ai-dev-manager-desktop/wails.json` 的 frontend build 会运行：

```text
go run ../../../tools/prepare-desktop-assets
```

它会：

- 复制 `assets/icons/ai-dev-manager-app.png` 到 Wails `build/appicon.png`；
- 复制 window branding asset 到 Desktop frontend；
- 使用 Wails Desktop Kit `icon.NormalizeFile` 裁掉 tray source 透明留白并按 0.94 fill 生成 tray asset；
- Windows 下删除旧 generated icon，让 Wails 从最新 `build/appicon.png` 重建平台图标。

`scripts/prepare-desktop-icons.ps1` 只保留为手动入口，内部直接委托同一个 Go asset tool，不再包含 System.Drawing 图像处理实现。

## 12. 发布前最低检查

即使 master 最近已绿，正式 tag 仍会重新跑完整 release workflow。建议人工确认：

```text
working tree clean
master == origin/master
目标 tag 不存在
master CI success
```

然后再 push tag。最终以 tag workflow + Release assets 为发布事实。
