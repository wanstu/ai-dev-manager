package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayResourceRetentionCleanupDryRunUsesRuntimeOwnerObservations(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)
	mcpEntry, err := service.MCPs.AddMCPConfigWithRetention("temporary-unused-mcp", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	}, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-runtime",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	dryRun, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "resource_retention_cleanup",
		Arguments: map[string]any{},
	})
	if err != nil || dryRun.IsError {
		t.Fatalf("resource_retention_cleanup dry-run failed: err=%v result=%+v", err, dryRun)
	}
	text := toolText(t, dryRun)
	for _, required := range []string{"\"dry_run\":true", "\"would_remove\"", mcpEntry.ID, model.RetentionResourceMCP} {
		if !strings.Contains(strings.ReplaceAll(text, " ", ""), strings.ReplaceAll(required, " ", "")) {
			t.Fatalf("dry-run cleanup output missing %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, app.RetentionRuntimeObservationBlocker) {
		t.Fatalf("Gateway-owner dry run should resolve runtime observation blocker: %s", text)
	}
}

func TestGatewayResourceRetentionCleanupExecuteRemovesRuntimeSafeMCP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)
	mcpEntry, err := service.MCPs.AddMCPConfigWithRetention("temporary-runtime-safe-mcp", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	}, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-runtime",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	executed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "resource_retention_cleanup",
		Arguments: map[string]any{"execute": true},
	})
	if err != nil || executed.IsError {
		t.Fatalf("resource_retention_cleanup execute failed: err=%v result=%+v text=%s", err, executed, toolText(t, executed))
	}
	text := toolText(t, executed)
	for _, required := range []string{"\"dry_run\":false", "\"removed\"", mcpEntry.ID, model.RetentionResourceMCP} {
		if !strings.Contains(strings.ReplaceAll(text, " ", ""), strings.ReplaceAll(required, " ", "")) {
			t.Fatalf("execute cleanup output missing %q:\n%s", required, text)
		}
	}
	if _, err := service.MCPs.Get(mcpEntry.ID); err == nil {
		t.Fatalf("execute did not remove runtime-safe temporary MCP %s", mcpEntry.ID)
	}
}

func TestGatewayResourceRetentionCleanupExecuteRemovesTemporaryEnvironmentStateOnly(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-environment-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour)
	environment, err := service.Environments.CreateWithRetention(workspace.ID, "temporary-environment", "", model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-environment",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	executed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "resource_retention_cleanup",
		Arguments: map[string]any{"execute": true},
	})
	if err != nil || executed.IsError {
		t.Fatalf("resource_retention_cleanup execute failed: err=%v result=%+v text=%s", err, executed, toolText(t, executed))
	}
	text := toolText(t, executed)
	for _, required := range []string{"\"removed\"", environment.ID, model.RetentionResourceEnvironment} {
		if !strings.Contains(strings.ReplaceAll(text, " ", ""), strings.ReplaceAll(required, " ", "")) {
			t.Fatalf("environment cleanup output missing %q:\n%s", required, text)
		}
	}
	if _, err := service.Environments.Get(environment.ID); err == nil {
		t.Fatalf("execute did not remove temporary Environment state %s", environment.ID)
	}
	if _, err := service.Workspaces.Get(workspace.ID); err != nil {
		t.Fatalf("environment cleanup removed Workspace state: %v", err)
	}
	if info, err := os.Stat(workspaceRoot); err != nil || !info.IsDir() {
		t.Fatalf("environment cleanup touched Workspace/project directory: info=%v err=%v", info, err)
	}
}

func TestGatewayResourceRetentionCleanupExecuteDestroysSafeTemporaryManagedWorktree(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	initRetentionGitWorkspace(t, workspaceRoot)
	workspace, err := service.Workspaces.Add(workspaceRoot, "temporary-managed-worktree-cleanup")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := service.Isolation.Create(ctx, workspace.ID, "temporary-managed-worktree", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:      model.RetentionResourceEnvironment,
		ID:        created.Environment.ID,
		OwnerID:   "owner-managed-worktree",
		ExpiresAt: &expiredAt,
	}); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	result, err := owner.ResourceRetentionCleanup(ctx, model.ResourceRetentionCleanupRequest{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	removed := false
	for _, mutation := range result.Removed {
		if mutation.Kind == model.RetentionResourceEnvironment && mutation.ID == created.Environment.ID {
			removed = true
			break
		}
	}
	if !removed {
		t.Fatalf("managed worktree Environment was not removed: %+v", result)
	}
	if _, err := service.Environments.Get(created.Environment.ID); err == nil {
		t.Fatalf("managed worktree Environment state still exists: %s", created.Environment.ID)
	}
	if _, ok, err := service.Isolation.GetByEnvironment(created.Environment.ID); err != nil || ok {
		t.Fatalf("managed worktree metadata still exists: ok=%v err=%v", ok, err)
	}
	if _, err := os.Stat(created.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("managed worktree root still exists or stat failed unexpectedly: %v", err)
	}
	if out, err := exec.Command("git", "-C", workspaceRoot, "show-ref", "--verify", "refs/heads/"+created.ManagedWorktree.Branch).CombinedOutput(); err != nil {
		t.Fatalf("managed branch should be retained: %v\n%s", err, out)
	}
}

func TestGatewayResourceRetentionCleanupBlocksDirtyTemporaryManagedWorktree(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	initRetentionGitWorkspace(t, workspaceRoot)
	workspace, err := service.Workspaces.Add(workspaceRoot, "dirty-temporary-managed-worktree")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, err := service.Isolation.Create(ctx, workspace.ID, "dirty-temporary-managed-worktree", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind:      model.RetentionResourceEnvironment,
		ID:        created.Environment.ID,
		OwnerID:   "owner-dirty-managed-worktree",
		ExpiresAt: &expiredAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(created.ManagedWorktree.Root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	report, err := owner.ResourceRetentionReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundDirtyBlocker := false
	for _, item := range report.Resources {
		if item.Kind == model.RetentionResourceEnvironment && item.ID == created.Environment.ID {
			foundDirtyBlocker = hasRetentionBlocker(item.Blockers, "managed_worktree_dirty")
			if item.CleanupEligible {
				t.Fatalf("dirty managed worktree should not be cleanup eligible: %+v", item)
			}
		}
	}
	if !foundDirtyBlocker {
		t.Fatalf("dirty managed worktree blocker missing: %+v", report.Resources)
	}
	result, err := owner.ResourceRetentionCleanup(ctx, model.ResourceRetentionCleanupRequest{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range result.Removed {
		if mutation.Kind == model.RetentionResourceEnvironment && mutation.ID == created.Environment.ID {
			t.Fatalf("dirty managed worktree was removed: %+v", result)
		}
	}
	if _, err := service.Environments.Get(created.Environment.ID); err != nil {
		t.Fatalf("dirty managed worktree Environment was removed: %v", err)
	}
	if info, err := os.Stat(created.ManagedWorktree.Root); err != nil || !info.IsDir() {
		t.Fatalf("dirty managed worktree root was removed: info=%v err=%v", info, err)
	}
}

func initRetentionGitWorkspace(t *testing.T, root string) {
	t.Helper()
	commands := [][]string{
		{"init"},
		{"config", "user.email", "retention-test@example.invalid"},
		{"config", "user.name", "Retention Test"},
	}
	for _, args := range commands {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("retention test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func TestGatewayResourceRetentionMarkTemporaryAndPromoteTools(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)
	mcpEntry, err := service.MCPs.AddMCP("retention-tool-mcp", "http://127.0.0.1:65534/mcp", false)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	session := connectInMemory(t, ctx, NewAdmin(service))
	defer session.Close()
	marked, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "resource_retention_mark_temporary",
		Arguments: map[string]any{
			"kind":       model.RetentionResourceMCP,
			"id":         mcpEntry.ID,
			"owner_id":   "owner-tool",
			"expires_at": expiredAt.Format(time.RFC3339Nano),
		},
	})
	if err != nil || marked.IsError {
		t.Fatalf("resource_retention_mark_temporary failed: err=%v result=%+v", err, marked)
	}
	if text := toolText(t, marked); !strings.Contains(text, model.RetentionUpdateActionMarkTemporary) || !strings.Contains(text, "owner-tool") {
		t.Fatalf("mark temporary output missing retention facts: %s", text)
	}
	storedMarked, err := service.MCPs.Get(mcpEntry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedMarked.Retention.Persistence != model.PersistenceTemporary || storedMarked.Retention.OwnerID != "owner-tool" {
		t.Fatalf("mark temporary did not persist: %+v", storedMarked.Retention)
	}

	promoted, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "resource_retention_promote",
		Arguments: map[string]any{"kind": model.RetentionResourceMCP, "id": mcpEntry.ID},
	})
	if err != nil || promoted.IsError {
		t.Fatalf("resource_retention_promote failed: err=%v result=%+v", err, promoted)
	}
	if text := toolText(t, promoted); !strings.Contains(text, model.RetentionUpdateActionPromoteDurable) || !strings.Contains(text, model.PersistenceDurable) {
		t.Fatalf("promote output missing durable facts: %s", text)
	}
	storedPromoted, err := service.MCPs.Get(mcpEntry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedPromoted.Retention.Persistence != model.PersistenceDurable || storedPromoted.Retention.OwnerID != "" || storedPromoted.Retention.ExpiresAt != nil {
		t.Fatalf("promote did not clear temporary fields: %+v", storedPromoted.Retention)
	}
}

func TestGatewayResourceRetentionCleanupExecuteBlocksActiveMCPRuntime(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	expiredAt := time.Now().UTC().Add(-time.Hour)
	mcpEntry, err := service.MCPs.AddMCPConfigWithRetention("temporary-inflight-mcp", catalog.MCPConfig{
		Transport: catalog.MCPTransportStreamableHTTP,
		AuthMode:  catalog.MCPAuthNone,
		Endpoint:  "http://127.0.0.1:65534/mcp",
	}, model.ResourceRetention{
		Persistence:    model.PersistenceTemporary,
		CreatorSurface: "gateway",
		OwnerID:        "owner-runtime",
		ExpiresAt:      &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	owner.mu.Lock()
	owner.inFlight[runtimeOwnerKey{environmentID: "env_active", mcpID: mcpEntry.ID}] = true
	owner.mu.Unlock()

	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	executed, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "resource_retention_cleanup",
		Arguments: map[string]any{"execute": true},
	})
	if err != nil || executed.IsError {
		t.Fatalf("resource_retention_cleanup execute with active runtime failed unexpectedly: err=%v result=%+v", err, executed)
	}
	text := toolText(t, executed)
	if strings.Contains(text, "\"removed\"") && strings.Contains(text, mcpEntry.ID) {
		t.Fatalf("active MCP runtime should not be removed:\n%s", text)
	}
	if !strings.Contains(text, "mcp_operation_in_flight") {
		t.Fatalf("active MCP runtime blocker missing:\n%s", text)
	}
	if _, err := service.MCPs.Get(mcpEntry.ID); err != nil {
		t.Fatalf("active runtime cleanup removed MCP: %v", err)
	}
}
