package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOnlyGitRefusesActiveFilterAttributes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGitForCommitTest(t, git, root, "init", "-b", "main")
	runGitForCommitTest(t, git, root, "config", "user.name", "ADM Test")
	runGitForCommitTest(t, git, root, "config", "user.email", "adm-test@example.invalid")
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=unsafe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForCommitTest(t, git, root, "add", ".gitattributes")
	runGitForCommitTest(t, git, root, "add", "tracked.txt")
	runGitForCommitTest(t, git, root, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := NewWithEnvironment(root, nil, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.GitStatus(context.Background()); err == nil || !strings.Contains(err.Error(), "may execute host code") {
		t.Fatalf("GitStatus must refuse active filter attributes, got %v", err)
	}
	if _, err := rt.GitDiff(context.Background()); err == nil || !strings.Contains(err.Error(), "may execute host code") {
		t.Fatalf("GitDiff must refuse active filter attributes, got %v", err)
	}
	if _, err := rt.GitCommit(context.Background(), "must refuse filter"); err == nil || !strings.Contains(err.Error(), "may execute host code") {
		t.Fatalf("GitCommit must refuse active filter attributes, got %v", err)
	}
}
