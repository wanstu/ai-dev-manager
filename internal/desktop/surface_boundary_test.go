package desktop

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const desktopImportPath = "ai-dev-manager-v2/internal/desktop"
const cliSurfaceImportPath = "ai-dev-manager-v2/internal/cli"

func TestDesktopSurfaceDependencyDirection(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	assertNoImport(t, filepath.Join(repoRoot, "internal"), desktopImportPath, filepath.Join(repoRoot, "internal", "desktop"))
	assertNoImport(t, filepath.Join(repoRoot, "internal", "desktop"), cliSurfaceImportPath, "")
}

func TestReleaseArtifactNamesRemainStable(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	assertFileContains(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"), `adm$tagSuffix-${{ matrix.name }}${{ matrix.ext }}`)
	assertFileContains(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"), `adm-desktop$tagSuffix-windows-amd64.exe`)
	assertFileContains(t, filepath.Join(repoRoot, "scripts", "build-desktop.ps1"), `[string]$OutputName = 'adm-desktop-windows-amd64.exe'`)
}

func assertNoImport(t *testing.T, root, forbiddenImport, excludedRoot string) {
	t.Helper()
	excludedRoot = filepath.Clean(excludedRoot)
	err := filepath.Walk(root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cleanPath := filepath.Clean(path)
		if info.IsDir() {
			if excludedRoot != "." && excludedRoot != "" && cleanPath == excludedRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if importPath == forbiddenImport {
				t.Fatalf("surface dependency violation: %s imports %s", cleanPath, forbiddenImport)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertFileContains(t *testing.T, path, required string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), required) {
		t.Fatalf("%s no longer contains stable release contract %q", path, required)
	}
}
