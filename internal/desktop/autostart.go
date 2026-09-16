package desktop

import "fmt"

// DesktopPreferences contains settings owned by the Desktop process itself.
// These preferences are intentionally independent from ADM Gateway connectivity.
type DesktopPreferences struct {
	LaunchAtLoginSupported bool `json:"launch_at_login_supported"`
	LaunchAtLogin          bool `json:"launch_at_login"`
}

func (a *Adapter) GetDesktopPreferences() (DesktopPreferences, error) {
	enabled, err := launchAtLoginEnabled()
	if err != nil {
		return DesktopPreferences{}, err
	}
	return DesktopPreferences{
		LaunchAtLoginSupported: launchAtLoginSupported(),
		LaunchAtLogin:          enabled,
	}, nil
}

func (a *Adapter) SetLaunchAtLogin(enabled bool) (DesktopPreferences, error) {
	if enabled && !launchAtLoginSupported() {
		return DesktopPreferences{}, fmt.Errorf("launch at login is not supported on this platform")
	}
	if err := setLaunchAtLogin(enabled); err != nil {
		return DesktopPreferences{}, err
	}
	return a.GetDesktopPreferences()
}
