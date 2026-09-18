package configpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirUsesHomeConfigADM(t *testing.T) {
	home := t.TempDir()
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
