package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/projectanalysis"
)

func setupIndexEnvironment(t *testing.T) (*Service, string, string) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "Service.php")
	if err := os.WriteFile(file, []byte("<?php\nclass Service {\n public function first() {}\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(t.TempDir(), "adm-state.json"))
	ws, err := s.Workspaces.Add(root, "index-owned-by-ADM")
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.Environments.Create(ws.ID, "index-env", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, env.ID, root
}

func TestExternalProjectIndexNeverRequiresEnvironmentWriter(t *testing.T) {
	s, id, root := setupIndexEnvironment(t)
	const writer = "real-agent-editing-source"
	if _, err := s.Environments.AcquireWriter(id, writer); err != nil {
		t.Fatal(err)
	}
	before, err := s.Environments.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.AnalyzeProject(id, "", 100, 100)
	if err != nil {
		t.Fatalf("read-only index must not compete with source writer: %v", err)
	}
	if result.ReindexedSourceFiles != 1 || result.IndexMode != "full" {
		t.Fatalf("first index=%+v", result)
	}
	after, err := s.Environments.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if before.Writer == nil || after.Writer == nil || after.Writer.Owner != writer {
		t.Fatalf("source lease mutated by index: before=%+v after=%+v", before.Writer, after.Writer)
	}
	if _, err := os.Stat(filepath.Join(root, ".adm")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project source directory contaminated with index files: %v", err)
	}
	rootIndex, err := s.indexStore.current(id, root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rootIndex, s.indexStore.root) {
		t.Fatalf("index stored outside ADM data dir: %q", rootIndex)
	}
	if _, err := os.Stat(filepath.Join(rootIndex, filepath.FromSlash(projectanalysis.IndexManifestRelativePath))); err != nil {
		t.Fatal(err)
	}
	found, err := s.ProjectIndexQuery(id, projectanalysis.IndexQuery{Query: "Service::first", Exact: true})
	if err != nil || found.Returned != 1 {
		t.Fatalf("query external index=%+v %v", found, err)
	}
	status, err := s.ProjectIndexStatus(id, 5)
	if err != nil || status.State != "fresh" {
		t.Fatalf("external index status=%+v %v", status, err)
	}
	if err := os.WriteFile(filepath.Join(root, "Service.php"), []byte("<?php\nclass Service {\n public function second() {}\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := s.AnalyzeProject(id, "", 100, 100)
	if err != nil || second.IndexMode != "incremental" {
		t.Fatalf("reindex under writer=%+v %v", second, err)
	}
	queries, err := s.ProjectIndexQuery(id, projectanalysis.IndexQuery{Query: "Service::second", Exact: true})
	if err != nil || queries.Returned != 1 {
		t.Fatalf("refreshed symbol=%+v %v", queries, err)
	}
	assertNoProjectIndex(t, root)
	if _, err := s.Environments.ReleaseWriter(id, writer, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Environments.Remove(id); err != nil {
		t.Fatal(err)
	}
	// Environment.Remove hooks must clean ADM cache without an explicit
	// CleanupProjectIndex call from the UI or the caller.
	if _, err := os.Stat(s.indexStore.environmentDir(id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan index after deletion: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Service.php")); err != nil {
		t.Fatalf("source deleted with index: %v", err)
	}
}
func assertNoProjectIndex(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".adm")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("index should never create project-owned .adm: %v", err)
	}
}

func TestProjectIndexConcurrentReadersObserveCompleteGenerations(t *testing.T) {
	s, id, root := setupIndexEnvironment(t)
	if _, err := s.AnalyzeProject(id, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 100)
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for n := 0; n < 30; n++ {
				q, err := s.ProjectIndexQuery(id, projectanalysis.IndexQuery{Query: "Service", MaxResults: 10})
				if err != nil || !q.ArtifactVerified || q.Returned < 1 {
					errs <- errors.New("reader saw torn index generation")
				}
			}
		}()
	}
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			v := "first"
			if i%2 == 1 {
				v = "second"
			}
			code := "<?php\nclass Service {\n public function " + v + "() {}\n}\n"
			if err := os.WriteFile(filepath.Join(root, "Service.php"), []byte(code), 0644); err != nil {
				errs <- err
				return
			}
			if _, err := s.AnalyzeProject(id, "", 0, 0); err != nil {
				errs <- err
				return
			}
			time.Sleep(12 * time.Millisecond)
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if _, err := s.indexStore.current(id, root); err != nil {
		t.Fatal(err)
	}
	generations, err := os.ReadDir(filepath.Join(s.indexStore.environmentDir(id), "generations"))
	if err != nil || len(generations) > 2 {
		t.Fatalf("index generation cleanup: %d %v", len(generations), err)
	}
}
