package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestGitRelationReportsBehindAheadAndDivergence(t *testing.T) {
	root, gitPath := initGitRepository(t)
	runtime := mustNativeWithID(t, "ws_relation", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"git"},
		ToolPaths:          map[string]string{"git": gitPath},
	}, nil)

	equal, err := runtime.GitRelation("main", "main")
	if err != nil {
		t.Fatalf("GitRelation(equal) error = %v", err)
	}
	if equal.Ahead != 0 || equal.Behind != 0 || equal.Diverged {
		t.Fatalf("equal relation = %+v", equal)
	}

	created, err := runtime.GitWorktreeCreate("relation-env", "adm/relation-env")
	if err != nil {
		t.Fatalf("GitWorktreeCreate() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "base-new.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, root, "add", "base-new.txt")
	gitTestRun(t, gitPath, root, "commit", "-m", "base advances")

	behind, err := runtime.GitRelation("adm/relation-env", "main")
	if err != nil {
		t.Fatalf("GitRelation(behind) error = %v", err)
	}
	if behind.Ahead != 0 || behind.Behind != 1 || behind.Diverged {
		t.Fatalf("behind relation = %+v", behind)
	}

	if err := os.WriteFile(filepath.Join(created.Path, "env-new.txt"), []byte("env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, gitPath, created.Path, "add", "env-new.txt")
	gitTestRun(t, gitPath, created.Path, "commit", "-m", "environment advances")

	diverged, err := runtime.GitRelation("adm/relation-env", "main")
	if err != nil {
		t.Fatalf("GitRelation(diverged) error = %v", err)
	}
	if diverged.Ahead != 1 || diverged.Behind != 1 || !diverged.Diverged {
		t.Fatalf("diverged relation = %+v", diverged)
	}

	if _, err := runtime.GitRelation("--bad", "main"); err == nil {
		t.Fatal("option-like relation ref unexpectedly accepted")
	}
}
