package app

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/projectanalysis"
)

func TestAnalyzeProjectIncrementallyReusesAndRefreshesSource(t *testing.T) {
	root := t.TempDir()
	phpPath := filepath.Join(root, "Api.php")
	if err := os.WriteFile(phpPath, []byte("<?php\nclass Api {\n  public function first() {}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "incremental-project")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "incremental-project", "")
	if err != nil {
		t.Fatal(err)
	}
	const owner = "incremental-project-test"
	if _, err := service.Environments.AcquireWriter(env.ID, owner); err != nil {
		t.Fatal(err)
	}
	defer service.Environments.ReleaseWriter(env.ID, owner, false)

	first, err := service.AnalyzeProject(env.ID, owner, 100, 100)
	if err != nil || first.IndexMode != "full" || first.ReindexedSourceFiles != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.AnalyzeProject(env.ID, owner, 100, 100)
	if err != nil || second.IndexMode != "incremental" ||
		second.ReusedSourceFiles != 1 || second.ReindexedSourceFiles != 0 {
		t.Fatalf("second=%+v err=%v", second, err)
	}

	if err := os.WriteFile(phpPath, []byte("<?php\nclass Api {\n  public function later() {}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := service.AnalyzeProject(env.ID, owner, 100, 100)
	if err != nil || third.IndexMode != "incremental" ||
		third.ReindexedSourceFiles != 1 || third.ReusedSourceFiles != 0 {
		t.Fatalf("modified=%+v err=%v", third, err)
	}
	matches, err := service.ProjectIndexQuery(env.ID, projectanalysis.IndexQuery{Query: "Api::later", Exact: true})
	if err != nil || matches.Returned != 1 || matches.Matches[0].Path != "Api.php" {
		t.Fatalf("updated query=%+v err=%v", matches, err)
	}
	stale, err := service.ProjectIndexQuery(env.ID, projectanalysis.IndexQuery{Query: "Api::first", Exact: true})
	if err != nil || stale.Returned != 0 {
		t.Fatalf("removed symbol must be gone: %+v err=%v", stale, err)
	}
}
