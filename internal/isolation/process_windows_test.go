//go:build windows

package isolation

import (
	"os/exec"
	"testing"
)

func TestConfigureProcessCommandHidesWindowsConsole(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "exit", "0")
	configureProcessCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("Windows isolation command must configure SysProcAttr")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("Windows isolation command must hide its console window")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags=%#x; CREATE_NO_WINDOW is missing", cmd.SysProcAttr.CreationFlags)
	}
}
