package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileAPIsReserveGitMetadata(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const sentinel = "ADM-GIT-METADATA-SENTINEL"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "normal.txt"), []byte("normal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rt.Read(".git/config", 4096); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Read must refuse Git metadata, got %v", err)
	}
	if _, err := rt.Write(".git/config", "changed", false); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Write must refuse Git metadata, got %v", err)
	}
	if _, err := rt.Edit(".git/config", sentinel, "changed", 1); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Edit must refuse Git metadata, got %v", err)
	}
	if _, err := rt.Delete(".git/config"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Delete must refuse Git metadata, got %v", err)
	}
	items, err := rt.Tree("", 8, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if isReservedGitMetadataPath(item.Path) {
			t.Fatalf("Tree exposed Git metadata: %+v", item)
		}
	}
	matches, err := rt.Search("", sentinel, 100, 100, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("Search exposed Git metadata: %+v", matches)
	}

	dirLink := filepath.Join(root, "git-meta-link")
	if err := os.Symlink(gitDir, dirLink); err == nil {
		if _, err := rt.Read("git-meta-link/config", 4096); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("Read followed directory symlink into Git metadata: %v", err)
		}
	}
	fileLink := filepath.Join(root, "git-config-link")
	if err := os.Symlink(filepath.Join(gitDir, "config"), fileLink); err == nil {
		if _, err := rt.Read("git-config-link", 4096); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("Read followed file symlink into Git metadata: %v", err)
		}
		matches, err := rt.Search("", sentinel, 100, 100, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("Search followed symlink into Git metadata: %+v", matches)
		}
	}
}
