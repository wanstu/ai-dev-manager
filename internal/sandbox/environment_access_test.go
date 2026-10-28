package sandbox

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnvironmentWorkspaceTraversePathsStayInsideWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	environment := filepath.Join(workspace, "worktrees", "ws_example", "wt_example")

	got, err := environmentWorkspaceTraversePaths(workspace, environment)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		workspace,
		filepath.Join(workspace, "worktrees"),
		filepath.Join(workspace, "worktrees", "ws_example"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("traverse paths=%v want=%v", got, want)
	}
}

func TestEnvironmentWorkspaceTraversePathsRootEnvironmentNeedsNoAncestorGrant(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	got, err := environmentWorkspaceTraversePaths(workspace, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("workspace-root Environment should need no ancestor traverse ACLs: %v", got)
	}
}

func TestEnvironmentWorkspaceTraversePathsRejectEscape(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	environment := filepath.Join(base, "other", "project")
	if _, err := environmentWorkspaceTraversePaths(workspace, environment); err == nil {
		t.Fatal("Environment outside Workspace unexpectedly accepted")
	}
}
