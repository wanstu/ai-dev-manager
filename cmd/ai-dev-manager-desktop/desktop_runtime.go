package main

import (
	"context"
	"sync"

	"ai-dev-manager-v2/internal/desktop"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	desktopkit "github.com/wanstu/wails-desktop-kit"
)

const (
	trayQuitKeepBackgroundLabel = "退出（保留后台）"
	trayQuitStopBackgroundLabel = "退出（不保留后台）"
)

type desktopAutoStartProvider struct {
	adapter *desktop.Adapter

	mu  sync.RWMutex
	ctx context.Context
}

func newDesktopAutoStartProvider(adapter *desktop.Adapter) *desktopAutoStartProvider {
	return &desktopAutoStartProvider{adapter: adapter}
}

func (p *desktopAutoStartProvider) setContext(ctx context.Context) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.ctx = ctx
	p.mu.Unlock()
}

func (p *desktopAutoStartProvider) Supported() bool {
	if p == nil || p.adapter == nil {
		return false
	}
	preferences, err := p.adapter.GetDesktopPreferences()
	return err == nil && preferences.LaunchAtLoginSupported
}

func (p *desktopAutoStartProvider) Enabled() (bool, error) {
	if p == nil || p.adapter == nil {
		return false, nil
	}
	preferences, err := p.adapter.GetDesktopPreferences()
	if err != nil {
		return false, err
	}
	return preferences.LaunchAtLogin, nil
}

func (p *desktopAutoStartProvider) SetEnabled(enabled bool) error {
	if p == nil || p.adapter == nil {
		return nil
	}
	if _, err := p.adapter.SetLaunchAtLogin(enabled); err != nil {
		return err
	}
	p.mu.RLock()
	ctx := p.ctx
	p.mu.RUnlock()
	if ctx != nil {
		wailsruntime.EventsEmit(ctx, "desktop:preferences-changed")
	}
	return nil
}

func desktopTrayConfig(icon []byte, adapter *desktop.Adapter, autoStart desktopkit.AutoStartProvider) desktopkit.TrayConfig {
	return desktopkit.TrayConfig{
		Enabled:     true,
		Icon:        icon,
		Tooltip:     "adm-desktop",
		AutoStart:   autoStart,
		DisableQuit: true,
		FooterItems: []desktopkit.TrayItem{
			desktopkit.Action(trayQuitKeepBackgroundLabel, func(controller *desktopkit.Controller) error {
				controller.Quit()
				return nil
			}),
			desktopkit.Action(trayQuitStopBackgroundLabel, func(controller *desktopkit.Controller) error {
				quitAndStopLocalBackground(controller, adapter)
				return nil
			}),
		},
	}
}

func quitAndStopLocalBackground(controller *desktopkit.Controller, adapter *desktop.Adapter) {
	if controller == nil {
		return
	}
	if adapter == nil {
		showTrayStopBlocked(controller.Context(), "Desktop adapter 不可用，无法确认或停止后台服务。")
		return
	}

	controller.ShowWindow()
	profile, status, err := activeConnectionStatusForQuit(adapter)
	if err != nil {
		showTrayStopBlocked(controller.Context(), "无法安全确认当前后台服务状态；未停止任何服务。\n\n"+err.Error())
		return
	}
	if status.State != "running" {
		controller.Quit()
		return
	}
	if profile.ID == "" || profile.BaseURL == "" || !status.LocalBootstrapEligible {
		showTrayStopBlocked(controller.Context(), "当前活动连接不是 Desktop 可安全停止的本地 loopback ADM；未停止任何服务。")
		return
	}
	if _, err := adapter.StopLocalADM(desktop.ADMConnectionInput{BaseURL: profile.BaseURL}); err != nil {
		showTrayStopBlocked(controller.Context(), "本地后台服务停止失败；Desktop 保持运行，后台服务没有被强制终止。\n\n"+err.Error())
		return
	}
	controller.Quit()
}

func showTrayStopBlocked(ctx context.Context, message string) {
	if ctx == nil {
		return
	}
	_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.WarningDialog,
		Title:   "未退出",
		Message: message,
	})
}

func activeConnectionStatusForQuit(adapter *desktop.Adapter) (desktop.ConnectionProfile, desktop.ADMConnectionStatus, error) {
	profiles, err := adapter.GetConnectionProfiles()
	if err != nil {
		return desktop.ConnectionProfile{}, desktop.ADMConnectionStatus{}, err
	}
	profile, ok := activeConnectionProfile(profiles)
	if !ok {
		return desktop.ConnectionProfile{}, desktop.ADMConnectionStatus{}, nil
	}
	status, err := adapter.InspectADMConnection(desktop.ADMConnectionInput{BaseURL: profile.BaseURL})
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
