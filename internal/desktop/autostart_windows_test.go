//go:build windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchAtLoginCommandUsesHiddenAutostartFlag(t *testing.T) {
	command, err := launchAtLoginCommand()
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := `"` + filepath.Clean(executable) + `" --autostart`
	if command != want {
		t.Fatalf("startup command must preserve literal Windows path separators: got %q, want %q", command, want)
	}
	if !strings.HasSuffix(command, " --autostart") {
		t.Fatalf("launch at login command must use --autostart: %q", command)
	}
	if !strings.HasPrefix(command, `"`) {
		t.Fatalf("launch at login command must quote the executable path: %q", command)
	}
}
