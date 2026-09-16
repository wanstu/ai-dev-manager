package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWorkspaceDiscoveryAvailableOnAgentAndAdminSurfaces(t *testing.T) {
	root := t.TempDir()
	for _, project := range []string{"projects/p1", "projects/p2"} {
		projectRoot := filepath.Join(root, filepath.FromSlash(project))
		if err := os.MkdirAll(projectRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectRoot, "package.json"), []byte("sentinel-content-must-not-be-read"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "discovery-surface")
	if err != nil {
		t.Fatal(err)
	}

	request := map[string]any{"workspace_id": workspace.ID, "path": "projects", "max_depth": 3, "max_entries": 100}
	agentSession := connectInMemory(t, context.Background(), newServer(service, nil))
	defer agentSession.Close()
	adminSession := connectInMemory(t, context.Background(), newServerForSurface(service, nil, serverSurfaceAdmin))
	defer adminSession.Close()

	agent := decodeDiscoveryReport(t, callGatewayTool(t, context.Background(), agentSession, "workspace_discover", request))
	admin := decodeDiscoveryReport(t, callGatewayTool(t, context.Background(), adminSession, "workspace_discover", request))
	if len(agent.Candidates) != 2 {
		t.Fatalf("agent candidates=%+v", agent.Candidates)
	}
	agent.ObservedAt = time.Time{}
	admin.ObservedAt = time.Time{}
	if !reflect.DeepEqual(agent, admin) {
		t.Fatalf("Agent/Admin discovery reports differ\nagent=%+v\nadmin=%+v", agent, admin)
	}
	for label, session := range map[string]*mcp.ClientSession{"agent": agentSession, "admin": adminSession} {
		bad := callGatewayTool(t, context.Background(), session, "workspace_discover", map[string]any{"workspace_id": workspace.ID, "path": ".."})
		if !bad.IsError {
			t.Fatalf("%s workspace discovery accepted out-of-scope path: %+v", label, bad.StructuredContent)
		}
	}
}

func TestEnvironmentTreeDigestAvailableOnAgentAndAdminSurfaces(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "project", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sibling", "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "digest-surface")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "project", filepath.Join(root, "project"))
	if err != nil {
		t.Fatal(err)
	}

	request := map[string]any{"environment_id": environment.ID, "max_depth": 3, "max_entries": 100}
	agentSession := connectInMemory(t, context.Background(), newServer(service, nil))
	defer agentSession.Close()
	adminSession := connectInMemory(t, context.Background(), newServerForSurface(service, nil, serverSurfaceAdmin))
	defer adminSession.Close()

	agent := decodeDiscoveryReport(t, callGatewayTool(t, context.Background(), agentSession, "environment_tree_digest", request))
	admin := decodeDiscoveryReport(t, callGatewayTool(t, context.Background(), adminSession, "environment_tree_digest", request))
	if agent.Scope.EnvironmentID != environment.ID || admin.Scope.EnvironmentID != environment.ID {
		t.Fatalf("digest scope mismatch agent=%+v admin=%+v", agent.Scope, admin.Scope)
	}
	for _, entry := range agent.Digest {
		if entry.Path == "../sibling" || entry.Path == "sibling" {
			t.Fatalf("environment digest escaped root: %+v", agent.Digest)
		}
	}

	bad := callGatewayTool(t, context.Background(), agentSession, "environment_tree_digest", map[string]any{"environment_id": environment.ID, "path": "../sibling"})
	if !bad.IsError {
		t.Fatalf("out-of-scope environment digest unexpectedly succeeded: %+v", bad.StructuredContent)
	}
}

func TestWorkspaceDiscoverySurfacesRejectUnknownStableIDs(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()
	for _, surface := range []serverSurface{serverSurfaceAgent, serverSurfaceAdmin} {
		session := connectInMemory(t, ctx, newServerForSurface(service, nil, surface))
		for _, call := range []struct {
			name string
			args map[string]any
		}{
			{name: "workspace_discover", args: map[string]any{"workspace_id": "ws_missing"}},
			{name: "environment_tree_digest", args: map[string]any{"environment_id": "env_missing"}},
		} {
			result := callGatewayTool(t, ctx, session, call.name, call.args)
			if !result.IsError {
				_ = session.Close()
				t.Fatalf("%v %s unexpectedly accepted missing stable ID: %+v", surface, call.name, result.StructuredContent)
			}
		}
		_ = session.Close()
	}
}

func decodeDiscoveryReport(t *testing.T, result *mcp.CallToolResult) model.DiscoveryReport {
	t.Helper()
	if result == nil || result.IsError {
		t.Fatalf("discovery tool failed: %s", toolText(t, result))
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var report model.DiscoveryReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode discovery report: %v; raw=%s", err, raw)
	}
	return report
}
