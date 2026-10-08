package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistedPHPCallsIncrementalRefreshAndQuery(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/Service.php", `<?php
class Service {
 public function process() {}
}
`)
	writeFixture(t, root, "src/Caller.php", `<?php
class Caller {
 public function action($service) {
  Service::process();
  $service->process();
  // Service::process();
  $text = "Service::process()";
 }
}
`)
	opts := Options{MaxFiles: 100, MaxSymbols: 100}
	first, err := AnalyzeIncremental(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.CallsIndexed == 0 || first.ReindexedPHPCallFiles != 2 || first.IndexCallsPath != IndexCallsRelativePath {
		t.Fatalf("first=%+v", first)
	}
	installFixtureIndex(t, root, first)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Service::process", Path: "src/Service.php", Kind: "method"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.References) != 2 || refs.References[0].Kind != "resolved_call" || refs.References[1].Kind != "candidate_call" {
		t.Fatalf("stored calls=%+v", refs)
	}
	next, err := AnalyzeIncremental(root, opts)
	if err != nil || next.ReusedPHPCallFiles != 2 || next.ReindexedPHPCallFiles != 0 ||
		next.CallsJSONL != first.CallsJSONL {
		t.Fatalf("reused calls=%+v err=%v", next, err)
	}
	installFixtureIndex(t, root, next)
	writeFixture(t, root, "src/Caller.php", `<?php
class Caller {
 public function action($service) {
  Service::process();
 }
}
`)
	updated, err := AnalyzeIncremental(root, opts)
	if err != nil || updated.ReusedPHPCallFiles != 1 || updated.ReindexedPHPCallFiles != 1 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	installFixtureIndex(t, root, updated)
	refreshed, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Service::process", Kind: "method", Path: "src/Service.php"})
	if err != nil || len(refreshed.References) != 1 || refreshed.References[0].Kind != "resolved_call" {
		t.Fatalf("refreshed=%+v err=%v", refreshed, err)
	}
	if err := os.Remove(filepath.Join(root, "src", "Caller.php")); err != nil {
		t.Fatal(err)
	}
	removed, err := AnalyzeIncremental(root, opts)
	if err != nil || removed.ReusedPHPCallFiles != 1 || removed.ReindexedPHPCallFiles != 0 {
		t.Fatalf("removed=%+v err=%v", removed, err)
	}
	installFixtureIndex(t, root, removed)
	empty, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Service::process", Kind: "method", Path: "src/Service.php"})
	if err != nil || len(empty.References) != 0 {
		t.Fatalf("removed calls=%+v err=%v", empty, err)
	}
}

func TestPersistedPHPCallIndexIntegrityAndMetadataFreshness(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Service.php", `<?php
class Service {public function run() {}}
Service::run();
`)
	first, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	query := PHPReferenceQuery{Name: "Service::run", Kind: "method", Path: "Service.php"}
	if _, err := FindPHPCallReferences(root, query); err != nil {
		t.Fatal(err)
	}
	// Corrupt just the call index: both query and incremental reuse reject it.
	writeFixture(t, root, IndexCallsRelativePath, "{}\n")
	if _, err := FindPHPCallReferences(root, query); err == nil || !strings.Contains(err.Error(), "calls hash mismatch") {
		t.Fatalf("call artifact tampering undetected: %v", err)
	}
	rebuilt, err := AnalyzeIncremental(root, Options{})
	if err != nil || rebuilt.IndexMode != "full" || rebuilt.ReusedPHPCallFiles != 0 {
		t.Fatalf("tamper must force full build: %+v err=%v", rebuilt, err)
	}
	installFixtureIndex(t, root, rebuilt)
	writeFixture(t, root, "Service.php", `<?php
class Service {public function run() {}}
Service::run();
Service::run();
`)
	if _, err := FindPHPCallReferences(root, query); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("edited source should not appear fresh: %v", err)
	}
}

func TestPersistedPHPFunctionReferences(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "functions.php", `<?php
function helper() {}
// helper();
$text = "helper()";
$helper();
helper();
call_user_func("helper");
`)
	first, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "helper", Kind: "function"})
	if err != nil || len(refs.References) != 1 || refs.References[0].Kind != "candidate_call" {
		t.Fatalf("free function references=%+v err=%v", refs, err)
	}
}
