//go:build windows || darwin || linux

package main

import (
	"context"
	"runtime"
	"sync"

	"ai-dev-manager-v2/internal/desktop"

	"github.com/gogpu/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const traySupported = true

// Linux tray availability depends on the active desktop's StatusNotifier host.
// Keep the tray enabled there, but do not make the main window unreachable by
// automatically hiding it until runtime tray-host detection is available.
func trayWindowHidingSupported() bool { return runtime.GOOS != "linux" }

const (
	trayQuitKeepBackgroundLabel = "退出（保留后台）"
	trayQuitStopBackgroundLabel = "退出（不保留后台）"
)

type trayManager struct {
	adapter *desktop.Adapter
	icon    []byte

	mu         sync.RWMutex
	ctx        context.Context
	tray       *systray.SystemTray
	launchItem *systray.MenuItem
	stopEvents func()
	started    bool
	stopping   bool
}

func newTrayManager(icon []byte, adapter *desktop.Adapter) *trayManager {
	return &trayManager{adapter: adapter, icon: icon}
}

// Startup only captures the Wails runtime context. The tray message loop is
// intentionally started from DomReady, matching the proven ime-lock-v2
// lifecycle and avoiding tray callbacks before the WebView/runtime is ready.
func (t *trayManager) Startup(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	t.mu.Unlock()
}

func (t *trayManager) DomReady(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	if t.started {
		t.mu.Unlock()
		return
	}
	t.started = true
	t.stopping = false
	t.mu.Unlock()

	go t.run()
}

func (t *trayManager) Shutdown(context.Context) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.stopping = true
	tray := t.tray
	stopEvents := t.stopEvents
	t.stopEvents = nil
	t.mu.Unlock()
	if stopEvents != nil {
		stopEvents()
	}
	if tray != nil {
		tray.Remove()
	}
}

func (t *trayManager) run() {
	// Keep the systray platform loop on one locked OS thread. gogpu/systray
	// owns the native tray/message-loop integration on Windows, macOS, and
	// Linux; keeping creation and Run on one thread also preserves the
	// Windows message-loop requirement.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tray := systray.New()
	menu := systray.NewMenu()
	menu.Add("显示主窗口", t.ShowWindow)
	menu.Add("隐藏主窗口", t.hideWindow)
	menu.AddSeparator()

	launchEnabled := false
	launchSupported := false
	if t.adapter != nil {
		if preferences, err := t.adapter.GetDesktopPreferences(); err == nil {
			launchSupported = preferences.LaunchAtLoginSupported
			launchEnabled = preferences.LaunchAtLogin
		}
	}
	var launchItem *systray.MenuItem
	launchItem = menu.AddCheckbox("开机启动", launchEnabled, func() {
		t.toggleLaunchAtLogin(launchItem)
	})
	if !launchSupported {
		launchItem.SetDisabled(true)
	}
	menu.AddSeparator()
	menu.Add(trayQuitKeepBackgroundLabel, t.quitDesktop)
	menu.Add(trayQuitStopBackgroundLabel, t.quitAndStopLocalBackground)

	tray.SetIcon(t.icon).
		SetTooltip("adm-desktop").
		SetMenu(menu)
	tray.OnClick(t.ShowWindow)
	tray.OnDoubleClick(t.ShowWindow)
	tray.OnRightClick(t.syncLaunchAtLogin)
	tray.Show()

	t.mu.Lock()
	if t.stopping {
		t.started = false
		t.mu.Unlock()
		tray.Remove()
		return
	}
	t.tray = tray
	t.launchItem = launchItem
	ctx := t.ctx
	t.mu.Unlock()

	if ctx != nil {
		stopEvents := wailsruntime.EventsOn(ctx, "desktop:preferences-changed", func(...interface{}) {
			t.syncLaunchAtLogin()
		})
		t.mu.Lock()
		if t.stopping {
			t.mu.Unlock()
			stopEvents()
			tray.Remove()
			return
		}
		t.stopEvents = stopEvents
		t.mu.Unlock()
	}

	_ = tray.Run()

	t.mu.Lock()
	stopEvents := t.stopEvents
	t.stopEvents = nil
	t.tray = nil
	t.launchItem = nil
	t.started = false
	t.mu.Unlock()
	if stopEvents != nil {
		stopEvents()
	}
}

func (t *trayManager) ShowWindow() {
	ctx := t.runtimeContext()
	if ctx == nil {
		return
	}
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}

func (t *trayManager) hideWindow() {
	ctx := t.runtimeContext()
	if ctx != nil {
		wailsruntime.WindowHide(ctx)
	}
}

func (t *trayManager) quitDesktop() {
	ctx := t.runtimeContext()
	if ctx != nil {
		wailsruntime.Quit(ctx)
	}
}

func (t *trayManager) quitAndStopLocalBackground() {
	ctx := t.runtimeContext()
	if ctx == nil {
		return
	}
	if t == nil || t.adapter == nil {
		showTrayStopBlocked(ctx, "Desktop adapter 不可用，无法确认或停止后台服务。")
		return
	}

	t.ShowWindow()
	profile, status, err := t.activeConnectionStatusForQuit()
	if err != nil {
		showTrayStopBlocked(ctx, "无法安全确认当前后台服务状态；未停止任何服务。\n\n"+err.Error())
		return
	}
	if status.State != "running" {
		wailsruntime.Quit(ctx)
		return
	}
	if profile.ID == "" || profile.BaseURL == "" || !status.LocalBootstrapEligible {
		showTrayStopBlocked(ctx, "当前活动连接不是 Desktop 可安全停止的本地 loopback ADM；未停止任何服务。")
		return
	}
	if _, err := t.adapter.StopLocalADM(desktop.ADMConnectionInput{BaseURL: profile.BaseURL}); err != nil {
		showTrayStopBlocked(ctx, "本地后台服务停止失败；Desktop 保持运行，后台服务没有被强制终止。\n\n"+err.Error())
		return
	}
	wailsruntime.Quit(ctx)
}

func showTrayStopBlocked(ctx context.Context, message string) {
	_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.WarningDialog,
		Title:   "未退出",
		Message: message,
	})
}

func (t *trayManager) activeConnectionStatusForQuit() (desktop.ConnectionProfile, desktop.ADMConnectionStatus, error) {
	profiles, err := t.adapter.GetConnectionProfiles()
	if err != nil {
		return desktop.ConnectionProfile{}, desktop.ADMConnectionStatus{}, err
	}
	profile, ok := activeConnectionProfile(profiles)
	if !ok {
		return desktop.ConnectionProfile{}, desktop.ADMConnectionStatus{}, nil
	}
	status, err := t.adapter.InspectADMConnection(desktop.ADMConnectionInput{BaseURL: profile.BaseURL})
	return profile, status, err
}

func activeConnectionProfile(profiles desktop.ConnectionProfiles) (desktop.ConnectionProfile, bool) {
	for _, profile := range profiles.Profiles {
		if profile.ID == profiles.ActiveID && profile.ID != "" {
			return profile, true
		}
	}
	return desktop.ConnectionProfile{}, false
}

func (t *trayManager) runtimeContext() context.Context {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ctx
}

func (t *trayManager) toggleLaunchAtLogin(item *systray.MenuItem) {
	if t.adapter == nil || item == nil {
		return
	}
	preferences, err := t.adapter.GetDesktopPreferences()
	if err != nil || !preferences.LaunchAtLoginSupported {
		item.SetDisabled(true)
		return
	}
	preferences, err = t.adapter.SetLaunchAtLogin(!preferences.LaunchAtLogin)
	if err != nil {
		t.ShowWindow()
		if ctx := t.runtimeContext(); ctx != nil {
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Type: wailsruntime.ErrorDialog, Title: "开机启动设置失败", Message: err.Error(),
			})
		}
		return
	}
	item.SetChecked(preferences.LaunchAtLogin)
	if ctx := t.runtimeContext(); ctx != nil {
		wailsruntime.EventsEmit(ctx, "desktop:preferences-changed")
	}
}

func (t *trayManager) syncLaunchAtLogin() {
	if t.adapter == nil {
		return
	}
	preferences, err := t.adapter.GetDesktopPreferences()
	if err != nil {
		return
	}
	t.mu.RLock()
	item := t.launchItem
	t.mu.RUnlock()
	if item == nil {
		return
	}
	item.SetDisabled(!preferences.LaunchAtLoginSupported)
	item.SetChecked(preferences.LaunchAtLogin)
}
