package sandbox

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEnvironmentDropsSecretsAndRedirectsProfileAndCaches(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sandbox")
	dirs := DefaultRuntimeDirs(root)
	base := []string{
		"PATH=C:\\Tools",
		"SystemRoot=C:\\Windows",
		"GOROOT=C:\\Go",
		"GITHUB_TOKEN=secret",
		"AWS_SECRET_ACCESS_KEY=secret",
		"SSH_AUTH_SOCK=secret",
		"DATABASE_URL=secret",
		"NODE_OPTIONS=--require=C:\\outside.js",
		"USERPROFILE=C:\\Users\\real-user",
		"APPDATA=C:\\Users\\real-user\\AppData\\Roaming",
	}
	got, err := BuildEnvironment(base, dirs, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, forbidden := range []string{
		"GITHUB_TOKEN=",
		"AWS_SECRET_ACCESS_KEY=",
		"SSH_AUTH_SOCK=",
		"DATABASE_URL=",
		"NODE_OPTIONS=",
		"C:\\Users\\real-user",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("sandbox environment leaked %q:\n%s", forbidden, joined)
		}
	}
	for _, required := range []string{
		"PATH=C:\\Tools",
		"SYSTEMROOT=C:\\Windows",
		"GOROOT=C:\\Go",
		"USERPROFILE=" + dirs.ProfileRoot,
		"TEMP=" + dirs.TempRoot,
		"GOCACHE=" + filepath.Join(dirs.CacheRoot, "go-build"),
		"GIT_TERMINAL_PROMPT=0",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("sandbox environment missing %q:\n%s", required, joined)
		}
	}
}

func TestBuildEnvironmentAllowsTrustedADMOverrides(t *testing.T) {
	dirs := DefaultRuntimeDirs(filepath.Join(t.TempDir(), "sandbox"))
	got, err := BuildEnvironment([]string{"PATH=old"}, dirs, map[string]string{
		"PATH":             "new",
		"ADM_SANDBOX_MODE": "strong",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "PATH=new") || !strings.Contains(joined, "ADM_SANDBOX_MODE=strong") {
		t.Fatalf("trusted overrides missing:\n%s", joined)
	}
}

func TestRuntimeDirsAllowEnvironmentPersistentCacheAndRejectEscape(t *testing.T) {
	base := t.TempDir()
	environmentRoot := filepath.Join(base, "environment")
	dirs := DefaultRuntimeDirs(filepath.Join(environmentRoot, "execution"))
	dirs.CacheRoot = filepath.Join(environmentRoot, "cache")
	if err := dirs.Validate(); err != nil {
		t.Fatalf("Environment-persistent cache should be valid: %v", err)
	}

	dirs.CacheRoot = filepath.Join(base, "outside")
	if err := dirs.Validate(); err == nil {
		t.Fatal("expected cache root outside Environment sandbox root to be rejected")
	}
}
