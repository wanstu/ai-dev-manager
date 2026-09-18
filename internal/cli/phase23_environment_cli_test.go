package cli

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

func TestPhase23CLIEnvironmentContextUsesCanonicalPassiveAdminBundle(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	workspace, err := service.Workspaces.Add(workspaceRoot, "phase23-context")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "phase23-context", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.GlobalWrite("global-secret", "phase23-global-secret"); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(environment.ID, "private-secret", "phase23-private-secret"); err != nil {
		t.Fatal(err)
	}
	writerBefore, err := service.Environments.AcquireWriter(environment.ID, "existing-writer")
	if err != nil {
		t.Fatal(err)
	}

	var upstreamRequests atomic.Int64
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		http.Error(w, "context must not connect", http.StatusInternalServerError)
	}))
	defer remote.Close()
	entry, err := service.MCPs.AddMCP("passive-only", remote.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, entry.ID, true); err != nil {
		t.Fatal(err)
	}

	baseURL := startOwnedCLIAdminGateway(t, service)
	requestsBeforeContext := upstreamRequests.Load()
	output := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "context", "--environment-id", environment.ID, "--max-depth", "2", "--max-entries", "100", "--max-digest-entries", "20", "--max-output-bytes", "65536"}); err != nil {
			t.Fatal(err)
		}
	})
	var bundle model.EnvironmentContextBundle
	if err := json.Unmarshal([]byte(output), &bundle); err != nil {
		t.Fatalf("context output is not canonical JSON: %v\n%s", err, output)
	}
	if bundle.Environment.EnvironmentID != environment.ID || bundle.Environment.WorkspaceID != workspace.ID {
		t.Fatalf("context identity=%+v", bundle.Environment)
	}
	for _, forbidden := range []string{"phase23-global-secret", "phase23-private-secret", "existing-writer"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("context leaked %q: %s", forbidden, output)
		}
	}
	if upstreamRequests.Load() != requestsBeforeContext {
		t.Fatalf("context added upstream MCP requests: before=%d after=%d", requestsBeforeContext, upstreamRequests.Load())
	}
	after, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Writer == nil || writerBefore.Writer == nil || after.Writer.LastSeenAt != writerBefore.Writer.LastSeenAt || !after.Writer.ExpiresAt.Equal(writerBefore.Writer.ExpiresAt) {
		t.Fatalf("context changed writer lease: before=%+v after=%+v", writerBefore.Writer, after.Writer)
	}
	if err := run([]string{"--adm-url", baseURL, "environment", "context", "--environment-id", environment.ID, "--path", ".."}); err == nil {
		t.Fatal("context traversal unexpectedly succeeded")
	}
	if err := run([]string{"--adm-url", baseURL, "environment", "context", "--environment-id", "env_missing"}); err == nil {
		t.Fatal("unknown Environment context unexpectedly succeeded")
	}
}

func TestPhase23CLITemporaryEnvironmentLifecycleUsesAdminMCP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspaceRoot := t.TempDir()
	marker := filepath.Join(workspaceRoot, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(workspaceRoot, "phase23-temporary")
	if err != nil {
		t.Fatal(err)
	}
	baseURL := startOwnedCLIAdminGateway(t, service)

	createOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "create", "--workspace-id", workspace.ID, "--name", "promote-me", "--owner-id", "phase23-owner", "--ttl-seconds", "60", "--session-id", "session-proof", "--run-id", "run-proof"}); err != nil {
			t.Fatal(err)
		}
	})
	var created model.TemporaryEnvironmentCreateResult
	if err := json.Unmarshal([]byte(createOutput), &created); err != nil || created.Environment.ID == "" {
		t.Fatalf("temporary create output=%s err=%v", createOutput, err)
	}
	if created.Environment.Retention.Persistence != model.PersistenceTemporary || created.Environment.Retention.OwnerID != "phase23-owner" || created.Environment.Retention.SessionID != "session-proof" || created.Environment.Retention.RunID != "run-proof" {
		t.Fatalf("temporary retention=%+v", created.Environment.Retention)
	}
	statusOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "status", "--environment-id", created.Environment.ID}); err != nil {
			t.Fatal(err)
		}
	})
	var status model.TemporaryEnvironmentStatus
	if err := json.Unmarshal([]byte(statusOutput), &status); err != nil || status.EnvironmentID != created.Environment.ID {
		t.Fatalf("temporary status=%s err=%v", statusOutput, err)
	}
	if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "promote", "--environment-id", created.Environment.ID, "--owner-id", "wrong-owner"}); err == nil {
		t.Fatal("wrong-owner promote unexpectedly succeeded")
	}
	promoteOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "promote", "--environment-id", created.Environment.ID, "--owner-id", "phase23-owner"}); err != nil {
			t.Fatal(err)
		}
	})
	var promoted model.TemporaryEnvironmentStatus
	if err := json.Unmarshal([]byte(promoteOutput), &promoted); err != nil || promoted.EnvironmentID != created.Environment.ID || promoted.Retention.Persistence != model.PersistenceDurable {
		t.Fatalf("promoted status=%s err=%v", promoteOutput, err)
	}

	cleanupCreateOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "create", "--workspace-id", workspace.ID, "--name", "cleanup-me", "--owner-id", "cleanup-owner", "--ttl-seconds", "1"}); err != nil {
			t.Fatal(err)
		}
	})
	var cleanupCreated model.TemporaryEnvironmentCreateResult
	if err := json.Unmarshal([]byte(cleanupCreateOutput), &cleanupCreated); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := service.Environments.AcquireWriter(cleanupCreated.Environment.ID, "blocking-writer"); err != nil {
		t.Fatal(err)
	}
	blockedPreviewOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "cleanup", "--environment-id", cleanupCreated.Environment.ID, "--owner-id", "cleanup-owner"}); err != nil {
			t.Fatal(err)
		}
	})
	var blockedPreview model.ResourceRetentionCleanupResult
	if err := json.Unmarshal([]byte(blockedPreviewOutput), &blockedPreview); err != nil || !blockedPreview.DryRun || !strings.Contains(blockedPreviewOutput, "active_writer") || len(blockedPreview.WouldRemove) != 0 {
		t.Fatalf("blocked cleanup preview=%s err=%v", blockedPreviewOutput, err)
	}
	if _, err := service.Environments.ReleaseWriter(cleanupCreated.Environment.ID, "blocking-writer", false); err != nil {
		t.Fatal(err)
	}
	previewOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "cleanup", "--environment-id", cleanupCreated.Environment.ID, "--owner-id", "cleanup-owner"}); err != nil {
			t.Fatal(err)
		}
	})
	var preview model.ResourceRetentionCleanupResult
	if err := json.Unmarshal([]byte(previewOutput), &preview); err != nil || !preview.DryRun || len(preview.WouldRemove) != 1 || preview.WouldRemove[0].ID != cleanupCreated.Environment.ID {
		t.Fatalf("cleanup preview=%s err=%v", previewOutput, err)
	}
	if _, err := service.Environments.Get(cleanupCreated.Environment.ID); err != nil {
		t.Fatalf("preview mutated Environment: %v", err)
	}
	if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "cleanup", "--environment-id", cleanupCreated.Environment.ID, "--owner-id", "wrong-owner", "--execute"}); err == nil {
		t.Fatal("wrong-owner cleanup unexpectedly succeeded")
	}
	executeOutput := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "cleanup", "--environment-id", cleanupCreated.Environment.ID, "--owner-id", "cleanup-owner", "--execute"}); err != nil {
			t.Fatal(err)
		}
	})
	var executed model.ResourceRetentionCleanupResult
	if err := json.Unmarshal([]byte(executeOutput), &executed); err != nil || executed.DryRun || len(executed.Removed) != 1 || executed.Removed[0].ID != cleanupCreated.Environment.ID {
		t.Fatalf("cleanup execute=%s err=%v", executeOutput, err)
	}
	if _, err := service.Environments.Get(cleanupCreated.Environment.ID); err == nil {
		t.Fatal("cleanup did not remove selected Environment")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("ordinary cleanup touched project files: data=%q err=%v", data, err)
	}
	if _, err := service.Environments.Get(created.Environment.ID); err != nil {
		t.Fatalf("targeted cleanup touched unrelated promoted Environment: %v", err)
	}

	if err := run([]string{"--adm-url", baseURL, "environment", "temporary", "create", "--workspace-id", workspace.ID, "--name", "managed-needs-git", "--owner-id", "phase23-owner", "--ttl-seconds", "60", "--mode", "managed_worktree"}); err == nil {
		t.Fatal("managed_worktree unexpectedly succeeded for non-Git Workspace")
	}
}

func TestPhase23CLIEnvironmentAgentCommandsUseNormalNoFallbackAdminPath(t *testing.T) {
	localHome := t.TempDir()
	t.Setenv("ADM_V2_HOME", localHome)
	local := app.New(filepath.Join(localHome, "state.json"))
	workspace, err := local.Workspaces.Add(t.TempDir(), "local-only")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := netListenClosed()
	if err != nil {
		t.Fatal(err)
	}
	baseURL := listener
	for _, args := range [][]string{
		{"--adm-url", baseURL, "environment", "context", "--environment-id", "env_missing"},
		{"--adm-url", baseURL, "environment", "temporary", "status", "--environment-id", "env_missing"},
		{"--adm-url", baseURL, "environment", "temporary", "create", "--workspace-id", workspace.ID, "--name", "must-not-create", "--owner-id", "owner", "--ttl-seconds", "60"},
	} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "Admin MCP") {
			t.Fatalf("%v should fail through Admin MCP without fallback, got %v", args, err)
		}
	}
	items, err := local.Environments.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("failed remote commands touched local Environment state: items=%+v err=%v", items, err)
	}
}

func TestPhase23CLIEnvironmentAgentHelpKeepsExplicitSafetyBoundaries(t *testing.T) {
	environmentHelp := captureStdout(t, func() {
		if err := runEnvironment(nil, []string{"-h"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, required := range []string{"environment injection-plan --environment-id", "environment context --environment-id", "environment temporary -h", "Admin MCP", "owner-local"} {
		if !strings.Contains(environmentHelp, required) {
			t.Fatalf("environment help missing %q:\n%s", required, environmentHelp)
		}
	}
	temporaryHelp := captureStdout(t, func() {
		if err := runEnvironmentTemporary(nil, []string{"-h"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, required := range []string{"--owner-id OWNER --ttl-seconds N", "默认 preview", "没有 force 路径", "不删除项目目录", "保留生成分支"} {
		if !strings.Contains(temporaryHelp, required) {
			t.Fatalf("temporary help missing %q:\n%s", required, temporaryHelp)
		}
	}
	if strings.Contains(temporaryHelp, "--force") {
		t.Fatalf("temporary help exposes force cleanup:\n%s", temporaryHelp)
	}
}

func netListenClosed() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", err
	}
	return "http://" + address, nil
}
