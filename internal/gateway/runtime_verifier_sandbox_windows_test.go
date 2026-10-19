//go:build windows

package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"
)

func TestSandboxVerifierRunAsyncWindowsE2E(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Windows sandbox async verifier E2E runs in CI")
	}
	service, environmentID, _ := asyncVerifierTestService(t)
	environment, err := service.Environments.Get(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := service.AddVerifier(environmentID, model.VerifierDefinition{
		Name:                      "sandbox-async",
		Kind:                      verifier.KindTest,
		Enabled:                   true,
		AgentHostExecutionAllowed: false,
		Executable:                os.Args[0],
		Args:                      []string{"-test.run=^TestSandboxAsyncVerifierCommandHelper$", "-test.v"},
		TimeoutSeconds:            30,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()

	modePath := filepath.Join(environment.Root, "sandbox-async-mode.txt")
	if err := os.WriteFile(modePath, []byte("success"), 0o644); err != nil {
		t.Fatal(err)
	}
	started, err := owner.StartSandboxVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	succeeded := waitVerifierRunTerminal(t, owner, environmentID, started.ID, 10*time.Second)
	if succeeded.State != verifierRunSucceeded || succeeded.Result == nil || succeeded.Result.Status != verifier.StatusPassed {
		t.Fatalf("sandbox async success = %+v", succeeded)
	}
	if !strings.Contains(succeeded.Stdout, "sandbox-async-success") {
		t.Fatalf("sandbox async stdout missing: %+v", succeeded)
	}

	if err := os.WriteFile(modePath, []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	started, err = owner.StartSandboxVerifierRun(environmentID, asyncVerifierTestWriter, definition.ID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	waitVerifierRunOutput(t, owner, environmentID, started.ID, "sandbox-async-ready")
	canceled, err := owner.CancelVerifierRun(environmentID, asyncVerifierTestWriter, started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.State != verifierRunCanceled {
		t.Fatalf("sandbox async cancel = %+v", canceled)
	}
}

func TestSandboxAsyncVerifierCommandHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") != "1" {
		t.Skip("sandbox async verifier helper")
	}
	mode, err := os.ReadFile("sandbox-async-mode.txt")
	if err != nil {
		t.Fatal(err)
	}
	switch strings.TrimSpace(string(mode)) {
	case "success":
		fmt.Fprintln(os.Stdout, "sandbox-async-success")
	case "long":
		fmt.Fprintln(os.Stdout, "sandbox-async-ready")
		time.Sleep(30 * time.Second)
	default:
		t.Fatalf("unknown sandbox async mode %q", mode)
	}
}
