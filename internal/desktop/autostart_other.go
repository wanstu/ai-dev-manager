//go:build !windows

package desktop

func launchAtLoginSupported() bool { return false }

func launchAtLoginEnabled() (bool, error) { return false, nil }

func setLaunchAtLogin(enabled bool) error { return nil }
