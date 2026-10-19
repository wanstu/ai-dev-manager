//go:build windows

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"
)

func TestRunVerifierSandboxedWindowsE2E(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Windows sandbox verifier E2E runs in CI")
	}
	root := t.TempDir()
	service := New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "sandbox-verifier")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "sandbox-verifier", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "sandbox-verifier-writer"
	if _, err := service.Environments.AcquireWriter(env.ID, writer); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(executable); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	blockedRoot, err := os.MkdirTemp(home, "adm-app-sandbox-blocked-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(blockedRoot)
	blockedFile := filepath.Join(blockedRoot, "secret.txt")
	if err := os.WriteFile(blockedFile, []byte("host-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "blocked-path.txt"), []byte(blockedFile), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ADM_SANDBOX_TEST_SECRET", "must-not-cross")
	definition, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Name:           "sandbox-e2e",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     executable,
		Args:           []string{"-test.run=^TestSandboxVerifierChildHelper$", "-test.v"},
		TimeoutSeconds: 15,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.RunVerifierSandboxed(context.Background(), env.ID, writer, definition.ID, 64*1024)
	if err != nil {
		t.Fatalf("sandbox verifier failed: %v", err)
	}
	if result.Status != verifier.StatusPassed || result.ExitCode != 0 {
		t.Fatalf("unexpected sandbox verifier result: %+v", result)
	}
	if !strings.Contains(result.Stdout, "SANDBOX_VERIFIER_OK") {
		t.Fatalf("sandbox stdout was not captured: %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "SANDBOX_VERIFIER_STDERR") {
		t.Fatalf("sandbox stderr was not captured: %q", result.Stderr)
	}
	generated, err := os.ReadFile(filepath.Join(root, "generated.txt"))
	if err != nil {
		t.Fatalf("sandbox could not write Environment root: %v", err)
	}
	if string(generated) != "sandbox-write-ok" {
		t.Fatalf("sandbox generated content = %q", generated)
	}
}

func TestSandboxVerifierChildHelper(t *testing.T) {
	if os.Getenv(sandboxVerifierActiveEnv) != "1" {
		t.Skip("sandbox verifier child helper")
	}
	if secret := os.Getenv("ADM_SANDBOX_TEST_SECRET"); secret != "" {
		t.Fatalf("host secret leaked into sandbox environment: %q", secret)
	}
	blockedPath, err := os.ReadFile("blocked-path.txt")
	if err != nil {
		t.Fatalf("read blocked path fixture: %v", err)
	}
	if data, err := os.ReadFile(strings.TrimSpace(string(blockedPath))); err == nil {
		t.Fatalf("sandbox read ungranted host file: %q", data)
	}
	if err := os.WriteFile("generated.txt", []byte("sandbox-write-ok"), 0o644); err != nil {
		t.Fatalf("write Environment root: %v", err)
	}
	fmt.Fprintln(os.Stdout, "SANDBOX_VERIFIER_OK")
	fmt.Fprintln(os.Stderr, "SANDBOX_VERIFIER_STDERR")
}
