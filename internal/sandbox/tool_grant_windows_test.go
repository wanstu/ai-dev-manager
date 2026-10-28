//go:build windows

package sandbox

import (
	"path/filepath"
	"testing"
)

func TestRequiresExplicitToolGrantSkipsProtectedWindowsRoots(t *testing.T) {
	base := t.TempDir()
	programFiles := filepath.Join(base, "Program Files")
	programFilesX86 := filepath.Join(base, "Program Files (x86)")
	windowsRoot := filepath.Join(base, "Windows")
	t.Setenv("ProgramFiles", programFiles)
	t.Setenv("ProgramFiles(x86)", programFilesX86)
	t.Setenv("SystemRoot", windowsRoot)
	t.Setenv("WINDIR", windowsRoot)

	for _, path := range []string{
		filepath.Join(programFiles, "Git", "cmd", "git.exe"),
		filepath.Join(programFilesX86, "Tool", "tool.exe"),
		filepath.Join(windowsRoot, "System32", "cmd.exe"),
	} {
		if requiresExplicitToolGrant(path) {
			t.Fatalf("protected Windows application path unexpectedly requires DACL mutation: %s", path)
		}
	}
	if !requiresExplicitToolGrant(filepath.Join(base, "tools", "go", "bin", "go.exe")) {
		t.Fatal("custom tool root should still require an explicit narrow grant")
	}
}
