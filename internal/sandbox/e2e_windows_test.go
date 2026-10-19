//go:build windows

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const (
	sandboxE2EChildEnv         = "ADM_SANDBOX_E2E_CHILD"
	sandboxE2EAllowedEnv       = "ADM_SANDBOX_E2E_ALLOWED"
	sandboxE2EBlockedEnv       = "ADM_SANDBOX_E2E_BLOCKED"
	sandboxE2EResultEnv        = "ADM_SANDBOX_E2E_RESULT"
	sandboxE2ESecretEnv        = "ADM_SANDBOX_TEST_SECRET"
	sandboxE2EGrandchildPIDEnv = "ADM_SANDBOX_E2E_GRANDCHILD_PID"
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
	// The sandboxed child may create descendants of the same test binary.
	// Grant the image directory as a read-only toolchain mount so CreateProcess
	// can traverse and reopen the executable from inside AppContainer.
	grant(filepath.Dir(executable), PathAccessReadOnly)
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

	grandchildPIDPath := filepath.Join(allowedRoot, "grandchild.pid")
	jobDiagnosticPath := filepath.Join(allowedRoot, "job-diagnostic.txt")
	jobEnvironment, err := BuildEnvironment(base, dirs, map[string]string{
		sandboxE2EGrandchildPIDEnv: grandchildPIDPath,
		sandboxE2EResultEnv:        jobDiagnosticPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	jobCtx, cancelJob := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelJob()
	jobResult, launchErr := Launch(jobCtx, LaunchRequest{
		Executable:  executable,
		Args:        []string{"-test.run=^TestWindowsAppContainerJobChildHelper$", "-test.v"},
		Cwd:         allowedRoot,
		Environment: jobEnvironment,
		ProfileSID:  profile.SID,
	})
	if !errors.Is(launchErr, context.DeadlineExceeded) {
		diagnostic, _ := os.ReadFile(jobDiagnosticPath)
		t.Fatalf("expected AppContainer job cancellation deadline, got err=%v exit=%d diagnostic=%q", launchErr, jobResult.ExitCode, diagnostic)
	}
	pidData, err := os.ReadFile(grandchildPIDPath)
	if err != nil {
		t.Fatalf("grandchild did not publish PID before cancellation: %v", err)
	}
	pid64, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 10, 32)
	if err != nil {
		t.Fatalf("invalid grandchild PID %q: %v", pidData, err)
	}
	if err := waitWindowsProcessTerminated(uint32(pid64), 3*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsAppContainerJobChildHelper(t *testing.T) {
	pidPath := os.Getenv(sandboxE2EGrandchildPIDEnv)
	if pidPath == "" {
		t.Skip("AppContainer Job Object child helper")
	}
	diagnosticPath := os.Getenv(sandboxE2EResultEnv)
	fail := func(message string) {
		if diagnosticPath != "" {
			_ = os.WriteFile(diagnosticPath, []byte(message), 0o644)
		}
		t.Fatal(message)
	}
	executable, err := os.Executable()
	if err != nil {
		fail("resolve child executable: " + err.Error())
	}
	cmd := exec.Command(executable, "-test.run=^TestWindowsAppContainerGrandchildHelper$", "-test.v")
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		fail("start grandchild: " + err.Error())
	}
	if err := cmd.Wait(); err != nil {
		fail("wait grandchild: " + err.Error())
	}
	fail("grandchild exited before Job Object cancellation")
}

func TestWindowsAppContainerGrandchildHelper(t *testing.T) {
	pidPath := os.Getenv(sandboxE2EGrandchildPIDEnv)
	if pidPath == "" {
		t.Skip("AppContainer Job Object grandchild helper")
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Second)
}

func waitWindowsProcessTerminated(pid uint32, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				return nil
			}
			return fmt.Errorf("open grandchild process %d: %w", pid, err)
		}
		status, waitErr := windows.WaitForSingleObject(handle, 0)
		_ = windows.CloseHandle(handle)
		if waitErr != nil {
			return fmt.Errorf("wait grandchild process %d: %w", pid, waitErr)
		}
		if status == windows.WAIT_OBJECT_0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("grandchild process %d survived AppContainer Job Object cancellation", pid)
		}
		time.Sleep(50 * time.Millisecond)
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
