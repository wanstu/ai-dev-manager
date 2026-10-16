package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestCreateTemporaryEnvironmentAgentCannotExpandWorkspaceAuthority(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "agent-boundary")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "agent-existing-root",
		OwnerID:     "agent-owner",
		TTLSeconds:  60,
	}); err == nil || !strings.Contains(err.Error(), "managed_worktree mode with source_environment_id") {
		t.Fatalf("Agent existing_root authority was not rejected: %v", err)
	}

	if _, err := service.CreateTemporaryEnvironment(context.Background(), "agent", model.TemporaryEnvironmentCreateRequest{
		WorkspaceID: workspace.ID,
		Name:        "agent-workspace-derived",
		OwnerID:     "agent-owner",
		TTLSeconds:  60,
		Mode:        model.TemporaryEnvironmentModeManagedWorktree,
		BranchName:  "test/agent-workspace-derived",
	}); err == nil || !strings.Contains(err.Error(), "source_environment_id") {
		t.Fatalf("Agent workspace-derived managed worktree authority was not rejected: %v", err)
	}
}
