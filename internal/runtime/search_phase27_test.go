package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestSearchSupportsSingleFilePathAndDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.txt"), []byte("alpha\nneedle-one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "two.txt"), []byte("needle-two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runtime := mustNative(t, root, model.Policy{Mode: string(ModeReadOnly)})

	fileMatches, err := runtime.Search(SearchOptions{Path: "one.txt", Query: "needle"})
	if err != nil {
		t.Fatalf("single-file Search() error = %v", err)
	}
	if len(fileMatches) != 1 || fileMatches[0].Path != "one.txt" || fileMatches[0].Line != 2 || fileMatches[0].Text != "needle-one" {
		t.Fatalf("single-file matches = %+v", fileMatches)
	}

	directoryMatches, err := runtime.Search(SearchOptions{Path: ".", Query: "needle"})
	if err != nil {
		t.Fatalf("directory Search() error = %v", err)
	}
	if len(directoryMatches) != 2 || directoryMatches[0].Path != "nested\\two.txt" && directoryMatches[0].Path != "nested/two.txt" && directoryMatches[1].Path != "nested\\two.txt" && directoryMatches[1].Path != "nested/two.txt" {
		t.Fatalf("directory matches = %+v", directoryMatches)
	}
}
