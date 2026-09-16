//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	launchAtLoginRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	launchAtLoginValueName    = "adm-desktop"
)

func launchAtLoginSupported() bool { return true }

func launchAtLoginCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve Desktop executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve Desktop executable path: %w", err)
	}
	return `"` + filepath.Clean(executable) + `" --autostart`, nil
}

func launchAtLoginEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, launchAtLoginRegistryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open Windows startup registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(launchAtLoginValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Windows startup value: %w", err)
	}
	expected, err := launchAtLoginCommand()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(value), expected), nil
}

func setLaunchAtLogin(enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, launchAtLoginRegistryPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open Windows startup registry key: %w", err)
		}
		defer key.Close()
		if err := key.DeleteValue(launchAtLoginValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("remove Windows startup value: %w", err)
		}
		return nil
	}

	command, err := launchAtLoginCommand()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, launchAtLoginRegistryPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows startup registry key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(launchAtLoginValueName, command); err != nil {
		return fmt.Errorf("write Windows startup value: %w", err)
	}
	return nil
}
