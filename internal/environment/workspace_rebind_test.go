package environment

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/workspace"
)

func TestEnvironmentWorkspaceOptionsRecommendMostSpecificContainingWorkspace(t *testing.T) {
	root := t.TempDir()
	workRoot := filepath.Join(root, "work")
	groupRoot := filepath.Join(workRoot, "wm_group")
	if err := os.MkdirAll(groupRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	stateStore := store.New(filepath.Join(t.TempDir(), "state.json"))
	workspaces := workspace.New(stateStore)
	broad, err := workspaces.Add(root, "projects")
	if err != nil {
		t.Fatal(err)
	}
	specific, err := workspaces.Add(groupRoot, "wm_group")
	if err != nil {
		t.Fatal(err)
	}
	service := New(stateStore, workspaces)
	env, err := service.Create(broad.ID, "wm_main", groupRoot)
	if err != nil {
		t.Fatal(err)
	}

	options, err := service.WorkspaceOptions(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !options.RebindAllowed || options.ManagedWorktree {
		t.Fatalf("ordinary Environment should be rebindable: %+v", options)
	}
	if options.CurrentWorkspaceID != broad.ID || options.RecommendedWorkspaceID != specific.ID {
		t.Fatalf("unexpected workspace recommendation: %+v", options)
	}
	if len(options.Candidates) != 2 || options.Candidates[0].ID != specific.ID || options.Candidates[1].ID != broad.ID {
		t.Fatalf("candidates should be ordered most-specific first: %+v", options.Candidates)
	}
}

func TestEnvironmentSetWorkspacePreservesExplicitSelectionsAndRecalculatesInheritance(t *testing.T) {
	root := t.TempDir()
	groupRoot := filepath.Join(root, "work", "wm_group")
	otherRoot := filepath.Join(root, "other")
	if err := os.MkdirAll(groupRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	stateStore := store.New(filepath.Join(t.TempDir(), "state.json"))
	workspaces := workspace.New(stateStore)
	broad, err := workspaces.Add(root, "projects")
	if err != nil {
		t.Fatal(err)
	}
	specific, err := workspaces.Add(groupRoot, "wm_group")
	if err != nil {
		t.Fatal(err)
	}
	outside, err := workspaces.Add(otherRoot, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspaces.SetMCP(broad.ID, "mcp_broad", true); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaces.SetSkill(broad.ID, "skill_broad", true); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaces.SetMCP(specific.ID, "mcp_specific", true); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaces.SetSkill(specific.ID, "skill_specific", true); err != nil {
		t.Fatal(err)
	}

	service := New(stateStore, workspaces)
	env, err := service.Create(broad.ID, "wm_main", groupRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetMCP(env.ID, "mcp_explicit", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetSkill(env.ID, "skill_explicit", true); err != nil {
		t.Fatal(err)
	}

	rebound, err := service.SetWorkspace(env.ID, specific.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rebound.WorkspaceID != specific.ID {
		t.Fatalf("workspace_id=%s want %s", rebound.WorkspaceID, specific.ID)
	}
	assertStringSet(t, rebound.ExplicitMCPIDs, "mcp_explicit")
	assertStringSet(t, rebound.InheritedMCPIDs, "mcp_specific")
	assertStringSet(t, rebound.EnabledMCPIDs, "mcp_explicit", "mcp_specific")
	assertStringSet(t, rebound.ExplicitSkillIDs, "skill_explicit")
	assertStringSet(t, rebound.InheritedSkillIDs, "skill_specific")
	assertStringSet(t, rebound.EnabledSkillIDs, "skill_explicit", "skill_specific")

	state, err := stateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw := state.Environments[findEnvironment(state.Environments, env.ID)]
	assertStringSet(t, raw.EnabledMCPIDs, "mcp_explicit")
	assertStringSet(t, raw.EnabledSkillIDs, "skill_explicit")

	if _, err := service.SetWorkspace(env.ID, outside.ID); err == nil {
		t.Fatal("workspace outside Environment root must be rejected")
	}
	visible, err := service.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if visible.WorkspaceID != specific.ID {
		t.Fatalf("failed rebind changed workspace: got %s want %s", visible.WorkspaceID, specific.ID)
	}
}

func TestManagedWorktreeEnvironmentWorkspaceBindingCannotBeReboundByPath(t *testing.T) {
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "source")
	managedRoot := filepath.Join(root, "managed")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(managedRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	stateStore := store.New(filepath.Join(t.TempDir(), "state.json"))
	workspaces := workspace.New(stateStore)
	source, err := workspaces.Add(sourceRoot, "source")
	if err != nil {
		t.Fatal(err)
	}
	physical, err := workspaces.Add(managedRoot, "physical")
	if err != nil {
		t.Fatal(err)
	}
	service := New(stateStore, workspaces)
	env, _, err := service.CreateManaged(source.ID, "managed-env", managedRoot, model.ManagedWorktree{
		ID:           "mwt_test",
		Branch:       "adm/test",
		BaseCommit:   "0123456789abcdef",
		GitCommonDir: filepath.Join(sourceRoot, ".git"),
	})
	if err != nil {
		t.Fatal(err)
	}

	options, err := service.WorkspaceOptions(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if options.RebindAllowed || !options.ManagedWorktree || options.RecommendedWorkspaceID != source.ID {
		t.Fatalf("managed Environment must keep logical source Workspace: %+v", options)
	}
	if _, err := service.SetWorkspace(env.ID, physical.ID); err == nil {
		t.Fatal("managed worktree Environment must not rebind to physical storage Workspace")
	}
}

func TestEnvironmentWorkspaceRecommendationsOnlyReturnActionableDrift(t *testing.T) {
	root := t.TempDir()
	groupRoot := filepath.Join(root, "work", "wm_group")
	otherRoot := filepath.Join(root, "work", "other")
	if err := os.MkdirAll(groupRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	stateStore := store.New(filepath.Join(t.TempDir(), "state.json"))
	workspaces := workspace.New(stateStore)
	broad, err := workspaces.Add(root, "projects")
	if err != nil {
		t.Fatal(err)
	}
	specific, err := workspaces.Add(groupRoot, "wm_group")
	if err != nil {
		t.Fatal(err)
	}
	other, err := workspaces.Add(otherRoot, "other")
	if err != nil {
		t.Fatal(err)
	}
	service := New(stateStore, workspaces)
	drifted, err := service.Create(broad.ID, "wm_main", groupRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(specific.ID, "already_specific", groupRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(other.ID, "other_env", otherRoot); err != nil {
		t.Fatal(err)
	}

	items, err := service.WorkspaceRecommendations()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("recommendations=%+v want exactly one drifted Environment", items)
	}
	item := items[0]
	if item.EnvironmentID != drifted.ID || item.EnvironmentName != "wm_main" || item.Root != drifted.Root {
		t.Fatalf("unexpected recommendation Environment identity: %+v", item)
	}
	if item.CurrentWorkspace.ID != broad.ID || item.RecommendedWorkspace.ID != specific.ID {
		t.Fatalf("unexpected Workspace recommendation: %+v", item)
	}
}

func assertStringSet(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("values=%v want %v", got, want)
	}
	seen := map[string]bool{}
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("values=%v missing %q", got, value)
		}
	}
}
