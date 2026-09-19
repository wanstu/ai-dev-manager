package configpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirUsesHomeConfigADM(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	if os.Getenv("USERPROFILE") != "" {
		t.Setenv("USERPROFILE", home)
	}
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "adm")
	if got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}

func TestDirUsesKitXDGConfigHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "adm")
	if got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}
