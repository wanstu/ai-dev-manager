package cli

import "testing"

func TestCLIAdminMCPManagesVerifierAndWriter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ADM_V2_HOME", home)
	service := startCLIAdminTestServer(t, home)

	workspace, err := service.Workspaces.Add(t.TempDir(), "admin-cli")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "main", "")
	if err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		if err := run([]string{"environment", "verifier", "add", "--environment-id", environment.ID, "--kind", "test", "--executable", "go", "--name", "go-test"}); err != nil {
			t.Fatal(err)
		}
	})
	verifiers, err := service.ListVerifiers(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(verifiers) != 1 || verifiers[0].Name != "go-test" || verifiers[0].Executable != "go" {
		t.Fatalf("verifier was not managed through Admin MCP: %+v", verifiers)
	}

	captureStdout(t, func() {
		if err := run([]string{"environment", "writer", "acquire", "--environment-id", environment.ID, "--owner", "cli-admin-test"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := service.Environments.RequireWriter(environment.ID, "cli-admin-test"); err != nil {
		t.Fatalf("writer lease was not acquired through Admin MCP: %v", err)
	}
	summary, err := service.EnvironmentSummary(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Writer == nil || summary.Writer.Owner != "" {
		t.Fatalf("management summary should expose an active lease without its owner: %+v", summary.Writer)
	}

	captureStdout(t, func() {
		if err := run([]string{"environment", "writer", "release", "--environment-id", environment.ID, "--owner", "cli-admin-test"}); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"environment", "verifier", "remove", "--environment-id", environment.ID, "--verifier-id", verifiers[0].ID}); err != nil {
			t.Fatal(err)
		}
	})
	summary, err = service.EnvironmentSummary(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Writer != nil {
		t.Fatalf("writer lease was not released through Admin MCP: %+v", summary.Writer)
	}
	verifiers, err = service.ListVerifiers(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(verifiers) != 0 {
		t.Fatalf("verifier was not removed through Admin MCP: %+v", verifiers)
	}
}
