package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/projectanalysis"
)

func waitUntilIndexed(t *testing.T, s *app.Service, id, method string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		q, err := s.ProjectIndexQuery(id, projectanalysis.IndexQuery{Query: method, Exact: true})
		if err == nil && q.Returned == 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for native watch to index %s", method)
}

func TestAutomaticIndexUpdatesWithoutStealingSourceWriter(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Api.php")
	if err := os.WriteFile(file, []byte("<?php\nclass Api {\n public function first() {}\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := s.Workspaces.Add(root, "auto-index-watch")
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.Environments.Create(ws.ID, "auto-index-watch", "")
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(s)
	defer owner.Close()
	const agentOwner = "simulated-agent-writing-source"
	if _, err := s.Environments.AcquireWriter(env.ID, agentOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.SetAutoIndex(env.ID, true); err != nil {
		t.Fatal(err)
	}
	waitUntilIndexed(t, s, env.ID, "Api::first")
	status, err := owner.AutoIndexStatus(env.ID)
	if err != nil || !status.Enabled || status.State != "watching" {
		t.Fatalf("watching status=%+v err=%v", status, err)
	}
	if err := os.WriteFile(file, []byte("<?php\nclass Api {\n public function newerMethod() {}\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	waitUntilIndexed(t, s, env.ID, "Api::newerMethod")
	current, err := s.Environments.Get(env.ID)
	if err != nil || current.Writer == nil || current.Writer.Owner != agentOwner {
		t.Fatalf("watcher hijacked agent's writer: %+v err=%v", current.Writer, err)
	}
	status2, err := s.ProjectIndexStatus(env.ID, 2)
	if err != nil || !status2.Fresh {
		t.Fatalf("auto index stale after change: %+v %v", status2, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".adm")); !os.IsNotExist(err) {
		t.Fatalf("watcher wrote to source: %v", err)
	}
	if _, err := owner.SetAutoIndex(env.ID, false); err != nil {
		t.Fatal(err)
	}
	status3, _ := owner.AutoIndexStatus(env.ID)
	if status3.Enabled {
		t.Fatalf("disabled watcher still enabled: %+v", status3)
	}
	if _, err := s.Environments.ReleaseWriter(env.ID, agentOwner, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Environments.Remove(env.ID); err != nil {
		t.Fatal(err)
	}
	owner.DropEnvironment(env.ID)
	ids, err := s.AutoIndexIDs()
	if err != nil || len(ids) != 0 {
		t.Fatalf("orphan automatic watcher settings: %v %v", ids, err)
	}
	// Both the current pointer and immutable generations must be removed.
	storeDir := filepath.Join(filepath.Dir(s.Store.Path()), "indexes", env.ID)
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatalf("orphan index directory: %v", err)
	}
}

func TestAutoIndexSkipCacheAndIgnoredFiles(t *testing.T) {
	if !indexedSourceEvent("src/file.php") || !indexedSourceEvent("go.mod") || indexedSourceEvent("README.md") {
		t.Fatal("source event rules incorrect")
	}
	for _, name := range []string{".adm", ".git", "node_modules", "vendor", "dist"} {
		if !ignoreAutoIndexDir(name) {
			t.Fatalf("must ignore generated/cache folder %s", name)
		}
	}
	if strings.Contains(strings.ToLower("readme.md"), ".go") {
		t.Fatal("impossible")
	}
}
