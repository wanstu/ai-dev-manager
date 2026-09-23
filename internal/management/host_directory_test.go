package management

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHostDirectoryListReturnsDirectoriesOnly(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	listing, err := listHostDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(listing.Path) != filepath.Clean(root) {
		t.Fatalf("path=%q want=%q", listing.Path, root)
	}
	if len(listing.Directories) != 2 {
		t.Fatalf("directories=%v", listing.Directories)
	}
	if listing.Directories[0].Name != "alpha" || listing.Directories[1].Name != "beta" {
		t.Fatalf("unexpected directories=%v", listing.Directories)
	}
	if listing.Parent == "" {
		t.Fatal("temporary directory should expose a parent")
	}
}

func TestHostDirectoryListRejectsRelativeAndFiles(t *testing.T) {
	if _, err := listHostDirectories("relative/path"); err == nil {
		t.Fatal("relative path must be rejected")
	}
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := listHostDirectories(file); err == nil {
		t.Fatal("file path must be rejected")
	}
}

func TestHostDirectoryListEmptyPathStartsAtFilesystemRoot(t *testing.T) {
	listing, err := listHostDirectories("")
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if listing.Path != "" {
			t.Fatalf("windows roots listing path=%q", listing.Path)
		}
		if len(listing.Directories) == 0 {
			t.Fatal("expected at least one Windows drive")
		}
		return
	}
	if listing.Path != string(filepath.Separator) {
		t.Fatalf("root path=%q", listing.Path)
	}
}
