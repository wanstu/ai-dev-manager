//go:build windows

package pathutil

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsShortAndLongPathsCompareEqual(t *testing.T) {
	root := t.TempDir()
	longDir := filepath.Join(root, "runneradmin-canonical-path-test")
	if err := os.MkdirAll(longDir, 0o755); err != nil {
		t.Fatal(err)
	}

	input, err := windows.UTF16PtrFromString(longDir)
	if err != nil {
		t.Fatal(err)
	}
	size, err := windows.GetShortPathName(input, nil, 0)
	if err != nil || size == 0 {
		t.Skipf("8.3 short names unavailable: %v", err)
	}
	buf := make([]uint16, size)
	n, err := windows.GetShortPathName(input, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("8.3 short names unavailable: %v", err)
	}
	shortDir := windows.UTF16ToString(buf)
	if filepath.Clean(shortDir) == filepath.Clean(longDir) {
		t.Skip("filesystem returned no distinct 8.3 short path")
	}

	if !Same(shortDir, longDir) {
		t.Fatalf("short and long paths should compare equal:\nshort=%q\nlong=%q\nshort canonical=%q\nlong canonical=%q", shortDir, longDir, ForCompare(shortDir), ForCompare(longDir))
	}

	shortMissing := filepath.Join(shortDir, "future", "child")
	longMissing := filepath.Join(longDir, "future", "child")
	if !Same(shortMissing, longMissing) {
		t.Fatalf("missing descendants should preserve short/long equivalence:\nshort=%q\nlong=%q", ForCompare(shortMissing), ForCompare(longMissing))
	}
	if !Within(longDir, shortMissing) {
		t.Fatalf("short-path descendant should remain within long-path root")
	}
}
