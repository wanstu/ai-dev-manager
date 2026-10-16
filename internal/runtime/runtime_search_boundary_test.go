package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchDoesNotFollowFileSymlinkOutsideEnvironment(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	const sentinel = "ADM-OUTSIDE-SEARCH-SENTINEL"
	if err := os.WriteFile(outside, []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "leak.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("file symlink is unavailable on this platform/account: %v", err)
	}

	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := rt.Search("", sentinel, 100, 100, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("directory search escaped Environment through file symlink: %+v", matches)
	}
	if _, err := rt.Search("leak.txt", sentinel, 100, 100, 1<<20); err == nil || !strings.Contains(err.Error(), "escapes environment root") {
		t.Fatalf("direct symlink search must be rejected by Environment containment, got %v", err)
	}
}
