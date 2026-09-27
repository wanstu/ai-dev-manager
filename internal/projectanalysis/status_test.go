package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexStatusFreshThenStaleAfterSourceChange(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "service.go")
	if err := os.WriteFile(source, []byte("package demo\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(root, Options{MaxFiles: 100, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeStatusArtifacts(t, root, result)

	fresh, err := IndexStatus(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.State != "fresh" || !fresh.Fresh || !fresh.Complete || !fresh.ArtifactVerified || fresh.ChangeCount != 0 {
		t.Fatalf("fresh status=%+v", fresh)
	}

	if err := os.WriteFile(source, []byte("package demo\nfunc Run() { println(\"changed\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := IndexStatus(root, 20)
	if err != nil {
		t.Fatal(err)
	}
	if stale.State != "stale" || stale.Fresh || stale.ChangeCount != 1 {
		t.Fatalf("stale status=%+v", stale)
	}
	if stale.Changes[0].Path != "service.go" || stale.Changes[0].Change != "modified" {
		t.Fatalf("stale changes=%+v", stale.Changes)
	}
}

func TestIndexStatusDetectsAddedRemovedAndBoundsChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("package demo\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(root, Options{MaxFiles: 100, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeStatusArtifacts(t, root, result)

	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "c.go"), []byte("package demo\nfunc C() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := IndexStatus(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "stale" || status.ChangeCount != 2 || !status.ChangesTruncated || len(status.Changes) != 1 {
		t.Fatalf("status=%+v", status)
	}
}

func TestIndexStatusInvalidMissingAndPartial(t *testing.T) {
	missingRoot := t.TempDir()
	missing, err := IndexStatus(missingRoot, 10)
	if err != nil {
		t.Fatal(err)
	}
	if missing.State != "missing" {
		t.Fatalf("missing=%+v", missing)
	}

	invalidRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidRoot, "a.go"), []byte("package demo\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalidResult, err := Analyze(invalidRoot, Options{MaxFiles: 100, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeStatusArtifacts(t, invalidRoot, invalidResult)
	symbolPath := filepath.Join(invalidRoot, filepath.FromSlash(IndexSymbolsRelativePath))
	if err := os.WriteFile(symbolPath, []byte("{\"broken\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid, err := IndexStatus(invalidRoot, 10)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.State != "invalid" || invalid.ArtifactVerified || len(invalid.Reasons) == 0 || !strings.Contains(invalid.Reasons[0], "hash") {
		t.Fatalf("invalid=%+v", invalid)
	}

	partialRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(partialRoot, "a.go"), []byte("package demo\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partialRoot, "b.go"), []byte("package demo\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	partialResult, err := Analyze(partialRoot, Options{MaxFiles: 1, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeStatusArtifacts(t, partialRoot, partialResult)
	partial, err := IndexStatus(partialRoot, 10)
	if err != nil {
		t.Fatal(err)
	}
	if partial.State != "partial" || partial.Fresh || partial.Complete {
		t.Fatalf("partial=%+v", partial)
	}
}

func writeStatusArtifacts(t *testing.T, root string, result Result) {
	t.Helper()
	for _, item := range []struct{ path, content string }{
		{OverviewRelativePath, result.Markdown},
		{IndexManifestRelativePath, result.ManifestJSON},
		{IndexFilesRelativePath, result.FilesJSONL},
		{IndexSymbolsRelativePath, result.SymbolsJSONL},
	} {
		path := filepath.Join(root, filepath.FromSlash(item.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(item.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
