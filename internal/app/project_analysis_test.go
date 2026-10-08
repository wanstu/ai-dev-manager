package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/projectanalysis"
)

func TestAnalyzeProjectWritesOverviewAndMachineIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/indexed\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\ntype App struct{}\nfunc (a *App) Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "indexed")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "indexed", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "project-index-test"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}

	result, err := service.AnalyzeProject(environment.ID, writer, 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Markdown != "" || result.ManifestJSON != "" || result.FilesJSONL != "" || result.SymbolsJSONL != "" || result.CallsJSONL != "" {
		t.Fatal("AnalyzeProject response must not inline generated artifact contents")
	}

	if _,err:=os.Stat(filepath.Join(root,".adm"));!os.IsNotExist(err){
		t.Fatalf("index must not write into source root: %v",err)
	}
	indexRoot,err:=service.indexStore.current(environment.ID,root)
	if err!=nil{t.Fatal(err)}
	for _, rel := range []string{
		projectanalysis.OverviewRelativePath,
		projectanalysis.IndexManifestRelativePath,
		projectanalysis.IndexFilesRelativePath,
		projectanalysis.IndexSymbolsRelativePath,
		projectanalysis.IndexCallsRelativePath,
	} {
		if _, err := os.Stat(filepath.Join(indexRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("generated artifact %s: %v", rel, err)
		}
	}

	overview, err := os.ReadFile(filepath.Join(indexRoot, filepath.FromSlash(projectanalysis.OverviewRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overview), projectanalysis.IndexSymbolsRelativePath) {
		t.Fatalf("overview does not point at symbol index:\n%s", overview)
	}

	manifestData, err := os.ReadFile(filepath.Join(indexRoot, filepath.FromSlash(projectanalysis.IndexManifestRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest projectanalysis.IndexManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != projectanalysis.IndexSchemaVersion || manifest.SymbolsIndexed < 2 ||
		manifest.Artifacts["calls"].Path != projectanalysis.IndexCallsRelativePath {
		t.Fatalf("manifest=%+v", manifest)
	}

	symbols, err := os.ReadFile(filepath.Join(indexRoot, filepath.FromSlash(projectanalysis.IndexSymbolsRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(symbols), "\"qualified_name\":\"main.App.Run\"") {
		t.Fatalf("symbol index missing method:\n%s", symbols)
	}
	if _, err := service.Environments.ReleaseWriter(environment.ID, writer, false); err != nil {
		t.Fatal(err)
	}
	queried, err := service.ProjectIndexQuery(environment.ID, projectanalysis.IndexQuery{Query: "App.Run", MaxResults: 10})
	if err != nil {
		t.Fatal(err)
	}
	if queried.Returned != 1 || queried.Matches[0].Path != "main.go" || queried.Matches[0].Line != 4 {
		t.Fatalf("query result=%+v", queried)
	}
}
