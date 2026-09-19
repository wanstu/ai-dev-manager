package app

import (
	"ai-dev-manager-v2/internal/isolation"
	"ai-dev-manager-v2/internal/model"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDiscoveryAndEnvironmentDigestAuthority(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"p1", "p2"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name, "go.mod"), []byte("secret-content-fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := New(statePath)
	ws, err := s.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(statePath)
	report, err := s.DiscoverWorkspace(context.Background(), ws.ID, model.DiscoveryRequest{})
	if err != nil || len(report.Candidates) != 2 || report.Scope.WorkspaceID != ws.ID {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Fatal("discovery changed desired state")
	}
	envs, _ := s.Environments.List()
	if len(envs) != 0 {
		t.Fatal("discovery created Environment")
	}
	env, err := s.Environments.Create(ws.ID, "p1", filepath.Join(root, "p1"))
	if err != nil {
		t.Fatal(err)
	}
	before, _ = os.ReadFile(statePath)
	digest, err := s.EnvironmentTreeDigest(context.Background(), env.ID, model.DiscoveryRequest{})
	if err != nil || len(digest.Candidates) != 0 || digest.Scope.EnvironmentID != env.ID {
		t.Fatalf("digest=%+v err=%v", digest, err)
	}
	serialized, _ := json.Marshal(digest)
	if strings.Contains(string(serialized), "p2") || strings.Contains(string(serialized), "secret-content-fixture") {
		t.Fatal("digest exposed sibling or file contents")
	}
	after, _ = os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Fatal("digest changed state")
	}
	for _, r := range []model.DiscoveryRequest{{Path: "../p2"}, {Query: "p2"}} {
		if _, err := s.EnvironmentTreeDigest(context.Background(), env.ID, r); err == nil {
			t.Fatalf("digest accepted %+v", r)
		}
	}
	if _, err := s.DiscoverWorkspace(context.Background(), "missing", model.DiscoveryRequest{}); err == nil {
		t.Fatal("unknown Workspace accepted")
	}
	if _, err := s.EnvironmentTreeDigest(context.Background(), "missing", model.DiscoveryRequest{}); err == nil {
		t.Fatal("unknown Environment accepted")
	}
	outside := t.TempDir()
	if err := s.Store.Update(func(state *model.State) error {
		for i := range state.Environments {
			if state.Environments[i].ID == env.ID {
				state.Environments[i].Root = outside
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnvironmentTreeDigest(context.Background(), env.ID, model.DiscoveryRequest{}); err == nil {
		t.Fatal("invalid unmanaged root bypassed Runtime")
	}
}

func TestDiscoveryDigestRejectsTamperedManagedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("optional Git executable unavailable")
	}
	source := appTestGitRepo(t)
	s := New(filepath.Join(t.TempDir(), "adm", "state.json"))
	ws, err := s.Workspaces.Add(source, "managed")
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.CreateManagedWorktree(context.Background(), ws.ID, "managed", isolation.CreateOptions{BranchName: "test/discovery", BaseRef: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	root := created.ManagedWorktree.Root
	defer appGitCommand(source, "worktree", "remove", "--force", root)
	if _, err := s.EnvironmentTreeDigest(context.Background(), created.Environment.ID, model.DiscoveryRequest{}); err != nil {
		t.Fatal(err)
	}
	appGitRun(t, root, "checkout", "-b", "tampered-discovery")
	if _, err := s.EnvironmentTreeDigest(context.Background(), created.Environment.ID, model.DiscoveryRequest{}); err == nil {
		t.Fatal("tampered managed root accepted")
	}
}
