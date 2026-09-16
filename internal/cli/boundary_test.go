package cli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const cliImportPath = "ai-dev-manager-v2/internal/cli"
const desktopImportPath = "ai-dev-manager-v2/internal/desktop"

func TestSurfaceDependencyDirection(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	assertTreeDoesNotImport(t, filepath.Join(repoRoot, "internal"), cliImportPath, filepath.Join(repoRoot, "internal", "cli"))
	assertTreeDoesNotImport(t, filepath.Join(repoRoot, "cmd", "ai-dev-manager-desktop"), cliImportPath, "")
	assertTreeDoesNotImport(t, filepath.Join(repoRoot, "internal", "cli"), desktopImportPath, "")
}

func assertTreeDoesNotImport(t *testing.T, root, forbiddenImport, excludedRoot string) {
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
