//go:build darwin

package desktop

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

const launchAgentFileName = "com.wanstu.adm-desktop.plist"

func launchAtLoginSupported() bool { return true }

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentFileName), nil
}

func launchAgentContent() ([]byte, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve Desktop executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve Desktop executable path: %w", err)
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(filepath.Clean(executable))); err != nil {
		return nil, err
	}
	content := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
		"<plist version=\"1.0\">\n<dict>\n" +
		"  <key>Label</key>\n  <string>com.wanstu.adm-desktop</string>\n" +
		"  <key>ProgramArguments</key>\n  <array>\n    <string>" + escaped.String() + "</string>\n    <string>--autostart</string>\n  </array>\n" +
		"  <key>RunAtLoad</key>\n  <true/>\n</dict>\n</plist>\n"
	return []byte(content), nil
}

func launchAtLoginEnabled() (bool, error) {
	path, err := launchAgentPath()
	if err != nil {
		return false, err
	}
	actual, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read macOS LaunchAgent: %w", err)
	}
	expected, err := launchAgentContent()
	if err != nil {
		return false, err
	}
	return bytes.Equal(bytes.TrimSpace(actual), bytes.TrimSpace(expected)), nil
}

func setLaunchAtLogin(enabled bool) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove macOS LaunchAgent: %w", err)
		}
		return nil
	}
	content, err := launchAgentContent()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create macOS LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write macOS LaunchAgent: %w", err)
	}
	return nil
}
