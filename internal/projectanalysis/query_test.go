package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueryIndexFindsAndFiltersSymbols(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/query\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "demo", "service.go"), []byte("package demo\n\ntype Service struct{}\nfunc NewService() *Service { return &Service{} }\nfunc (s *Service) Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app", "Http"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "Http", "UserController.php"), []byte("<?php\nnamespace App\\Http;\nclass UserController {}\nfunction helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Analyze(root, Options{MaxFiles: 100, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeQueryArtifacts(t, root, result)

	byQualified, err := QueryIndex(root, IndexQuery{Query: "demo.Service.Run"})
	if err != nil {
		t.Fatal(err)
	}
	if byQualified.Returned != 1 || byQualified.Matches[0].QualifiedName != "demo.Service.Run" || byQualified.Matches[0].Match != "qualified_exact" {
		t.Fatalf("qualified query=%+v", byQualified)
	}
	if !byQualified.ArtifactVerified {
		t.Fatal("query must verify symbol artifact hash")
	}

	bySuffix, err := QueryIndex(root, IndexQuery{Query: "Service.Run"})
	if err != nil {
		t.Fatal(err)
	}
	if bySuffix.Returned != 1 || bySuffix.Matches[0].Match != "name_exact" {
		t.Fatalf("suffix query=%+v", bySuffix)
	}

	byFilter, err := QueryIndex(root, IndexQuery{Path: "internal/demo", Kind: "method", Language: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if byFilter.Returned != 1 || byFilter.Matches[0].Name != "Service.Run" {
		t.Fatalf("filtered query=%+v", byFilter)
	}

	php, err := QueryIndex(root, IndexQuery{Query: "usercontroller", Exact: true, Language: "PHP"})
	if err != nil {
		t.Fatal(err)
	}
	if php.Returned != 1 || php.Matches[0].QualifiedName != `App\Http\UserController` {
		t.Fatalf("php query=%+v", php)
	}
}

func TestQueryIndexBoundsAndRejectsTamperedSymbols(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package demo\nfunc Alpha() {}\nfunc Alpine() {}\nfunc Albatross() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(root, Options{MaxFiles: 100, MaxSymbols: 100})
	if err != nil {
		t.Fatal(err)
	}
	writeQueryArtifacts(t, root, result)

	bounded, err := QueryIndex(root, IndexQuery{Query: "Al", MaxResults: 2})
	if err != nil {
		t.Fatal(err)
	}
	if bounded.Returned != 2 || !bounded.Truncated {
		t.Fatalf("bounded query=%+v", bounded)
	}

	symbolPath := filepath.Join(root, filepath.FromSlash(IndexSymbolsRelativePath))
	data, err := os.ReadFile(symbolPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(symbolPath, append(data, []byte("{\"tampered\":true}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = QueryIndex(root, IndexQuery{Query: "Alpha"})
	if err == nil || !strings.Contains(err.Error(), "hash does not match") {
		t.Fatalf("tampered query err=%v", err)
	}
}

func TestQueryIndexRequiresGeneratedIndexAndFilter(t *testing.T) {
	root := t.TempDir()
	if _, err := QueryIndex(root, IndexQuery{Query: "Service"}); err == nil || !strings.Contains(err.Error(), "run project_analyze first") {
		t.Fatalf("missing index err=%v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(".adm/index")), 0o755); err != nil {
		t.Fatal(err)
	}
	result := Result{Markdown: "# overview\n"}
	manifest, files, symbols, calls, err := buildIndexArtifacts(result, Options{MaxFiles: 1, MaxSymbols: 1}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	result.ManifestJSON, result.FilesJSONL, result.SymbolsJSONL, result.CallsJSONL = manifest, files, symbols, calls
	writeQueryArtifacts(t, root, result)
	if _, err := QueryIndex(root, IndexQuery{}); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("empty query err=%v", err)
	}
}

func writeQueryArtifacts(t *testing.T, root string, result Result) {
	t.Helper()
	for _, item := range []struct{ path, content string }{
		{IndexManifestRelativePath, result.ManifestJSON},
		{IndexFilesRelativePath, result.FilesJSONL},
		{IndexSymbolsRelativePath, result.SymbolsJSONL},
		{IndexCallsRelativePath, result.CallsJSONL},
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
