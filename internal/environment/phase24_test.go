package environment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
	"ai-dev-manager/internal/workspace"
)

func TestManagerInvokeReadRoutesToValidatedEnvironmentWithoutTouchingActivity(t *testing.T) {
	ctx := context.Background()
	configRoot := t.TempDir()
	repo, gitPath := initEnvironmentGitRepo(t)

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := service.Registry().Add(workspace.Input{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Store().SaveProject(repo, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "standard",
			AllowedExecutables: []string{"git"},
			ToolPaths:          map[string]string{"git": gitPath},
		},
	}); err != nil {
		t.Fatal(err)
	}
	manager := newTestManager(t, configRoot, service)
	createdA, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "read-a"})
	if err != nil {
		t.Fatal(err)
	}
	createdB, err := manager.Create(ctx, CreateRequest{WorkspaceID: ws.ID, Name: "read-b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(createdA.Environment.WorktreePath, "source.txt"), []byte("environment-a needle-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(createdB.Environment.WorktreePath, "source.txt"), []byte("environment-b needle-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Holding a writer does not gate read-only operations. Capture activity after
	// acquire so the routed read itself must leave it unchanged.
	leased, err := manager.AcquireWriter(ctx, createdA.Environment.ID, "owner-a")
	if err != nil {
		t.Fatal(err)
	}
	activityBefore := leased.LastActivityAt

	readA, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpRead, map[string]any{"path": "source.txt"})
	if err != nil {
		t.Fatalf("InvokeRead(A) error = %v", err)
	}
	if !strings.Contains(fmt.Sprint(readA), "environment-a") || strings.Contains(fmt.Sprint(readA), "environment-b") {
		t.Fatalf("InvokeRead(A) routed to wrong Environment: %#v", readA)
	}
	readB, err := manager.InvokeRead(ctx, createdB.Environment.ID, runtimeadapter.OpRead, map[string]any{"path": "source.txt"})
	if err != nil {
		t.Fatalf("InvokeRead(B) error = %v", err)
	}
	if !strings.Contains(fmt.Sprint(readB), "environment-b") || strings.Contains(fmt.Sprint(readB), "environment-a") {
		t.Fatalf("InvokeRead(B) routed to wrong Environment: %#v", readB)
	}

	searchA, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpSearch, map[string]any{"query": "needle-a"})
	if err != nil || !strings.Contains(fmt.Sprint(searchA), "source.txt") {
		t.Fatalf("search A = %#v err=%v", searchA, err)
	}
	branchA, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpGitBranch, nil)
	if err != nil || strings.TrimSpace(fmt.Sprint(branchA)) != createdA.Environment.Branch {
		t.Fatalf("git branch A = %#v err=%v want=%s", branchA, err, createdA.Environment.Branch)
	}
	statusA, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpGitStatus, nil)
	if err != nil || !strings.Contains(fmt.Sprint(statusA), "source.txt") {
		t.Fatalf("git status A = %#v err=%v", statusA, err)
	}
	diffA, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpGitDiff, nil)
	if err != nil || !strings.Contains(fmt.Sprint(diffA), "environment-a") {
		t.Fatalf("git diff A = %#v err=%v", diffA, err)
	}

	persisted, err := NewStore(configRoot).Get(createdA.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.LastActivityAt.Equal(activityBefore) {
		t.Fatalf("read-only routing touched activity: before=%v after=%v", activityBefore, persisted.LastActivityAt)
	}
	if persisted.Writer == nil || persisted.Writer.Owner != "owner-a" {
		t.Fatalf("read-only routing changed writer: %+v", persisted.Writer)
	}

	if _, err := manager.InvokeRead(ctx, createdA.Environment.ID, runtimeadapter.OpWrite, map[string]any{"path": "x", "content": "x"}); !isEnvironmentCode(err, ErrInvalidInput) {
		t.Fatalf("mutation operation through InvokeRead error = %v, want invalid_input", err)
	}

	// Remove the managed worktree behind the registry. The next call must
	// revalidate and fail rather than reading the persisted old path.
	gitEnvRun(t, gitPath, repo, "worktree", "remove", "--force", createdB.Environment.WorktreePath)
	if _, err := manager.InvokeRead(ctx, createdB.Environment.ID, runtimeadapter.OpRead, map[string]any{"path": "source.txt"}); !isEnvironmentCode(err, ErrWorktreeMissing) {
		t.Fatalf("InvokeRead(missing worktree) error = %v, want worktree_missing", err)
	}
}

type phase24Runtime struct {
	id           string
	workspaceID  string
	capabilities []string
	invoke       func(string, map[string]any) (any, error)
}

func (r *phase24Runtime) ID() string             { return r.id }
func (r *phase24Runtime) WorkspaceID() string    { return r.workspaceID }
func (r *phase24Runtime) Capabilities() []string { return append([]string(nil), r.capabilities...) }
func (r *phase24Runtime) Status(context.Context) runtimeadapter.Status {
	return runtimeadapter.Status{ID: r.id, WorkspaceID: r.workspaceID, State: runtimeadapter.StateReady, Capabilities: r.Capabilities()}
}
func (r *phase24Runtime) Invoke(_ context.Context, operation string, input map[string]any) (any, error) {
	if r.invoke == nil {
		return nil, errors.New("unexpected invoke")
	}
	return r.invoke(operation, input)
}

func TestManagerInvokeReadChecksConcreteDerivedCapability(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC)
	env := Environment{
		ID:             "env_cap",
		WorkspaceID:    "ws_cap",
		Name:           "cap",
		Branch:         "adm/cap",
		BaseRef:        "dev",
		BaseCommit:     "abc",
		WorktreeName:   "env-cap",
		WorktreePath:   filepath.Join(root, "managed"),
		State:          StateReady,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
	}
	if err := store.Put(env); err != nil {
		t.Fatal(err)
	}
	base := &phase24Runtime{
		id: "base", workspaceID: env.WorkspaceID, capabilities: []string{"git.worktree"},
		invoke: func(operation string, _ map[string]any) (any, error) {
			if operation != runtimeadapter.OpGitWorktreeGet {
				return nil, fmt.Errorf("unexpected base operation %s", operation)
			}
			return worktreeInfo{Path: env.WorktreePath, Branch: env.Branch}, nil
		},
	}
	derivedBuilds := 0
	manager, err := NewManager(
		store,
		func(id string) (model.Workspace, error) { return model.Workspace{ID: id, Path: root}, nil },
		func(string) (runtimeadapter.Runtime, error) { return base, nil },
		func(workspaceID, runtimeID, path string) (runtimeadapter.Runtime, error) {
			derivedBuilds++
			if workspaceID != env.WorkspaceID || runtimeID != env.ID || !samePath(path, env.WorktreePath) {
				t.Fatalf("derived build args = %q %q %q", workspaceID, runtimeID, path)
			}
			return &phase24Runtime{id: runtimeID, workspaceID: workspaceID, capabilities: []string{"files.tree"}, invoke: func(string, map[string]any) (any, error) {
				return "unexpected", nil
			}}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.InvokeRead(context.Background(), env.ID, runtimeadapter.OpRead, map[string]any{"path": "x"}); !isEnvironmentCode(err, ErrCapabilityMissing) {
		t.Fatalf("missing derived capability error = %v, want capability_missing", err)
	}
	if _, err := manager.InvokeRead(context.Background(), env.ID, runtimeadapter.OpRead, map[string]any{"path": "x"}); !isEnvironmentCode(err, ErrCapabilityMissing) {
		t.Fatalf("second missing capability error = %v", err)
	}
	if derivedBuilds != 2 {
		t.Fatalf("derived runtime build count = %d, want revalidation/build on every call", derivedBuilds)
	}
}
