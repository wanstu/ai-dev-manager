package environment

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/model"
)

func TestValidatedRuntimesReportsMissingBaseWorktreeCapability(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Date(2026, 9, 5, 7, 0, 0, 0, time.UTC)
	env := Environment{
		ID:             "env_base_cap",
		WorkspaceID:    "ws_base_cap",
		Name:           "base-cap",
		Branch:         "adm/base-cap",
		BaseRef:        "master",
		BaseCommit:     "abc",
		WorktreeName:   "env-base-cap",
		WorktreePath:   filepath.Join(root, "managed"),
		State:          StateReady,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
	}
	if err := store.Put(env); err != nil {
		t.Fatal(err)
	}

	baseInvoked := false
	base := &phase24Runtime{
		id: "base", workspaceID: env.WorkspaceID, capabilities: []string{"files.read"},
		invoke: func(string, map[string]any) (any, error) {
			baseInvoked = true
			return nil, nil
		},
	}
	manager, err := NewManager(
		store,
		func(id string) (model.Workspace, error) { return model.Workspace{ID: id, Path: root}, nil },
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		func(string, string, string) (runtimeadapter.Runtime, error) {
			t.Fatal("derived runtime should not be built without base git.worktree capability")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.InvokeRead(context.Background(), env.ID, runtimeadapter.OpRead, map[string]any{"path": "source.txt"}); !isEnvironmentCode(err, ErrCapabilityMissing) {
		t.Fatalf("missing base worktree capability error = %v, want capability_missing", err)
	}
	if baseInvoked {
		t.Fatal("base runtime worktree operation was invoked despite missing capability")
	}
}
