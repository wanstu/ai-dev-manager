package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
)

func TestMCPIndexJobStaysAvailableAcrossRequests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Api.php"), []byte("<?php class Api { public function go() {} }"), 0644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "api-index")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "api-index", "")
	if err != nil {
		t.Fatal(err)
	}
	const agentWriter = "editor-session"
	if _, err := service.Environments.AcquireWriter(env.ID, agentWriter); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAgent))
	defer session.Close()
	started := callGatewayTool(t, ctx, session, "project_index_job_start", map[string]any{"environment_id": env.ID})
	if started.IsError || !strings.Contains(toolText(t, started), "job_id") {
		t.Fatalf("cannot start job: %s", toolText(t, started))
	}
	deadline := time.Now().Add(12 * time.Second)
	succeeded := false
	for time.Now().Before(deadline) {
		status := callGatewayTool(t, ctx, session, "project_index_job_status", map[string]any{"environment_id": env.ID})
		if status.IsError {
			t.Fatalf("cannot poll status: %s", toolText(t, status))
		}
		if strings.Contains(toolText(t, status), "\"state\": \"succeeded\"") || strings.Contains(toolText(t, status), "\"state\":\"succeeded\"") {
			succeeded = true
			break
		}
		if strings.Contains(toolText(t, status), "\"state\": \"failed\"") {
			t.Fatalf("job failed: %s", toolText(t, status))
		}
		time.Sleep(70 * time.Millisecond)
	}
	if !succeeded {
		t.Fatal("background index did not finish")
	}
	envNow, err := service.Environments.Get(env.ID)
	if err != nil || envNow.Writer == nil || envNow.Writer.Owner != agentWriter {
		t.Fatalf("source writer changed: %+v %v", envNow.Writer, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".adm")); !os.IsNotExist(err) {
		t.Fatalf("index appeared in source root: %v", err)
	}
}
