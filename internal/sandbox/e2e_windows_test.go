//go:build windows

package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	sandboxE2EChildEnv   = "ADM_SANDBOX_E2E_CHILD"
	sandboxE2EAllowedEnv = "ADM_SANDBOX_E2E_ALLOWED"
	sandboxE2EBlockedEnv = "ADM_SANDBOX_E2E_BLOCKED"
	sandboxE2EResultEnv  = "ADM_SANDBOX_E2E_RESULT"
	sandboxE2ESecretEnv  = "ADM_SANDBOX_TEST_SECRET"
)

func TestWindowsAppContainerFileBoundaryE2E(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Windows AppContainer E2E runs in CI to avoid creating sandbox profiles during ordinary local unit tests")
	}
	if !WindowsAppContainerAvailable() {
		t.Fatal("Windows AppContainer APIs are unavailable")
	}

	profileName, err := StableAppContainerProfileName(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile, _, err := EnsureWindowsAppContainerProfile(profileName)
	if err != nil {
		t.Fatalf("create AppContainer profile: %v", err)
	}
	defer func() {
		if err := DeleteWindowsAppContainerProfile(profileName); err != nil {
			t.Errorf("delete AppContainer profile: %v", err)
		}
	}()

	granted := make([]string, 0, 3)
	grant := func(path string, access PathAccess) {
		t.Helper()
		if err := GrantWindowsAppContainerPath(path, profile.SID, access); err != nil {
			t.Fatalf("grant AppContainer path %s: %v", path, err)
		}
		granted = append(granted, path)
	}
	defer func() {
		for i := len(granted) - 1; i >= 0; i-- {
			if err := RevokeWindowsAppContainerPath(granted[i], profile.SID); err != nil {
				t.Errorf("revoke AppContainer path %s: %v", granted[i], err)
			}
		}
	}()

	allowedRoot := filepath.Join(t.TempDir(), "allowed")
	if err := os.MkdirAll(allowedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	grant(allowedRoot, PathAccessReadWrite)
	allowedInput := filepath.Join(allowedRoot, "input.txt")
	allowedResult := filepath.Join(allowedRoot, "result.txt")
	if err := os.WriteFile(allowedInput, []byte("allowed-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	blockedRoot, err := os.MkdirTemp(home, "adm-sandbox-e2e-blocked-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(blockedRoot)
	blockedFile := filepath.Join(blockedRoot, "secret.txt")
	if err := os.WriteFile(blockedFile, []byte("host-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	grant(runtimeRoot, PathAccessReadWrite)
	dirs := DefaultRuntimeDirs(runtimeRoot)
	for _, dir := range []string{
		dirs.ProfileRoot,
		filepath.Join(dirs.ProfileRoot, "AppData", "Roaming"),
		filepath.Join(dirs.ProfileRoot, "AppData", "Local"),
		dirs.TempRoot,
		dirs.CacheRoot,
		dirs.BuildRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	grant(executable, PathAccessReadOnly)

	base := append([]string(nil), os.Environ()...)
	base = append(base, sandboxE2ESecretEnv+"=must-not-cross-boundary")
	environment, err := BuildEnvironment(base, dirs, map[string]string{
		sandboxE2EChildEnv:   "1",
		sandboxE2EAllowedEnv: allowedInput,
		sandboxE2EBlockedEnv: blockedFile,
		sandboxE2EResultEnv:  allowedResult,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, launchErr := Launch(context.Background(), LaunchRequest{
		Executable:  executable,
		Args:        []string{"-test.run=^TestWindowsAppContainerChildHelper$", "-test.v"},
		Cwd:         allowedRoot,
		Environment: environment,
		ProfileSID:  profile.SID,
	})
	diagnostic, _ := os.ReadFile(allowedResult)
	if launchErr != nil {
		t.Fatalf("AppContainer launch failed: %v; child diagnostic=%q", launchErr, diagnostic)
	}
	if result.ExitCode != 0 {
		t.Fatalf("AppContainer child exit code=%d diagnostic=%q", result.ExitCode, diagnostic)
	}
	if strings.TrimSpace(string(diagnostic)) != "ok" {
		t.Fatalf("unexpected AppContainer child result %q", diagnostic)
	}
}

func TestWindowsAppContainerChildHelper(t *testing.T) {
	if os.Getenv(sandboxE2EChildEnv) != "1" {
		t.Skip("AppContainer E2E child helper")
	}
	resultPath := os.Getenv(sandboxE2EResultEnv)
	fail := func(message string) {
		_ = os.WriteFile(resultPath, []byte(message), 0o644)
		t.Fatal(message)
	}

	if value := os.Getenv(sandboxE2ESecretEnv); value != "" {
		fail("host secret leaked into sandbox environment")
	}

	allowedPath := os.Getenv(sandboxE2EAllowedEnv)
	data, err := os.ReadFile(allowedPath)
	if err != nil {
		fail("cannot read explicitly granted file: " + err.Error())
	}
	if string(data) != "allowed-data" {
		fail("unexpected granted file contents")
	}

	blockedPath := os.Getenv(sandboxE2EBlockedEnv)
	if blockedData, err := os.ReadFile(blockedPath); err == nil {
		fail("sandbox read ungranted host file: " + string(blockedData))
	} else if errors.Is(err, os.ErrNotExist) {
		// Namespace/path hiding is also an acceptable denial outcome.
	}

	if err := os.WriteFile(resultPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
}
