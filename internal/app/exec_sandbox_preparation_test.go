package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPrepareSandboxExecutionAuthorizesBeforeElevationRequest(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "sandbox-auth-order")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "sandbox-auth-order", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "sandbox-auth-order-writer"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetExecFullAuthorization(false); err != nil {
		t.Fatal(err)
	}

	elevationRequests := 0
	service.SetSandboxEnvironmentElevationRequester(func(context.Context, string) error {
		elevationRequests++
		return nil
	})

	if _, err := service.PrepareSandboxExecution(
		context.Background(),
		environment.ID,
		writer,
		"adm-test-executable-that-is-not-allowlisted",
		nil,
		"",
		"test",
	); err == nil {
		t.Fatal("expected non-allowlisted sandbox command to be rejected")
	}
	if elevationRequests != 0 {
		t.Fatalf("sandbox elevation was requested before command authorization: %d", elevationRequests)
	}
}
