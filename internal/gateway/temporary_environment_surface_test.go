package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/verifier"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTemporaryEnvironmentSharedHTTPPlainLifecycleAuthorityAndTargeting(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "adm", "state.json")
	service := app.New(statePath)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("phase22-seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Workspaces.Add(root, "phase22-plain")
	if err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	server := httptest.NewServer(newHTTPHandler(service, owner))
	ctx := context.Background()
	agent := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	admin := connectHTTPWithRetry(t, ctx, server.URL+"/admin/mcp")

	for surface, session := range map[string]*mcp.ClientSession{"agent": agent, "admin": admin} {
		tools, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("%s list tools: %v", surface, err)
		}
		byName := map[string]*mcp.Tool{}
		for _, tool := range tools.Tools {
			byName[tool.Name] = tool
		}
		for _, name := range []string{"environment_temporary_create", "environment_temporary_status", "environment_temporary_promote", "environment_temporary_cleanup"} {
			if byName[name] == nil {
				t.Fatalf("%s surface missing %s", surface, name)
			}
		}
		encodedCleanup, err := json.Marshal(byName["environment_temporary_cleanup"])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(encodedCleanup)), `"force"`) {
			t.Fatalf("%s temporary cleanup unexpectedly exposes force: %s", surface, encodedCleanup)
		}
		if !strings.Contains(strings.ToLower(byName["environment_temporary_create"].Description), "provenance") || !strings.Contains(strings.ToLower(byName["environment_temporary_create"].Description), "not adm task orchestration") {
			t.Fatalf("%s temporary create description lost provenance boundary: %q", surface, byName["environment_temporary_create"].Description)
		}
		_, hasDurableCreate := byName["environment_create"]
		if surface == "agent" && hasDurableCreate {
			t.Fatal("Agent surface exposed generic durable environment_create")
		}
		if surface == "admin" && !hasDurableCreate {
			t.Fatal("Admin surface lost generic durable environment_create")
		}
	}

	createdTool := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"workspace_id": workspace.ID,
		"name":         "phase22-http-temp",
		"owner_id":     "lifecycle-owner-a",
		"ttl_seconds":  60,
		"session_id":   "session-provenance-a",
		"run_id":       "run_nonexistent_provenance",
	})
	if createdTool.IsError {
		t.Fatalf("plain temporary create failed: %s", toolText(t, createdTool))
	}
	created := temporaryCreateFromTool(t, createdTool)
	if created.Mode != model.TemporaryEnvironmentModeExistingRoot || created.Environment.Retention.Persistence != model.PersistenceTemporary || created.Environment.Retention.OwnerID != "lifecycle-owner-a" || created.Environment.Retention.SessionID != "session-provenance-a" || created.Environment.Retention.RunID != "run_nonexistent_provenance" || created.Environment.Retention.CreatorSurface != string(serverSurfaceAgent) || created.Environment.Retention.ExpiresAt == nil {
		t.Fatalf("plain temporary create lost retention/provenance: %+v", created)
	}
	if created.ManagedWorktree != nil || !pathutil.Same(created.Environment.Root, root) {
		t.Fatalf("plain temporary create changed root/isolation: %+v", created)
	}
	if runs, err := owner.ListAgentRuns(created.Environment.ID); err != nil || len(runs) != 0 {
		t.Fatalf("run provenance unexpectedly required/started a Run: runs=%+v err=%v", runs, err)
	}
	if err := service.Memory.EnvironmentWrite(created.Environment.ID, "private", "phase22-private-sentinel"); err != nil {
		t.Fatal(err)
	}

	status := callGatewayTool(t, ctx, admin, "environment_temporary_status", map[string]any{"environment_id": created.Environment.ID})
	if status.IsError {
		t.Fatalf("temporary status failed: %s", toolText(t, status))
	}
	statusText := toolText(t, status)
	for _, wanted := range []string{"lifecycle-owner-a", "session-provenance-a", "run_nonexistent_provenance"} {
		if !strings.Contains(statusText, wanted) {
			t.Fatalf("status missing provenance %q: %s", wanted, statusText)
		}
	}
	if strings.Contains(statusText, "phase22-private-sentinel") {
		t.Fatalf("temporary status leaked Environment-private Memory: %s", statusText)
	}

	writerOwner := "phase22-http-writer"
	acquired := callGatewayTool(t, ctx, agent, "environment_writer_acquire", map[string]any{"environment_id": created.Environment.ID, "owner": writerOwner})
	if acquired.IsError {
		t.Fatalf("writer acquire failed: %s", toolText(t, acquired))
	}
	beforeReadOnly, err := service.Environments.Get(created.Environment.ID)
	if err != nil || beforeReadOnly.Writer == nil {
		t.Fatalf("writer missing before read-only temporary lifecycle calls: env=%+v err=%v", beforeReadOnly, err)
	}
	readOnlyStatus := callGatewayTool(t, ctx, agent, "environment_temporary_status", map[string]any{"environment_id": created.Environment.ID})
	if readOnlyStatus.IsError {
		t.Fatalf("read-only temporary status failed: %s", toolText(t, readOnlyStatus))
	}
	readOnlyPreview := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "lifecycle-owner-a"})
	if readOnlyPreview.IsError {
		t.Fatalf("read-only temporary cleanup preview failed: %s", toolText(t, readOnlyPreview))
	}
	afterReadOnly, err := service.Environments.Get(created.Environment.ID)
	if err != nil || afterReadOnly.Writer == nil {
		t.Fatalf("writer missing after read-only temporary lifecycle calls: env=%+v err=%v", afterReadOnly, err)
	}
	if !afterReadOnly.Writer.LastSeenAt.Equal(beforeReadOnly.Writer.LastSeenAt) || !afterReadOnly.Writer.ExpiresAt.Equal(beforeReadOnly.Writer.ExpiresAt) {
		t.Fatalf("temporary status/preview renewed writer: before=%+v after=%+v", beforeReadOnly.Writer, afterReadOnly.Writer)
	}
	written := callGatewayTool(t, ctx, agent, "write", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "path": "agent.txt", "content": "phase22-agent-write\n"})
	if written.IsError {
		t.Fatalf("temporary Environment ordinary write failed: %s", toolText(t, written))
	}
	released := callGatewayTool(t, ctx, agent, "environment_writer_release", map[string]any{"environment_id": created.Environment.ID, "owner": writerOwner})
	if released.IsError {
		t.Fatalf("writer release failed: %s", toolText(t, released))
	}
	read := callGatewayTool(t, ctx, agent, "read", map[string]any{"environment_id": created.Environment.ID, "path": "agent.txt"})
	if read.IsError || !strings.Contains(toolText(t, read), "phase22-agent-write") {
		t.Fatalf("temporary Environment ordinary read failed: %s", toolText(t, read))
	}

	wrongPromote := callGatewayTool(t, ctx, agent, "environment_temporary_promote", map[string]any{"environment_id": created.Environment.ID, "owner_id": "wrong-owner"})
	if !wrongPromote.IsError {
		t.Fatalf("wrong lifecycle owner promoted temporary Environment: %s", toolText(t, wrongPromote))
	}
	wrongCleanup := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "wrong-owner", "execute": true})
	if !wrongCleanup.IsError {
		t.Fatalf("wrong lifecycle owner executed temporary cleanup: %s", toolText(t, wrongCleanup))
	}

	originalID, originalRoot := created.Environment.ID, created.Environment.Root
	promotedTool := callGatewayTool(t, ctx, admin, "environment_temporary_promote", map[string]any{"environment_id": created.Environment.ID, "owner_id": "lifecycle-owner-a"})
	if promotedTool.IsError {
		t.Fatalf("matching lifecycle owner promote failed: %s", toolText(t, promotedTool))
	}
	promoted := temporaryStatusFromTool(t, promotedTool)
	if promoted.EnvironmentID != originalID || promoted.Root != originalRoot || promoted.Retention.Persistence != model.PersistenceDurable || promoted.Retention.OwnerID != "" || promoted.Retention.ExpiresAt != nil {
		t.Fatalf("promotion changed context or failed to become durable: %+v", promoted)
	}
	entry, err := service.Memory.EnvironmentRead(originalID, "private")
	if err != nil || entry.Value != "phase22-private-sentinel" {
		t.Fatalf("promotion lost private Memory: entry=%+v err=%v", entry, err)
	}
	global, err := service.Memory.GlobalList()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range global {
		if entry.Value == "phase22-private-sentinel" {
			t.Fatalf("promotion leaked private Memory to Global scope: %+v", global)
		}
	}
	afterPromoteCleanup := callGatewayTool(t, ctx, admin, "environment_temporary_cleanup", map[string]any{"environment_id": originalID, "owner_id": "lifecycle-owner-a", "execute": true})
	if !afterPromoteCleanup.IsError {
		t.Fatalf("temporary cleanup removed promoted durable Environment: %s", toolText(t, afterPromoteCleanup))
	}

	firstRoot := filepath.Join(root, "temp-one")
	secondRoot := filepath.Join(root, "temp-two")
	for _, path := range []string{firstRoot, secondRoot} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	first := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "target-one", "owner_id": "target-owner", "ttl_seconds": 1, "root": firstRoot}))
	second := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "target-two", "owner_id": "target-owner", "ttl_seconds": 1, "root": secondRoot}))
	time.Sleep(1200 * time.Millisecond)
	preview := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": first.Environment.ID, "owner_id": "target-owner"})
	if preview.IsError {
		t.Fatalf("targeted cleanup preview failed: %s", toolText(t, preview))
	}
	previewText := toolText(t, preview)
	if !strings.Contains(previewText, first.Environment.ID) || strings.Contains(previewText, second.Environment.ID) {
		t.Fatalf("preview was not targeted to one Environment: %s", previewText)
	}
	if _, err := service.Environments.Get(first.Environment.ID); err != nil {
		t.Fatalf("preview removed first Environment: %v", err)
	}
	if _, err := service.Environments.Get(second.Environment.ID); err != nil {
		t.Fatalf("preview removed second Environment: %v", err)
	}
	executed := callGatewayTool(t, ctx, admin, "environment_temporary_cleanup", map[string]any{"environment_id": first.Environment.ID, "owner_id": "target-owner", "execute": true})
	if executed.IsError {
		t.Fatalf("targeted cleanup execute failed: %s", toolText(t, executed))
	}
	if _, err := service.Environments.Get(first.Environment.ID); err == nil {
		t.Fatalf("targeted cleanup did not remove first Environment")
	}
	if _, err := service.Environments.Get(second.Environment.ID); err != nil {
		t.Fatalf("targeted cleanup swept unrelated eligible Environment: %v", err)
	}
	if info, err := os.Stat(firstRoot); err != nil || !info.IsDir() {
		t.Fatalf("ordinary targeted cleanup touched project directory: info=%v err=%v", info, err)
	}

	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_TEST_AGENT_RUN_HELPER", "1")
	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "stream_large")
	restartWriter := "phase22-restart-writer"
	acquireTemporaryWriter(t, ctx, agent, second.Environment.ID, restartWriter)
	restartRun := callGatewayTool(t, ctx, agent, "run_start", map[string]any{"environment_id": second.Environment.ID, "writer_owner": restartWriter, "executable": os.Args[0], "args": []string{"-test.run=^TestAgentRunCommandHelper$"}, "timeout_ms": 10000})
	if restartRun.IsError {
		t.Fatalf("restart evidence Run start failed: %s", toolText(t, restartRun))
	}
	restartRunID := runIDFromToolText(t, restartRun)
	releaseTemporaryWriter(t, ctx, agent, second.Environment.ID, restartWriter)
	listedBeforeRestart := callGatewayTool(t, ctx, agent, "run_list", map[string]any{"environment_id": second.Environment.ID})
	if listedBeforeRestart.IsError || !strings.Contains(toolText(t, listedBeforeRestart), restartRunID) {
		t.Fatalf("restart evidence Run was not owner-observable before restart: %s", toolText(t, listedBeforeRestart))
	}

	_ = agent.Close()
	_ = admin.Close()
	server.Close()
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), restartRunID) {
		t.Fatalf("owner-local Run observation leaked into persisted state: %s", restartRunID)
	}
	freshOwner := newRuntimeOwner(service)
	defer freshOwner.Close()
	freshServer := httptest.NewServer(newHTTPHandler(service, freshOwner))
	defer freshServer.Close()
	fresh := connectHTTPWithRetry(t, ctx, freshServer.URL+"/mcp")
	defer fresh.Close()
	freshStatus := callGatewayTool(t, ctx, fresh, "environment_temporary_status", map[string]any{"environment_id": second.Environment.ID})
	if freshStatus.IsError || !strings.Contains(toolText(t, freshStatus), "target-owner") {
		t.Fatalf("temporary retention did not survive Gateway restart: %s", toolText(t, freshStatus))
	}
	freshRuns := callGatewayTool(t, ctx, fresh, "run_list", map[string]any{"environment_id": second.Environment.ID})
	if freshRuns.IsError || strings.Contains(toolText(t, freshRuns), restartRunID) {
		t.Fatalf("fresh Gateway owner resurrected runtime Run observation %s: %s", restartRunID, toolText(t, freshRuns))
	}
}

func TestTemporaryEnvironmentManagedHTTPIsolationAndSafety(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	source := gatewayTestGitRepo(t)
	workspace, err := service.Workspaces.Add(source, "phase22-managed-source")
	if err != nil {
		t.Fatal(err)
	}
	beforeBranch := gatewayGitRun(t, source, "branch", "--show-current")
	beforeHead := gatewayGitRun(t, source, "rev-parse", "HEAD")

	owner := newRuntimeOwner(service)
	defer owner.Close()
	server := httptest.NewServer(newHTTPHandler(service, owner))
	defer server.Close()
	ctx := context.Background()
	agent := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	defer agent.Close()

	created := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"workspace_id": workspace.ID,
		"name":         "phase22-managed-clean",
		"owner_id":     "managed-owner",
		"ttl_seconds":  1,
		"mode":         model.TemporaryEnvironmentModeManagedWorktree,
		"branch_name":  "test/phase22-clean",
		"base_ref":     "HEAD",
	}))
	if created.ManagedWorktree == nil || created.Environment.Retention.Persistence != model.PersistenceTemporary || created.Environment.Retention.CreatorSurface != string(serverSurfaceAgent) {
		t.Fatalf("managed temporary create lost identity/retention: %+v", created)
	}
	managed := *created.ManagedWorktree
	if gatewayGitRun(t, source, "branch", "--show-current") != beforeBranch || gatewayGitRun(t, source, "rev-parse", "HEAD") != beforeHead {
		t.Fatal("managed temporary create changed source checkout")
	}
	time.Sleep(1200 * time.Millisecond)
	preview := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "managed-owner"})
	if preview.IsError || !strings.Contains(toolText(t, preview), created.Environment.ID) {
		t.Fatalf("managed cleanup preview failed: %s", toolText(t, preview))
	}
	executed := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "managed-owner", "execute": true})
	if executed.IsError {
		t.Fatalf("managed cleanup execute failed: %s", toolText(t, executed))
	}
	if _, err := os.Stat(managed.Root); !os.IsNotExist(err) {
		t.Fatalf("managed cleanup did not remove ADM-owned worktree: %v", err)
	}
	if out, err := gatewayGitCommand(source, "show-ref", "--verify", "refs/heads/"+managed.Branch); err != nil {
		t.Fatalf("managed cleanup did not retain branch: %v\n%s", err, out)
	}

	sourceEnvironment, err := service.Environments.Create(workspace.ID, "phase22-source-environment", "")
	if err != nil {
		t.Fatal(err)
	}
	derived := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{
		"source_environment_id": sourceEnvironment.ID,
		"name":                  "phase22-managed-from-environment",
		"owner_id":              "managed-owner",
		"ttl_seconds":           60,
		"mode":                  model.TemporaryEnvironmentModeManagedWorktree,
		"branch_name":           "test/phase22-derived",
		"base_ref":              "HEAD",
	}))
	if derived.ManagedWorktree == nil || derived.ManagedWorktree.SourceEnvironmentID != sourceEnvironment.ID || !pathutil.Same(derived.ManagedWorktree.SourceRoot, sourceEnvironment.Root) {
		t.Fatalf("source_environment_id was not preserved through Gateway temporary create: %+v", derived)
	}
	if _, err := service.Environments.AcquireWriter(derived.Environment.ID, "derived-cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Isolation.Destroy(ctx, derived.Environment.ID, "derived-cleanup", true); err != nil {
		t.Fatal(err)
	}

	dirty := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "phase22-managed-dirty", "owner_id": "managed-owner", "ttl_seconds": 1, "mode": model.TemporaryEnvironmentModeManagedWorktree, "branch_name": "test/phase22-dirty"}))
	unpublished := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "phase22-managed-unpublished", "owner_id": "managed-owner", "ttl_seconds": 1, "mode": model.TemporaryEnvironmentModeManagedWorktree, "branch_name": "test/phase22-unpublished"}))
	tampered := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "phase22-managed-tampered", "owner_id": "managed-owner", "ttl_seconds": 1, "mode": model.TemporaryEnvironmentModeManagedWorktree, "branch_name": "test/phase22-tampered"}))
	defer func() {
		for _, item := range []*model.ManagedWorktree{dirty.ManagedWorktree, unpublished.ManagedWorktree, tampered.ManagedWorktree} {
			if item == nil {
				continue
			}
			_, _ = gatewayGitCommand(source, "worktree", "remove", "--force", item.Root)
			_, _ = gatewayGitCommand(source, "branch", "-D", item.Branch)
		}
	}()
	if err := os.WriteFile(filepath.Join(dirty.ManagedWorktree.Root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unpublished.ManagedWorktree.Root, "tracked.txt"), []byte("local commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gatewayGitRun(t, unpublished.ManagedWorktree.Root, "add", "tracked.txt")
	gatewayGitRun(t, unpublished.ManagedWorktree.Root, "commit", "-m", "local only")
	gatewayGitRun(t, tampered.ManagedWorktree.Root, "checkout", "-b", "phase22-tampered-branch")
	time.Sleep(1200 * time.Millisecond)
	for _, tc := range []struct {
		id      string
		blocker string
	}{
		{dirty.Environment.ID, "managed_worktree_dirty"},
		{unpublished.Environment.ID, "managed_worktree_unpublished"},
		{tampered.Environment.ID, "managed_worktree_safety_check_failed"},
	} {
		preview := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": tc.id, "owner_id": "managed-owner"})
		if preview.IsError || !strings.Contains(toolText(t, preview), tc.blocker) {
			t.Fatalf("managed blocker %s missing for %s: %s", tc.blocker, tc.id, toolText(t, preview))
		}
		execute := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": tc.id, "owner_id": "managed-owner", "execute": true})
		if !execute.IsError {
			t.Fatalf("unsafe managed Environment %s was cleanup-executable: %s", tc.id, toolText(t, execute))
		}
		if _, err := service.Environments.Get(tc.id); err != nil {
			t.Fatalf("unsafe managed Environment %s was removed: %v", tc.id, err)
		}
	}

	plainRoot := t.TempDir()
	plainWS, err := service.Workspaces.Add(plainRoot, "phase22-non-git")
	if err != nil {
		t.Fatal(err)
	}
	managedFailure := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": plainWS.ID, "name": "non-git-managed", "owner_id": "plain-owner", "ttl_seconds": 60, "mode": model.TemporaryEnvironmentModeManagedWorktree, "branch_name": "test/non-git-managed"})
	if !managedFailure.IsError {
		t.Fatalf("non-Git Workspace unexpectedly accepted managed-worktree mode: %s", toolText(t, managedFailure))
	}
	plainSuccess := callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": plainWS.ID, "name": "non-git-existing-root", "owner_id": "plain-owner", "ttl_seconds": 60})
	if plainSuccess.IsError {
		t.Fatalf("managed-worktree failure poisoned ordinary existing-root mode: %s", toolText(t, plainSuccess))
	}
}

func TestTemporaryEnvironmentHTTPActiveRuntimeBlockers(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "adm", "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "phase22-runtime-blockers")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(os.Args[0]); err != nil {
		t.Fatal(err)
	}

	owner := newRuntimeOwner(service)
	defer owner.Close()
	server := httptest.NewServer(newHTTPHandler(service, owner))
	defer server.Close()
	ctx := context.Background()
	agent := connectHTTPWithRetry(t, ctx, server.URL+"/mcp")
	defer agent.Close()

	created := temporaryCreateFromTool(t, callGatewayTool(t, ctx, agent, "environment_temporary_create", map[string]any{"workspace_id": workspace.ID, "name": "phase22-runtime", "owner_id": "runtime-owner", "ttl_seconds": 1}))
	definition, err := service.AddVerifier(created.Environment.ID, model.VerifierDefinition{Name: "phase22-long", Kind: verifier.KindTest, Enabled: true, Executable: os.Args[0], Args: []string{"-test.run=^TestAsyncVerifierCommandHelper$"}, TimeoutSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	writerOwner := "phase22-runtime-writer"

	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_writer", true)
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_writer", false)

	portFile := filepath.Join(t.TempDir(), "port.txt")
	t.Setenv("ADM_TEST_DEV_PROCESS_HELPER", "1")
	t.Setenv("ADM_TEST_DEV_PROCESS_PORT_FILE", portFile)
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	processStart := callGatewayTool(t, ctx, agent, "process_start", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "executable": os.Args[0], "args": []string{"-test.run=^TestDevProcessHelper$"}})
	if processStart.IsError {
		t.Fatalf("process start failed: %s", toolText(t, processStart))
	}
	processID := processIDFromToolText(t, processStart)
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_process", true)
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	stopped := callGatewayTool(t, ctx, agent, "process_stop", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "process_id": processID})
	if stopped.IsError {
		t.Fatalf("process stop failed: %s", toolText(t, stopped))
	}
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_process", false)

	t.Setenv("ADM_TEST_AGENT_RUN_HELPER", "1")
	t.Setenv("ADM_TEST_AGENT_RUN_MODE", "stream_large")
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	runStart := callGatewayTool(t, ctx, agent, "run_start", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "executable": os.Args[0], "args": []string{"-test.run=^TestAgentRunCommandHelper$"}, "timeout_ms": 10000})
	if runStart.IsError {
		t.Fatalf("Run start failed: %s", toolText(t, runStart))
	}
	runID := runIDFromToolText(t, runStart)
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_run", true)
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	canceledRun := callGatewayTool(t, ctx, agent, "run_cancel", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "run_id": runID})
	if canceledRun.IsError {
		t.Fatalf("Run cancel failed: %s", toolText(t, canceledRun))
	}
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_run", false)

	t.Setenv("ADM_TEST_ASYNC_VERIFIER_HELPER", "1")
	t.Setenv("ADM_TEST_ASYNC_VERIFIER_MODE", "long")
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	verifierStart := callGatewayTool(t, ctx, agent, "environment_verifier_run_start", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "verifier_id": definition.ID})
	if verifierStart.IsError {
		t.Fatalf("verifier Run start failed: %s", toolText(t, verifierStart))
	}
	verifierRunID := verifierRunIDFromToolText(t, verifierStart)
	waitVerifierHTTPOutput(t, ctx, agent, created.Environment.ID, verifierRunID, "phase21-long-ready")
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_verifier_run", true)
	blockedCleanup := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "runtime-owner", "execute": true})
	if !blockedCleanup.IsError || !strings.Contains(toolText(t, blockedCleanup), "active_verifier_run") {
		t.Fatalf("real vfrun did not block cleanup: %s", toolText(t, blockedCleanup))
	}
	acquireTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	canceledVerifier := callGatewayTool(t, ctx, agent, "environment_verifier_run_cancel", map[string]any{"environment_id": created.Environment.ID, "writer_owner": writerOwner, "verifier_run_id": verifierRunID})
	if canceledVerifier.IsError {
		t.Fatalf("verifier Run cancel failed: %s", toolText(t, canceledVerifier))
	}
	releaseTemporaryWriter(t, ctx, agent, created.Environment.ID, writerOwner)
	assertTemporaryStatusBlocker(t, ctx, agent, created.Environment.ID, "active_verifier_run", false)

	cleanup := callGatewayTool(t, ctx, agent, "environment_temporary_cleanup", map[string]any{"environment_id": created.Environment.ID, "owner_id": "runtime-owner", "execute": true})
	if cleanup.IsError {
		t.Fatalf("cleanup remained blocked after explicit runtime teardown: %s", toolText(t, cleanup))
	}
}

func temporaryCreateFromTool(t *testing.T, result *mcp.CallToolResult) model.TemporaryEnvironmentCreateResult {
	t.Helper()
	if result.IsError {
		t.Fatalf("temporary create tool error: %s", toolText(t, result))
	}
	var envelope struct {
		Result model.TemporaryEnvironmentCreateResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(toolText(t, result)), &envelope); err != nil {
		t.Fatalf("decode temporary create: %v body=%s", err, toolText(t, result))
	}
	return envelope.Result
}

func temporaryStatusFromTool(t *testing.T, result *mcp.CallToolResult) model.TemporaryEnvironmentStatus {
	t.Helper()
	if result.IsError {
		t.Fatalf("temporary status tool error: %s", toolText(t, result))
	}
	var envelope struct {
		Result model.TemporaryEnvironmentStatus `json:"result"`
	}
	if err := json.Unmarshal([]byte(toolText(t, result)), &envelope); err != nil {
		t.Fatalf("decode temporary status: %v body=%s", err, toolText(t, result))
	}
	return envelope.Result
}

func assertTemporaryStatusBlocker(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, blocker string, want bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		result := callGatewayTool(t, ctx, session, "environment_temporary_status", map[string]any{"environment_id": environmentID})
		if result.IsError {
			t.Fatalf("temporary status failed while checking %s: %s", blocker, toolText(t, result))
		}
		has := strings.Contains(toolText(t, result), `"`+blocker+`"`)
		if has == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("temporary blocker %s want=%v was not observed", blocker, want)
}

func acquireTemporaryWriter(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, writerOwner string) {
	t.Helper()
	result := callGatewayTool(t, ctx, session, "environment_writer_acquire", map[string]any{"environment_id": environmentID, "owner": writerOwner})
	if result.IsError {
		t.Fatalf("writer acquire failed: %s", toolText(t, result))
	}
}

func releaseTemporaryWriter(t *testing.T, ctx context.Context, session *mcp.ClientSession, environmentID, writerOwner string) {
	t.Helper()
	result := callGatewayTool(t, ctx, session, "environment_writer_release", map[string]any{"environment_id": environmentID, "owner": writerOwner})
	if result.IsError {
		t.Fatalf("writer release failed: %s", toolText(t, result))
	}
}
