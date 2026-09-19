package desktop

import (
	"fmt"

	"github.com/wanstu/wails-desktop-kit/autostart"
)

const (
	desktopAutostartID          = "com.wanstu.adm-desktop"
	desktopAutostartDisplayName = "AI Dev Manager"
	desktopAutostartComment     = "Local development control plane"
)

func newLaunchAtLoginManager() (*autostart.Manager, error) {
	return autostart.New(autostart.Config{
		ID:          desktopAutostartID,
		DisplayName: desktopAutostartDisplayName,
		Comment:     desktopAutostartComment,
		Arguments:   []string{"--autostart"},
	})
}

// DesktopPreferences contains settings owned by the Desktop process itself.
// These preferences are intentionally independent from ADM Gateway connectivity.
type DesktopPreferences struct {
	LaunchAtLoginSupported bool   `json:"launch_at_login_supported"`
	LaunchAtLogin          bool   `json:"launch_at_login"`
	ThemeMode              string `json:"theme_mode"`
	ThemePack              string `json:"theme_pack"`
}

func (a *Adapter) GetDesktopPreferences() (DesktopPreferences, error) {
	manager, err := newLaunchAtLoginManager()
	if err != nil {
		return DesktopPreferences{}, err
	}
	enabled, err := manager.Enabled()
	if err != nil {
		return DesktopPreferences{}, err
	}
	themePreferences, err := a.desktopThemePreferences()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return DesktopPreferences{
		LaunchAtLoginSupported: manager.Supported(),
		LaunchAtLogin:          enabled,
		ThemeMode:              themePreferences.ThemeMode,
		ThemePack:              themePreferences.ThemePack,
	}, nil
}

func (a *Adapter) SetLaunchAtLogin(enabled bool) (DesktopPreferences, error) {
	manager, err := newLaunchAtLoginManager()
	if err != nil {
		return DesktopPreferences{}, err
	}
	if enabled && !manager.Supported() {
		return DesktopPreferences{}, fmt.Errorf("launch at login is not supported on this platform")
	}
	if err := manager.SetEnabled(enabled); err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}
