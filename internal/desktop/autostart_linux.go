//go:build linux

package desktop

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const linuxAutostartFileName = "adm-desktop.desktop"

func launchAtLoginSupported() bool { return true }

func linuxAutostartPath() (string, error) {
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", linuxAutostartFileName), nil
}

func linuxAutostartContent() ([]byte, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve Desktop executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve Desktop executable path: %w", err)
	}
	quoted := strings.ReplaceAll(filepath.Clean(executable), "\\", "\\\\")
	quoted = strings.ReplaceAll(quoted, "\"", "\\\"")
	content := fmt.Sprintf("[Desktop Entry]\nType=Application\nVersion=1.0\nName=AI Dev Manager\nComment=Local development control plane\nExec=\"%s\" --autostart\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", quoted)
	return []byte(content), nil
}

func launchAtLoginEnabled() (bool, error) {
	path, err := linuxAutostartPath()
	if err != nil {
		return false, err
	}
	actual, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Linux autostart entry: %w", err)
	}
	expected, err := linuxAutostartContent()
	if err != nil {
		return false, err
	}
	return bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(expected)), nil
}

func setLaunchAtLogin(enabled bool) error {
	path, err := linuxAutostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove Linux autostart entry: %w", err)
		}
		return nil
	}
	content, err := linuxAutostartContent()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Linux autostart directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write Linux autostart entry: %w", err)
	}
	return nil
}
