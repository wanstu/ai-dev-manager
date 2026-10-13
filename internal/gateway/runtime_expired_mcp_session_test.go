package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A healthy, cached upstream MCP client is not evidence that an expired
// temporary Environment still has active work.
func TestExpiredEnvironmentCachedMCPSessionDoesNotBlockAndStopsReconnecting(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	expiredAt := time.Now().UTC().Add(-time.Minute)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind: model.RetentionResourceEnvironment, ID: environmentID,
		OwnerID: "expired-session-test", ExpiresAt: &expiredAt,
	}); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	connects := 0
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		connects++
		return &fakeOwnedMCPSession{}, nil
	}
	owner.Reconcile(context.Background())
	if connects != 0 {
		t.Fatalf("reconcile reopened MCP sessions for expired Environment: %d", connects)
	}

	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	// Protect our deterministic fixture while the automatic monitor runs.
	endOperation := owner.trackMCPOperation(key)
	cached := &fakeOwnedMCPSession{}
	owner.mu.Lock()
	owner.sessions[key] = cached
	owner.mu.Unlock()

	status, err := owner.TemporaryEnvironmentStatus(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if hasRetentionBlocker(status.Blockers, "active_mcp_session") {
		t.Fatalf("cached MCP session misclassified as active work: %+v", status.Blockers)
	}
	if !hasRetentionBlocker(status.Blockers, "mcp_operation_in_flight") {
		t.Fatalf("real MCP operation must block cleanup: %+v", status.Blockers)
	}
	owner.monitorOnce(time.Now().UTC())
	if _, _, closes := cached.counts(); closes != 0 {
		t.Fatalf("monitor closed MCP session while work was active, closes=%d", closes)
	}
	endOperation()
	status, err = owner.TemporaryEnvironmentStatus(context.Background(), environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if hasRetentionBlocker(status.Blockers, "active_mcp_session") ||
		hasRetentionBlocker(status.Blockers, "mcp_operation_in_flight") {
		t.Fatalf("idle cached session still blocks expired environment: %+v", status.Blockers)
	}
	owner.monitorOnce(time.Now().UTC())
	if _, _, closes := cached.counts(); closes != 1 {
		t.Fatalf("expired idle session was not closed, closes=%d", closes)
	}
	owner.mu.Lock()
	remaining := owner.sessions[key]
	owner.mu.Unlock()
	if remaining != nil {
		t.Fatal("monitor retained expired idle MCP session")
	}
	if connects != 0 {
		t.Fatalf("monitor reconnected expired environment: %d", connects)
	}
	cleaned, err := owner.TemporaryEnvironmentCleanup(context.Background(), environmentID, "expired-session-test", true)
	if err != nil || len(cleaned.Removed) != 1 {
		t.Fatalf("eligible expired Environment should clean up after idle session closes: %+v err=%v", cleaned, err)
	}
}

type waitingOwnedMCPSession struct {
	*fakeOwnedMCPSession
	entered chan struct{}
	release chan struct{}
}

func (s *waitingOwnedMCPSession) CallTool(ctx context.Context, _ *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	close(s.entered)
	select {
	case <-s.release:
		return &mcp.CallToolResult{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestExpiredEnvironmentRealToolCallBlocksSessionEviction(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	expired := time.Now().UTC().Add(-time.Hour)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind: model.RetentionResourceEnvironment, ID: environmentID,
		OwnerID: "active-tool-owner", ExpiresAt: &expired,
	}); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	hold := owner.trackMCPOperation(key)
	fake := &waitingOwnedMCPSession{
		fakeOwnedMCPSession: &fakeOwnedMCPSession{},
		entered:             make(chan struct{}),
		release:             make(chan struct{}),
	}
	owner.mu.Lock()
	owner.sessions[key] = fake
	owner.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := owner.CallTool(context.Background(), environmentID, mcpID, "owned_fake", nil)
		done <- err
	}()
	select {
	case <-fake.entered:
	case <-time.After(3 * time.Second):
		close(fake.release)
		t.Fatal("tool call did not start")
	}
	hold()
	if blockers := owner.runtimeRetentionBlockers(model.RetentionResourceEnvironment, environmentID); !hasRetentionBlocker(blockers, "mcp_operation_in_flight") {
		t.Fatalf("in-progress tool call must block cleanup: %+v", blockers)
	}
	owner.monitorOnce(time.Now().UTC())
	if _, _, closes := fake.counts(); closes != 0 {
		t.Fatalf("session was evicted while tool call was running: %d", closes)
	}
	close(fake.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tool call did not finish")
	}
	owner.monitorOnce(time.Now().UTC())
	if _, _, closes := fake.counts(); closes != 1 {
		t.Fatalf("idle expired connection not closed after operation: %d", closes)
	}
}

func TestExpiredManagedWorktreeCleanupReleasesCachedMCPAndKeepsBranch(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	initRetentionGitWorkspace(t, root)
	ws, err := service.Workspaces.Add(root, "expired-managed")
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Isolation.Create(context.Background(), ws.ID, "expired-managed", isolation.CreateOptions{
		BranchName: "test/expired-session-managed", BaseRef: "HEAD",
	})
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Minute)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind: model.RetentionResourceEnvironment, ID: created.Environment.ID,
		OwnerID: "expired-managed-owner", ExpiresAt: &expired,
	}); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	key := runtimeOwnerKey{environmentID: created.Environment.ID, mcpID: "cached-test-id"}
	cached := &fakeOwnedMCPSession{}
	owner.mu.Lock()
	owner.sessions[key] = cached
	owner.mu.Unlock()
	result, err := owner.TemporaryEnvironmentCleanup(context.Background(), created.Environment.ID, "expired-managed-owner", true)
	if err != nil || !hasTemporaryRetentionMutation(result.Removed, model.RetentionResourceEnvironment, created.Environment.ID) {
		t.Fatalf("expired clean managed worktree was not safely removed: %+v err=%v", result, err)
	}
	if _, err := os.Stat(created.ManagedWorktree.Root); !os.IsNotExist(err) {
		t.Fatalf("managed worktree directory still exists after cleanup: %v", err)
	}
	gatewayGitRun(t, root, "show-ref", "--verify", "refs/heads/"+created.ManagedWorktree.Branch)
	if _, _, closes := cached.counts(); closes != 1 {
		t.Fatalf("cached upstream connection was not closed on removal: %d", closes)
	}
}

func TestNonExpiredEnvironmentSessionStillBlocksCleanup(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	future := time.Now().UTC().Add(time.Hour)
	if _, err := service.MarkResourceTemporary(model.ResourceRetentionUpdateRequest{
		Kind: model.RetentionResourceEnvironment, ID: environmentID,
		OwnerID: "future-session-test", ExpiresAt: &future,
	}); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	key := runtimeOwnerKey{environmentID: environmentID, mcpID: mcpID}
	session := &fakeOwnedMCPSession{}
	owner.mu.Lock()
	owner.sessions[key] = session
	owner.mu.Unlock()
	if blockers := owner.runtimeRetentionBlockers(model.RetentionResourceEnvironment, environmentID); !hasRetentionBlocker(blockers, "active_mcp_session") {
		t.Fatalf("non-expired Environment session must retain original blocker: %s", strings.Join(blockers, ", "))
	}
}
