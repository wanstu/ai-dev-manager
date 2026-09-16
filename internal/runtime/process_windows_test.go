//go:build windows

package runtime

import (
	"os/exec"
	"testing"
)

func TestConfigureCommandSuppressesWindowsConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	configureCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("Windows command must configure SysProcAttr")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("Windows command must hide its console window")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("Windows command CreationFlags = %#x; CREATE_NO_WINDOW is missing", cmd.SysProcAttr.CreationFlags)
	}
}
