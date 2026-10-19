package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/verifier"

)

func TestAgentVerifierUsesSandboxOrExplicitHostApproval(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "verifier-approval")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "verifier-approval", "")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(executable); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, "verifier-approval-writer"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_GATEWAY_VERIFIER_HELPER", "1")
	definition, err := service.AddVerifier(environment.ID, model.VerifierDefinition{
		Name:       "approval-gated",
		Kind:       verifier.KindTest,
		Enabled:    true,
		Executable: executable,
		Args:       []string{"-test.run=^TestGatewayVerifierHelperProcess$"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	admin := connectInMemory(t, ctx, NewAdmin(service))
	defer admin.Close()

	agentTools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(toolNames(agentTools.Tools), "environment_verifier_agent_host_execution_set") {
		t.Fatal("Agent surface must keep verifier host-execution approval mutation discoverable")
	}
	agentApprovalDenied := callGatewayTool(t, ctx, agent, "environment_verifier_agent_host_execution_set", map[string]any{
		"environment_id": environment.ID,
		"verifier_id":    definition.ID,
		"allowed":        true,
	})
	if !agentApprovalDenied.IsError || !strings.Contains(toolText(t, agentApprovalDenied), "requires the Admin surface") {
		t.Fatalf("Agent approval mutation denial must explain Admin requirement: %s", toolText(t, agentApprovalDenied))
	}
	adminTools, err := admin.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(toolNames(adminTools.Tools), "environment_verifier_agent_host_execution_set") {
		t.Fatal("Admin surface lost verifier host-execution approval mutation")
	}

	mode, routeErr := agentVerifierExecutionMode(service, serverSurfaceAgent, environment.ID, definition.ID)
	if service.SandboxVerifierAvailable() {
		if routeErr != nil || mode != "sandbox" {
			t.Fatalf("unapproved Agent verifier route = %q err=%v, want sandbox", mode, routeErr)
		}
	} else {
		if routeErr == nil || !strings.Contains(routeErr.Error(), "trusted host execution is not approved") {
			t.Fatalf("unapproved Agent verifier was not fail-closed: mode=%q err=%v", mode, routeErr)
		}
	}

	if _, err := service.SetStrongPermissionControl(false); err != nil {
		t.Fatal(err)
	}
	mode, routeErr = agentVerifierExecutionMode(service, serverSurfaceAgent, environment.ID, definition.ID)
	if routeErr != nil || mode != "host" {
		t.Fatalf("v1.4.4-compatible mode must restore unapproved host verifier route: mode=%q err=%v", mode, routeErr)
	}
	if _, err := service.SetStrongPermissionControl(true); err != nil {
		t.Fatal(err)
	}

	allowed := callGatewayTool(t, ctx, admin, "environment_verifier_agent_host_execution_set", map[string]any{
		"environment_id": environment.ID,
		"verifier_id":    definition.ID,
		"allowed":        true,
	})
	if allowed.IsError || !strings.Contains(toolText(t, allowed), "\"agent_host_execution_allowed\":true") {
		t.Fatalf("Admin could not approve verifier: %s", toolText(t, allowed))
	}
	mode, routeErr = agentVerifierExecutionMode(service, serverSurfaceAgent, environment.ID, definition.ID)
	if routeErr != nil || mode != "host" {
		t.Fatalf("approved Agent verifier route = %q err=%v, want host", mode, routeErr)
	}
	passed := callGatewayTool(t, ctx, agent, "environment_verifier_run", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   "verifier-approval-writer",
		"verifier_id":    definition.ID,
	})
	if passed.IsError || !strings.Contains(toolText(t, passed), "GATEWAY_VERIFIER_OK") {
		t.Fatalf("approved verifier did not run on trusted host path: %s", toolText(t, passed))
	}

	revoked := callGatewayTool(t, ctx, admin, "environment_verifier_agent_host_execution_set", map[string]any{
		"environment_id": environment.ID,
		"verifier_id":    definition.ID,
		"allowed":        false,
	})
	if revoked.IsError {
		t.Fatalf("Admin could not revoke verifier approval: %s", toolText(t, revoked))
	}
	mode, routeErr = agentVerifierExecutionMode(service, serverSurfaceAgent, environment.ID, definition.ID)
	if service.SandboxVerifierAvailable() {
		if routeErr != nil || mode != "sandbox" {
			t.Fatalf("revoked Agent verifier route = %q err=%v, want sandbox", mode, routeErr)
		}
	} else if routeErr == nil {
		t.Fatalf("revoked verifier remained host-runnable: mode=%q", mode)
	}
}

func TestAgentControlledGitCommitWithoutRawExec(t *testing.T) {
	root := gatewayTestGitRepo(t)
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "controlled-commit")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "controlled-commit", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(environment.ID, "controlled-commit-writer"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("controlled\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	tools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(tools.Tools)
	if !contains(names, "git_commit") {
		t.Fatalf("Agent surface lost controlled git_commit: %v", names)
	}
	for _, raw := range []string{"exec", "run_start", "process_start"} {
		if !contains(names, raw) {
			t.Fatalf("Agent surface must keep raw host tool %q discoverable: %v", raw, names)
		}
	}
	rawDenied := callGatewayTool(t, ctx, agent, "exec", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   "controlled-commit-writer",
		"executable":     os.Args[0],
	})
	if !rawDenied.IsError || !strings.Contains(toolText(t, rawDenied), "Strong Permission Control is enabled") {
		t.Fatalf("raw host execution denial must explain strong permission mode: %s", toolText(t, rawDenied))
	}

	committed := callGatewayTool(t, ctx, agent, "git_commit", map[string]any{
		"environment_id": environment.ID,
		"writer_owner":   "controlled-commit-writer",
		"message":        "test: controlled agent commit",
	})
	if committed.IsError {
		t.Fatalf("git_commit failed: %s", toolText(t, committed))
	}
	if !strings.Contains(toolText(t, committed), "commit") {
		t.Fatalf("git_commit result missing commit identity: %s", toolText(t, committed))
	}
	if got := gatewayGitRun(t, root, "show", "-s", "--format=%s", "HEAD"); got != "test: controlled agent commit" {
		t.Fatalf("commit subject = %q", got)
	}
}

func TestAgentManagedWorktreeCreationDoesNotFetch(t *testing.T) {
	root := gatewayTestGitRepo(t)
	gatewayGitRun(t, root, "remote", "add", "origin", "http://127.0.0.1:1/must-not-fetch.git")
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "agent-safe-worktree")
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Environments.Create(workspace.ID, "agent-safe-source", "")
	if err != nil {
		t.Fatal(err)
	}
	sourceVerifier, err := service.AddVerifier(source.ID, model.VerifierDefinition{
		Name:                      "trusted-build",
		Kind:                      verifier.KindBuild,
		Enabled:                   true,
		AgentHostExecutionAllowed: true,
		Executable:                "go",
		Args:                      []string{"test", "./..."},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()
	created := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"source_environment_id": source.ID,
		"name":                  "agent-safe-child",
		"owner_id":              "agent-safe-owner",
		"ttl_seconds":           3600,
		"mode":                  model.TemporaryEnvironmentModeManagedWorktree,
		"branch_name":           "test/agent-safe-no-fetch",
	})
	if created.IsError {
		t.Fatalf("Agent-safe worktree unexpectedly fetched remote or failed: %s", toolText(t, created))
	}
	items, err := service.ManagedWorktrees()
	if err != nil || len(items) != 1 {
		t.Fatalf("managed worktrees=%+v err=%v", items, err)
	}
	managed := items[0]
	if managed.BaseRef != "HEAD" {
		t.Fatalf("Agent-safe worktree base_ref=%q want HEAD", managed.BaseRef)
	}
	childVerifiers, err := service.ListVerifiers(managed.EnvironmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(childVerifiers) != 1 || childVerifiers[0].ID != sourceVerifier.ID || !childVerifiers[0].AgentHostExecutionAllowed {
		t.Fatalf("derived worktree lost trusted verifier automation: source=%+v child=%+v", sourceVerifier, childVerifiers)
	}
	if _, err := service.Environments.AcquireWriter(managed.EnvironmentID, "agent-safe-cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Isolation.Destroy(ctx, managed.EnvironmentID, "agent-safe-cleanup", true); err != nil {
		t.Fatal(err)
	}
}

func TestAgentManagedWorktreeRejectsMigrationAndFilterCheckout(t *testing.T) {
	root := gatewayTestGitRepo(t)
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "agent-safe-reject")
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Environments.Create(workspace.ID, "agent-safe-reject-source", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	agent := connectInMemory(t, ctx, New(service))
	defer agent.Close()

	migration := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"source_environment_id":       source.ID,
		"name":                        "agent-safe-migrate",
		"owner_id":                    "agent-safe-owner",
		"ttl_seconds":                 3600,
		"mode":                        model.TemporaryEnvironmentModeManagedWorktree,
		"branch_name":                 "test/agent-safe-migrate",
		"migrate_uncommitted_changes": true,
	})
	if !migration.IsError || !strings.Contains(toolText(t, migration), "does not migrate uncommitted changes") {
		t.Fatalf("Agent-safe worktree migration was not rejected: %s", toolText(t, migration))
	}

	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.txt filter=unsafe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, root, "add", ".gitattributes")
	gatewayGitRun(t, root, "commit", "-m", "add unsafe filter attribute")

	filtered := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"source_environment_id": source.ID,
		"name":                  "agent-safe-filter",
		"branch_name":           "test/agent-safe-filter",
		"owner_id":              "agent-safe-owner",
		"ttl_seconds":           3600,
		"mode":                  model.TemporaryEnvironmentModeManagedWorktree,
	})
	if !filtered.IsError || !strings.Contains(toolText(t, filtered), "may execute host code") {
		t.Fatalf("Agent-safe filter checkout was not rejected: %s", toolText(t, filtered))
	}
}
