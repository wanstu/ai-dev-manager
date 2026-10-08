package projectanalysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeIncrementalReusesUnchangedSourceAndRebuildsChangedFiles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "mod/service.go", "package mod\nfunc Serve() {}\n")
	writeFixture(t, root, "app/Controller.php", "<?php\nclass Controller { public function run() {} }\n")
	writeFixture(t, root, "web/widget.js", "export function draw() {}\n")

	full, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if full.IndexMode != "full" || full.ReindexedSourceFiles != 3 || full.ReusedSourceFiles != 0 {
		t.Fatalf("first generation: %+v", full)
	}
	installFixtureIndex(t, root, full)

	reused, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if reused.IndexMode != "incremental" || reused.ReusedSourceFiles != 3 ||
		reused.ReindexedSourceFiles != 0 || reused.SymbolsJSONL != full.SymbolsJSONL {
		t.Fatalf("no-change generation: %+v", reused)
	}
	installFixtureIndex(t, root, reused)

	// Same size and mtime: content digest, not coarse filesystem metadata,
	// must still force a new parse.
	changed := filepath.Join(root, "web", "widget.js")
	old, err := os.Stat(changed)
	if err != nil {
		t.Fatal(err)
	}
	previous := "export function draw() {}\n"
	next := "export function drop() {}\n"
	if len(previous) != len(next) {
		t.Fatal("fixture must preserve byte length")
	}
	if err := os.WriteFile(changed, []byte(next), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(changed, old.ModTime(), old.ModTime()); err != nil {
		t.Fatal(err)
	}
	updated, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.IndexMode != "incremental" || updated.ReusedSourceFiles != 2 ||
		updated.ReindexedSourceFiles != 1 || !strings.Contains(updated.SymbolsJSONL, `"name":"drop"`) ||
		strings.Contains(updated.SymbolsJSONL, `"name":"draw"`) {
		t.Fatalf("modified file generation: %+v", updated)
	}
	installFixtureIndex(t, root, updated)

	if err := os.Remove(filepath.Join(root, "app", "Controller.php")); err != nil {
		t.Fatal(err)
	}
	removed, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if removed.IndexMode != "incremental" || removed.ReusedSourceFiles != 2 ||
		removed.PHPFiles != 0 || strings.Contains(removed.SymbolsJSONL, "Controller") {
		t.Fatalf("removed source generation: %+v", removed)
	}
	installFixtureIndex(t, root, removed)

	// Freshness must be valid against the updated artifacts.
	status, err := IndexStatus(root, 20)
	if err != nil || status.State != "fresh" {
		t.Fatalf("updated index status=%+v err=%v", status, err)
	}
}

func TestAnalyzeIncrementalFallsBackOnInvalidOrIncompatibleIndex(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "main.go", "package demo\nfunc Run() {}\n")
	base, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, base)

	differentBounds, err := AnalyzeIncremental(root, Options{MaxFiles: 25, MaxSymbols: 25})
	if err != nil || differentBounds.IndexMode != "full" || differentBounds.ReindexedSourceFiles != 1 {
		t.Fatalf("different bounds=%+v err=%v", differentBounds, err)
	}
	corrupt := filepath.Join(root, filepath.FromSlash(IndexSymbolsRelativePath))
	if err := os.WriteFile(corrupt, []byte("malformed symbols\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recovered, err := AnalyzeIncremental(root, Options{})
	if err != nil || recovered.IndexMode != "full" || recovered.ReindexedSourceFiles != 1 {
		t.Fatalf("corrupt cache=%+v err=%v", recovered, err)
	}
	installFixtureIndex(t, root, recovered)

	var oldManifest IndexManifest
	if err := json.Unmarshal([]byte(recovered.ManifestJSON), &oldManifest); err != nil {
		t.Fatal(err)
	}
	oldManifest.SchemaVersion = IndexSchemaVersion - 1
	legacyJSON, err := json.Marshal(oldManifest)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, IndexManifestRelativePath, string(legacyJSON))
	legacyRebuilt, err := AnalyzeIncremental(root, Options{})
	if err != nil || legacyRebuilt.IndexMode != "full" || legacyRebuilt.ReindexedSourceFiles != 1 {
		t.Fatalf("old schema requires migration: %+v err=%v", legacyRebuilt, err)
	}
	installFixtureIndex(t, root, legacyRebuilt)

	// Files added after a successful generation should not make us reuse
	// stale indexes for those additions.
	writeFixture(t, root, "new.go", "package demo\nfunc New() {}\n")
	added, err := AnalyzeIncremental(root, Options{})
	if err != nil || added.IndexMode != "incremental" || added.ReusedSourceFiles != 1 ||
		added.ReindexedSourceFiles != 1 {
		t.Fatalf("added file=%+v err=%v", added, err)
	}
}

func TestAnalyzeIncrementalSingleFileSymbolLimitDoesNotCachePartialOutline(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "types.php", "<?php\nclass First {}\nclass Second {}\n")
	options := Options{MaxFiles: 20, MaxSymbols: 1}
	first, err := AnalyzeIncremental(root, options)
	if err != nil || !first.Truncated || first.Symbols != 1 {
		t.Fatalf("single-source limit=%+v err=%v", first, err)
	}
	installFixtureIndex(t, root, first)
	next, err := AnalyzeIncremental(root, options)
	if err != nil || next.IndexMode != "incremental" || next.ReusedSourceFiles != 0 ||
		next.ReindexedSourceFiles != 1 || !next.Truncated {
		t.Fatalf("partial single source must reindex: %+v err=%v", next, err)
	}
}

func TestAnalyzeIncrementalReusesCompleteFilesWhenSymbolBudgetIsReached(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.php", "<?php\nclass Alpha {}\n")
	writeFixture(t, root, "b.php", "<?php\nclass Beta {}\nclass Gamma {}\n")
	options := Options{MaxFiles: 20, MaxSymbols: 1}
	first, err := AnalyzeIncremental(root, options)
	if err != nil || !first.Truncated || first.Symbols != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	installFixtureIndex(t, root, first)
	next, err := AnalyzeIncremental(root, options)
	if err != nil || next.IndexMode != "incremental" || next.ReusedSourceFiles != 1 ||
		next.ReindexedSourceFiles != 1 || !next.Truncated || next.Symbols != 1 {
		t.Fatalf("symbol-bound prefix reuse=%+v err=%v", next, err)
	}
}

func TestAnalyzeIncrementalReusesCompletePrefixOfTruncatedIndex(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.go", "package demo\nfunc A() {}\n")
	writeFixture(t, root, "b.go", "package demo\nfunc B() {}\n")
	first, err := AnalyzeIncremental(root, Options{MaxFiles: 1, MaxSymbols: 10})
	if err != nil || !first.Truncated {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	installFixtureIndex(t, root, first)
	next, err := AnalyzeIncremental(root, Options{MaxFiles: 1, MaxSymbols: 10})
	if err != nil || next.IndexMode != "incremental" || next.ReusedSourceFiles != 1 ||
		next.ReindexedSourceFiles != 0 || !next.Truncated {
		t.Fatalf("truncated project should reuse complete indexed prefix: %+v err=%v", next, err)
	}
}
