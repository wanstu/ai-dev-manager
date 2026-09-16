package gateway

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/pathutil"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayManagedWorktreeLifecycleIsOptionalAndSafe(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	source := gatewayTestGitRepo(t)
	statePath := filepath.Join(t.TempDir(), "adm", "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(source, "source")
	if err != nil {
		t.Fatal(err)
	}
	beforeBranch := gatewayGitRun(t, source, "branch", "--show-current")
	beforeHead := gatewayGitRun(t, source, "rev-parse", "HEAD")

	ctx := context.Background()
	httpServer := httptest.NewServer(NewHTTPHandler(service))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-worktree-acceptance", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect managed worktree HTTP Gateway: %v", err)
	}
	defer session.Close()
	adminClient := mcp.NewClient(&mcp.Implementation{Name: "adm-v2-worktree-admin-acceptance", Version: "dev"}, nil)
	adminSession, err := adminClient.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/admin/mcp"}, nil)
	if err != nil {
		t.Fatalf("connect managed worktree Admin MCP: %v", err)
	}
	defer adminSession.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(tools.Tools)
	for _, required := range []string{"environment_worktree_create", "environment_worktree_list", "environment_worktree_destroy"} {
		if !contains(names, required) {
			t.Fatalf("missing managed worktree tool %q in %v", required, names)
		}
	}

	createdResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_worktree_create",
		Arguments: map[string]any{"workspace_id": ws.ID, "name": "managed"},
	})
	if err != nil || createdResult.IsError {
		t.Fatalf("environment_worktree_create failed: err=%v result=%+v", err, createdResult)
	}
	managedItems, err := service.ManagedWorktrees()
	if err != nil || len(managedItems) != 1 {
		t.Fatalf("managed state after create: items=%+v err=%v", managedItems, err)
	}
	managed := managedItems[0]
	defer func() {
		_, _ = gatewayGitCommand(source, "worktree", "remove", "--force", managed.Root)
		_, _ = gatewayGitCommand(source, "branch", "-D", managed.Branch)
	}()
	if !strings.Contains(toolText(t, createdResult), managed.ID) || !strings.Contains(toolText(t, createdResult), managed.EnvironmentID) {
		t.Fatalf("create result missing managed identities: %s", toolText(t, createdResult))
	}
	if !pathutil.Within(filepath.Join(filepath.Dir(statePath), "worktrees"), managed.Root) {
		t.Fatalf("managed root %s is not under ADM-owned state root", managed.Root)
	}
	listed, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "environment_worktree_list", Arguments: map[string]any{}})
	if err != nil || listed.IsError || !strings.Contains(toolText(t, listed), managed.ID) || !strings.Contains(toolText(t, listed), managed.Branch) {
		t.Fatalf("environment_worktree_list failed: err=%v result=%+v", err, listed)
	}
	blockedRemove, err := adminSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_remove",
		Arguments: map[string]any{"environment_id": managed.EnvironmentID},
	})
	if err != nil {
		t.Fatalf("environment_remove transport error: %v", err)
	}
	if !blockedRemove.IsError {
		t.Fatalf("generic environment_remove must refuse managed worktree: %+v", blockedRemove)
	}
	acquired, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_writer_acquire",
		Arguments: map[string]any{"environment_id": managed.EnvironmentID, "owner": "gateway-writer"},
	})
	if err != nil || acquired.IsError {
		t.Fatalf("managed writer acquire failed: err=%v result=%+v", err, acquired)
	}
	destroyed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "environment_worktree_destroy",
		Arguments: map[string]any{
			"environment_id": managed.EnvironmentID,
			"writer_owner":   "gateway-writer",
		},
	})
	if err != nil || destroyed.IsError {
		t.Fatalf("clean environment_worktree_destroy failed: err=%v result=%+v", err, destroyed)
	}
	if !strings.Contains(toolText(t, destroyed), managed.Branch) {
		t.Fatalf("destroy result must report retained branch: %s", toolText(t, destroyed))
	}
	if _, err := os.Stat(managed.Root); !os.IsNotExist(err) {
		t.Fatalf("managed root still exists after destroy: %v", err)
	}
	if got := gatewayGitRun(t, source, "show-ref", "--verify", "--hash", "refs/heads/"+managed.Branch); got == "" {
		t.Fatal("destroy deleted managed branch")
	}
	if got := gatewayGitRun(t, source, "branch", "--show-current"); got != beforeBranch {
		t.Fatalf("source branch changed: got %q want %q", got, beforeBranch)
	}
	if got := gatewayGitRun(t, source, "rev-parse", "HEAD"); got != beforeHead {
		t.Fatalf("source HEAD changed: got %q want %q", got, beforeHead)
	}

	sourceEnvironment, err := service.Environments.Create(ws.ID, "source-environment", "")
	if err != nil {
		t.Fatal(err)
	}
	fromEnvironment, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "environment_worktree_create",
		Arguments: map[string]any{
			"source_environment_id": sourceEnvironment.ID,
			"name":                  "managed-from-environment",
			"base_ref":              "HEAD",
		},
	})
	if err != nil || fromEnvironment.IsError {
		t.Fatalf("environment_worktree_create from Environment failed: err=%v result=%+v", err, fromEnvironment)
	}
	managedItems, err = service.ManagedWorktrees()
	if err != nil || len(managedItems) != 1 {
		t.Fatalf("managed state after Environment-source create: items=%+v err=%v", managedItems, err)
	}
	fromEnvironmentManaged := managedItems[0]
	if fromEnvironmentManaged.SourceEnvironmentID != sourceEnvironment.ID || !pathutil.Same(fromEnvironmentManaged.SourceRoot, sourceEnvironment.Root) {
		t.Fatalf("Environment source lineage missing: %+v", fromEnvironmentManaged)
	}
	if _, err := service.Environments.AcquireWriter(fromEnvironmentManaged.EnvironmentID, "environment-source-cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Isolation.Destroy(ctx, fromEnvironmentManaged.EnvironmentID, "environment-source-cleanup", true); err != nil {
		t.Fatal(err)
	}

	plainRoot := t.TempDir()
	plainWS, err := service.Workspaces.Add(plainRoot, "plain")
	if err != nil {
		t.Fatal(err)
	}
	unsupported, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_worktree_create",
		Arguments: map[string]any{"workspace_id": plainWS.ID, "name": "unsupported"},
	})
	if err != nil {
		t.Fatalf("non-Git worktree create transport error: %v", err)
	}
	if !unsupported.IsError {
		t.Fatalf("non-Git managed create must fail locally: %+v", unsupported)
	}
	ordinary, err := adminSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "environment_create",
		Arguments: map[string]any{"workspace_id": plainWS.ID, "name": "ordinary"},
	})
	if err != nil || ordinary.IsError {
		t.Fatalf("worktree failure must not break ordinary Environment create: err=%v result=%+v", err, ordinary)
	}
}

func gatewayTestGitRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, root, "init", "-b", "main")
	gatewayGitRun(t, root, "config", "user.email", "adm-test@example.invalid")
	gatewayGitRun(t, root, "config", "user.name", "ADM Test")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, root, "add", "tracked.txt")
	gatewayGitRun(t, root, "commit", "-m", "base")
	return root
}

func gatewayGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gatewayGitCommand(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func gatewayGitCommand(dir string, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
