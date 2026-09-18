//go:build !windows

package pathutil

import (
	"path/filepath"
	"testing"
)

func TestSameIsCaseSensitiveOnUnix(t *testing.T) {
	root := t.TempDir()
	upper := filepath.Join(root, "Project")
	lower := filepath.Join(root, "project")
	if Same(upper, lower) {
		t.Fatalf("Unix path comparison must preserve case: %q and %q", upper, lower)
	}
}
