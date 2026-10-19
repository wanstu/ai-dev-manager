//go:build windows

package gateway

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

func TestAgentUnapprovedVerifierDefaultsToWindowsSandbox(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Windows Agent sandbox verifier E2E runs in CI")
	}
	service, environmentID, _ := asyncVerifierTestService(t)
	environment, err := service.Environments.Get(environmentID)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := service.AddVerifier(environmentID, model.VerifierDefinition{
		Name:                      "agent-sandbox-default",
		Kind:                      verifier.KindTest,
		Enabled:                   true,
		AgentHostExecutionAllowed: false,
		Executable:                os.Args[0],
		Args:                      []string{"-test.run=^TestAgentSandboxVerifierCommandHelper$", "-test.v"},
		TimeoutSeconds:            30,
	})
	if err != nil {
		t.Fatal(err)
	}

	modePath := filepath.Join(environment.Root, "agent-sandbox-mode.txt")
	if err := os.WriteFile(modePath, []byte("success"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_SANDBOX_TEST_SECRET", "must-not-cross-agent-boundary")

	ctx := context.Background()
	owner := newRuntimeOwner(service)
	defer owner.Close()
	agent := connectInMemory(t, ctx, newServer(service, owner))
	defer agent.Close()

	syncResult := callGatewayTool(t, ctx, agent, "environment_verifier_run", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    definition.ID,
		"max_output_bytes": 4096,
	})
	if syncResult.IsError {
		t.Fatalf("Agent sandbox verifier failed: %s", toolText(t, syncResult))
	}
	if !strings.Contains(toolText(t, syncResult), "agent-sandbox-success") {
		t.Fatalf("Agent sandbox verifier output missing: %s", toolText(t, syncResult))
	}

	if err := os.WriteFile(modePath, []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	started := callGatewayTool(t, ctx, agent, "environment_verifier_run_start", map[string]any{
		"environment_id": environmentID,
		"writer_owner":   asyncVerifierTestWriter,
		"verifier_id":    definition.ID,
		"max_output_bytes": 4096,
	})
	if started.IsError {
		t.Fatalf("Agent async sandbox verifier failed to start: %s", toolText(t, started))
	}
	runID := verifierRunIDFromToolText(t, started)
	waitVerifierHTTPOutput(t, ctx, agent, environmentID, runID, "agent-sandbox-ready")
	canceled := callGatewayTool(t, ctx, agent, "environment_verifier_run_cancel", map[string]any{
		"environment_id":  environmentID,
		"writer_owner":    asyncVerifierTestWriter,
		"verifier_run_id": runID,
	})
	if canceled.IsError {
		t.Fatalf("Agent async sandbox verifier cancel failed: %s", toolText(t, canceled))
	}
	if status := verifierRunStatusFromTool(t, canceled); status.State != verifierRunCanceled {
		t.Fatalf("Agent async sandbox verifier cancel status = %+v", status)
	}
}

func TestAgentSandboxVerifierCommandHelper(t *testing.T) {
	if os.Getenv("ADM_SANDBOX_ACTIVE") != "1" {
		t.Skip("Agent sandbox verifier helper")
	}
	if secret := os.Getenv("ADM_SANDBOX_TEST_SECRET"); secret != "" {
		t.Fatalf("host secret leaked into Agent sandbox: %q", secret)
	}
	mode, err := os.ReadFile("agent-sandbox-mode.txt")
	if err != nil {
		t.Fatal(err)
	}
	switch strings.TrimSpace(string(mode)) {
	case "success":
		fmt.Fprintln(os.Stdout, "agent-sandbox-success")
	case "long":
		fmt.Fprintln(os.Stdout, "agent-sandbox-ready")
		select {}
	default:
		t.Fatalf("unknown Agent sandbox mode %q", mode)
	}
}