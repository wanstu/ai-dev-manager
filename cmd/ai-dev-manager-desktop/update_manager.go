package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wanstu/wails-desktop-kit/updater"
)

const desktopUpdateAppID = "com.wanstu.adm-desktop"

// DesktopUpdateStatus is safe to expose to the desktop frontend. Never return
// raw update/download URLs or allow the browser to select a local setup path.
type DesktopUpdateStatus struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	ReleasePage     string `json:"release_page,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
	DownloadPath    string `json:"download_path,omitempty"`
	InstallMode     string `json:"install_mode"`
	Phase           string `json:"phase"`
	UpdateAvailable bool   `json:"update_available"`
	Downloaded      int64  `json:"downloaded"`
	Total           int64  `json:"total"`
	InstallReady    bool   `json:"install_ready"`
	Supported       bool   `json:"supported"`
}

type UpdateManager struct {
	mu            sync.Mutex
	version       string
	phase         string
	installing    bool
	checked       updater.CheckResult
	download      updater.DownloadResult
	cancel        context.CancelFunc
	bytes         int64
	total         int64
	emit          func(DesktopUpdateStatus)
	quit          func()
	clientFactory func(bool) *updater.Client
	installation  func() (updater.Installation, error)
	cacheDir      func() (string, error)
}

func newUpdateManager(version string) *UpdateManager {
	return &UpdateManager{
		version: version,
		phase:   "idle",
		installation: func() (updater.Installation, error) {
			return updater.CurrentInstallation(desktopUpdateAppID)
		},
		clientFactory: func(prerelease bool) *updater.Client {
			return &updater.Client{
				CurrentVersion: version,
				Provider: updater.GitHubProvider{
					Owner:             "wanstu",
					Repository:        "ai-dev-manager",
					IncludePrerelease: prerelease,
					HTTPClient:        &http.Client{Timeout: 30 * time.Second},
				},
				SelectAsset: updater.AssetBySuffix("windows-amd64-setup.exe"),
				HTTPClient:  &http.Client{Timeout: 0}, // cancellation uses the request context
			}
		},
		cacheDir: func() (string, error) {
			root, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(root, "adm", "updates"), nil
		},
	}
}

func (m *UpdateManager) onReady(emit func(DesktopUpdateStatus), quit func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emit = emit
	m.quit = quit
}

func (m *UpdateManager) stateLocked(mode string) DesktopUpdateStatus {
	return DesktopUpdateStatus{
		CurrentVersion:  m.version,
		LatestVersion:   m.checked.LatestVersion,
		ReleasePage:     m.checked.Release.PageURL,
		AssetName:       m.checked.Asset.Name,
		DownloadPath:    m.download.Path,
		InstallMode:     mode,
		Phase:           m.phase,
		UpdateAvailable: m.checked.UpdateAvailable,
		Downloaded:      m.bytes,
		Total:           m.total,
		InstallReady:    m.phase == "downloaded" && m.download.Path != "" && mode == string(updater.InstallationUser),
		Supported:       runtime.GOOS == "windows" && m.version != "dev",
	}
}

func (m *UpdateManager) GetDesktopUpdateStatus() (DesktopUpdateStatus, error) {
	installation, err := m.installation()
	if err != nil {
		return DesktopUpdateStatus{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateLocked(string(installation.Mode)), nil
}

func (m *UpdateManager) begin(phase string) (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.installing {
		return nil, errors.New("更新安装已经启动，请等待 Desktop 重启")
	}
	if m.cancel != nil {
		return nil, errors.New("已有更新操作进行中，请先取消或等待完成")
	}
	if runtime.GOOS != "windows" {
		return nil, errors.New("目前只支持在 Windows Desktop 检查安装包更新")
	}
	if strings.TrimSpace(m.version) == "" || m.version == "dev" {
		return nil, errors.New("开发构建不能比较更新版本，请使用正式发行版")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.phase = phase
	return ctx, nil
}

func (m *UpdateManager) finish(phase string) {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.phase = phase
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *UpdateManager) CheckDesktopUpdate(includePrerelease bool) (DesktopUpdateStatus, error) {
	ctx, err := m.begin("checking")
	if err != nil {
		return DesktopUpdateStatus{}, err
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()
	client := m.clientFactory(includePrerelease)
	result, err := client.Check(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		m.finish("error")
		return DesktopUpdateStatus{}, err
	}
	m.mu.Lock()
	m.checked = result
	m.download = updater.DownloadResult{}
	m.bytes = 0
	m.total = 0
	m.mu.Unlock()
	m.finish("checked")
	return m.GetDesktopUpdateStatus()
}

func (m *UpdateManager) DownloadDesktopUpdate() (DesktopUpdateStatus, error) {
	m.mu.Lock()
	if !m.checked.UpdateAvailable || m.checked.Asset.Name == "" {
		m.mu.Unlock()
		return DesktopUpdateStatus{}, errors.New("请先检查更新，确认存在可下载版本")
	}
	if m.download.Path != "" {
		m.mu.Unlock()
		return DesktopUpdateStatus{}, errors.New("当前版本已下载并校验，无需重复下载")
	}
	check := m.checked
	m.mu.Unlock()
	ctx, err := m.begin("downloading")
	if err != nil {
		return DesktopUpdateStatus{}, err
	}
	dir, err := m.cacheDir()
	if err != nil {
		m.finish("error")
		return DesktopUpdateStatus{}, err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		m.finish("error")
		return DesktopUpdateStatus{}, fmt.Errorf("创建更新缓存目录: %w", err)
	}
	downloadDir, err := os.MkdirTemp(dir, "download-*")
	if err != nil {
		m.finish("error")
		return DesktopUpdateStatus{}, err
	}
	client := m.clientFactory(check.Release.Prerelease)
	result, err := client.Download(ctx, check, downloadDir, func(progress updater.Progress) {
		m.mu.Lock()
		m.bytes, m.total = progress.Downloaded, progress.Total
		emit := m.emit
		status := m.stateLocked("")
		m.mu.Unlock()
		if emit != nil {
			emit(status)
		}
	})
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		_ = os.RemoveAll(downloadDir)
		m.finish("error")
		return DesktopUpdateStatus{}, err
	}
	m.mu.Lock()
	m.download = result
	m.mu.Unlock()
	m.finish("downloaded")
	return m.GetDesktopUpdateStatus()
}

func (m *UpdateManager) CancelDesktopUpdate() (DesktopUpdateStatus, error) {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return m.GetDesktopUpdateStatus()
}

// OpenDesktopUpdateFolder only reveals the already-verified setup created by
// the updater. The UI cannot supply or execute an arbitrary filesystem path.
func (m *UpdateManager) OpenDesktopUpdateFolder() error {
	if runtime.GOOS != "windows" {
		return errors.New("只有 Windows Desktop 支持打开安装包目录")
	}
	m.mu.Lock()
	path := m.download.Path
	m.mu.Unlock()
	if path == "" {
		return errors.New("尚未下载并校验安装包")
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return errors.New("安装包不存在，请重新检查并下载")
	}
	command := exec.Command("explorer.exe", "/select,", path)
	if err := command.Start(); err != nil {
		return fmt.Errorf("打开安装包位置: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}

func (m *UpdateManager) InstallDesktopUpdate() error {
	status, err := m.GetDesktopUpdateStatus()
	if err != nil {
		return err
	}
	if status.InstallMode != string(updater.InstallationUser) {
		return errors.New("只有 Kit 管理的 Windows 当前用户安装版支持自动安装；Portable / 系统级安装需要手动运行 Setup")
	}
	m.mu.Lock()
	if m.cancel != nil || m.installing {
		m.mu.Unlock()
		return errors.New("更新操作或安装仍在进行中")
	}
	if m.phase != "downloaded" || m.download.Path == "" {
		m.mu.Unlock()
		return errors.New("请先下载并校验更新安装包")
	}
	if m.quit == nil {
		m.mu.Unlock()
		return errors.New("Desktop 退出控制器未就绪，无法安全更新")
	}
	m.installing = true
	m.phase = "installing"
	download, quit := m.download, m.quit
	m.mu.Unlock()
	if _, err := updater.InstallAndRestart(desktopUpdateAppID, download); err != nil {
		m.mu.Lock()
		m.installing = false
		m.phase = "downloaded"
		m.mu.Unlock()
		return err
	}
	// Kit Setup waits for the current process to quit; preserve any running
	// Gateway exactly as with the existing 'exit keeping background' action.
	quit()
	return nil
}
