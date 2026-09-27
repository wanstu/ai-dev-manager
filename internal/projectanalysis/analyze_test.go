package projectanalysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeGoAndPHPProject(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	goSource := "package demo\n\ntype Service struct{}\n\nfunc NewService() *Service { return &Service{} }\nfunc (s *Service) Run() {}\n"
	if err := os.WriteFile(filepath.Join(root, "internal", "demo", "service.go"), []byte(goSource), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "composer.json"), []byte("{\"name\":\"example/demo\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app", "Http"), 0o755); err != nil {
		t.Fatal(err)
	}
	phpSource := "<?php\nnamespace App\\Http;\nclass UserController {\n    public function show() {}\n}\nfunction helper() {}\n"
	if err := os.WriteFile(filepath.Join(root, "app", "Http", "UserController.php"), []byte(phpSource), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "vendor", "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "ignored", "Skip.php"), []byte("<?php class Skip {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if result.GoModule != "example.com/demo" {
		t.Fatalf("go module=%q", result.GoModule)
	}
	if result.ComposerPackage != "example/demo" {
		t.Fatalf("composer package=%q", result.ComposerPackage)
	}
	if result.GoFiles != 1 || result.PHPFiles != 1 {
		t.Fatalf("file counts: go=%d php=%d", result.GoFiles, result.PHPFiles)
	}
	if strings.Join(result.Languages, ",") != "Go,PHP" {
		t.Fatalf("languages=%v", result.Languages)
	}
	for _, want := range []string{
		"example.com/demo",
		"example/demo",
		"internal/demo/service.go",
		"Service",
		"NewService",
		"Service.Run",
		"app/Http/UserController.php",
		"App\\Http",
		"UserController",
		"show",
		"helper",
	} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("overview missing %q:\n%s", want, result.Markdown)
		}
	}
	if strings.Contains(result.Markdown, "Skip") {
		t.Fatalf("vendor content should be ignored:\n%s", result.Markdown)
	}
	if result.FilesIndexed != 4 {
		t.Fatalf("files indexed=%d", result.FilesIndexed)
	}
	for _, want := range []string{IndexManifestRelativePath, IndexFilesRelativePath, IndexSymbolsRelativePath} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("overview missing machine index path %q:\n%s", want, result.Markdown)
		}
	}
	if !strings.Contains(result.FilesJSONL, `"path":"internal/demo/service.go"`) || !strings.Contains(result.FilesJSONL, `"sha256":"`) {
		t.Fatalf("file index missing source metadata:\n%s", result.FilesJSONL)
	}
	for _, want := range []string{`"qualified_name":"demo.Service.Run"`, `"qualified_name":"App\\Http\\UserController"`} {
		if !strings.Contains(result.SymbolsJSONL, want) {
			t.Fatalf("symbol index missing %q:\n%s", want, result.SymbolsJSONL)
		}
	}
	var manifest IndexManifest
	if err := json.Unmarshal([]byte(result.ManifestJSON), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != IndexSchemaVersion || manifest.FilesIndexed != 4 || manifest.SymbolsIndexed != result.Symbols {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	for _, key := range []string{"overview", "files", "symbols"} {
		artifact := manifest.Artifacts[key]
		if artifact.Path == "" || artifact.SHA256 == "" || artifact.Bytes <= 0 {
			t.Fatalf("manifest artifact %q incomplete: %+v", key, artifact)
		}
	}
}

func TestAnalyzeHonorsBounds(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package demo\nfunc One() {}\nfunc Two() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Analyze(root, Options{MaxFiles: 2, MaxSymbols: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("expected bounded analysis to report truncation")
	}
	if result.FilesScanned != 2 {
		t.Fatalf("files scanned=%d", result.FilesScanned)
	}
	if result.Symbols != 1 {
		t.Fatalf("symbols=%d", result.Symbols)
	}
}
