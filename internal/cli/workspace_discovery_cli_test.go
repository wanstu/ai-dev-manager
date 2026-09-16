package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestCLIWorkspaceDiscoverAndEnvironmentTreeDigestUseAdminMCP(t *testing.T) {
	home := t.TempDir()
	service := startCLIAdminTestServer(t, home)
	root := t.TempDir()
	projectRoot := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte("sentinel-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "cli-discovery")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "web", projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	workspaceOutput := captureStdout(t, func() {
		if err := run([]string{"workspace", "discover", "--workspace-id", workspace.ID, "--path", "apps", "--query", "web", "--max-depth", "3", "--max-entries", "100", "--max-candidates", "5", "--max-digest-entries", "20", "--max-output-bytes", "8192"}); err != nil {
			t.Fatal(err)
		}
	})
	var workspaceReport model.DiscoveryReport
	if err := json.Unmarshal([]byte(workspaceOutput), &workspaceReport); err != nil {
		t.Fatalf("decode workspace discovery JSON: %v\n%s", err, workspaceOutput)
	}
	if workspaceReport.Scope.WorkspaceID != workspace.ID || workspaceReport.ScanPath != "apps" || workspaceReport.Query != "web" {
		t.Fatalf("workspace discovery scope/request mismatch: %+v", workspaceReport)
	}
	if len(workspaceReport.Candidates) != 1 || workspaceReport.Candidates[0].Root != filepath.ToSlash(filepath.Join("apps", "web")) {
		t.Fatalf("workspace discovery candidates=%+v", workspaceReport.Candidates)
	}
	if workspaceReport.Limits.MaxDepth != 3 || workspaceReport.Limits.MaxEntries != 100 || workspaceReport.Limits.MaxOutputBytes != 8192 {
		t.Fatalf("workspace discovery limits=%+v", workspaceReport.Limits)
	}

	digestOutput := captureStdout(t, func() {
		if err := run([]string{"environment", "tree-digest", "--environment-id", environment.ID, "--max-depth", "2", "--max-entries", "50", "--max-digest-entries", "10", "--max-output-bytes", "8192"}); err != nil {
			t.Fatal(err)
		}
	})
	var digest model.DiscoveryReport
	if err := json.Unmarshal([]byte(digestOutput), &digest); err != nil {
		t.Fatalf("decode environment digest JSON: %v\n%s", err, digestOutput)
	}
	if digest.Scope.EnvironmentID != environment.ID || digest.Scope.WorkspaceID != workspace.ID {
		t.Fatalf("environment digest scope=%+v", digest.Scope)
	}
	for _, entry := range digest.Digest {
		if strings.Contains(entry.Path, "..") || strings.Contains(strings.ToLower(entry.Path), "sibling") {
			t.Fatalf("environment digest escaped root: %+v", digest.Digest)
		}
	}
}

func TestCLIDiscoveryRejectsUnknownStableIDs(t *testing.T) {
	startCLIAdminTestServer(t, t.TempDir())
	for _, args := range [][]string{
		{"workspace", "discover", "--workspace-id", "ws_missing"},
		{"environment", "tree-digest", "--environment-id", "env_missing"},
	} {
		err := run(args)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Fatalf("unknown discovery target args=%v error=%v", args, err)
		}
	}
}

func TestCLIDiscoveryRejectsOutOfScopePaths(t *testing.T) {
	service := startCLIAdminTestServer(t, t.TempDir())
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "scope")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "project", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"workspace", "discover", "--workspace-id", workspace.ID, "--path", ".."},
		{"environment", "tree-digest", "--environment-id", environment.ID, "--path", ".."},
	} {
		err := run(args)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "path") {
			t.Fatalf("out-of-scope discovery args=%v error=%v", args, err)
		}
	}
}
