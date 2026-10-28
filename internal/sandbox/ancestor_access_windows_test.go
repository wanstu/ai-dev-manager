//go:build windows

package sandbox

import (
	"path/filepath"
	"testing"
)

func TestWindowsAncestorPathsStopsAtVolumeRoot(t *testing.T) {
	target := filepath.Join(`D:\`, "projects", "worktrees", "env", "repo")
	got, err := windowsAncestorPaths(target)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Clean(`D:\`),
		filepath.Join(`D:\`, "projects"),
		filepath.Join(`D:\`, "projects", "worktrees"),
		filepath.Join(`D:\`, "projects", "worktrees", "env"),
	}
	if len(got) != len(want) {
		t.Fatalf("ancestor count=%d want=%d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if !filepath.IsAbs(got[i]) || !filepathEqualFold(got[i], want[i]) {
			t.Fatalf("ancestor[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}

func filepathEqualFold(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
