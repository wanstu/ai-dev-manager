package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGatewayWriterAcquireRejectsOverlappingRootsAndAllowsSiblings(t *testing.T) {
	root := t.TempDir()
	workRoot := filepath.Join(root, "work")
	groupRoot := filepath.Join(workRoot, "wm_group")
	group1Root := filepath.Join(workRoot, "wm_group1")
	for _, dir := range []string{workRoot, groupRoot, group1Root} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "writer-overlap")
	if err != nil {
		t.Fatal(err)
	}
	parentEnv, err := service.Environments.Create(ws.ID, "work", workRoot)
	if err != nil {
		t.Fatal(err)
	}
	groupEnv, err := service.Environments.Create(ws.ID, "wm_group", groupRoot)
	if err != nil {
		t.Fatal(err)
	}
	group1Env, err := service.Environments.Create(ws.ID, "wm_group1", group1Root)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	session := connectInMemory(t, ctx, New(service))
	defer session.Close()

	acquire := func(environmentID, owner string) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "environment_writer_acquire",
			Arguments: map[string]any{"environment_id": environmentID, "owner": owner},
		})
		if err != nil {
			t.Fatalf("writer acquire transport error: %v", err)
		}
		return result
	}
	release := func(environmentID, owner string) {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "environment_writer_release",
			Arguments: map[string]any{"environment_id": environmentID, "owner": owner},
		})
		if err != nil || result.IsError {
			t.Fatalf("writer release failed: err=%v result=%+v", err, result)
		}
	}

	if result := acquire(parentEnv.ID, "parent-owner"); result.IsError {
		t.Fatalf("parent writer acquire failed: %+v", result)
	}
	if result := acquire(groupEnv.ID, "child-owner"); !result.IsError {
		t.Fatal("descendant environment unexpectedly acquired writer while ancestor root was owned")
	} else if text := toolText(t, result); !strings.Contains(text, "overlaps active writer") {
		t.Fatalf("overlap denial should explain the conflicting scope: %s", text)
	}
	release(parentEnv.ID, "parent-owner")

	if result := acquire(groupEnv.ID, "shared-owner"); result.IsError {
		t.Fatalf("child writer acquire failed: %+v", result)
	}
	if result := acquire(parentEnv.ID, "shared-owner"); !result.IsError {
		t.Fatal("ancestor environment unexpectedly acquired writer while descendant root was owned by same owner")
	}
	if result := acquire(group1Env.ID, "sibling-owner"); result.IsError {
		t.Fatalf("sibling root should acquire writer concurrently: %s", toolText(t, result))
	}
}
