package gateway

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdminWriterDiagnosticsShowsOverlappingBlockingLease(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "overlap-recovery")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := service.Environments.Create(ws.ID, "parent", root)
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.Environments.Create(ws.ID, "child", child)
	if err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	ctx := context.Background()
	server := httptest.NewServer(newHTTPHandler(service, owner))
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "overlap-agent", Version: "test"}, nil)
	agent, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	adminClient := adminmcp.New(server.URL + "/admin/mcp")
	adminPeer := mcp.NewClient(&mcp.Implementation{Name: "overlap-admin", Version: "test"}, nil)
	admin, err := adminPeer.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/admin/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	call := func(name string, args map[string]any, adminSurface bool) string {
		session := agent
		if adminSurface {
			session = admin
		}
		r := callGatewayTool(t, ctx, session, name, args)
		if r.IsError {
			t.Fatal(toolText(t, r))
		}
		return toolText(t, r)
	}
	const secret = "hidden-parent-credential"
	call("environment_writer_acquire", map[string]any{"environment_id": parent.ID, "owner": secret}, false)
	args := map[string]any{"environment_id": target.ID, "owner": "child-owner"}
	if r := callGatewayTool(t, ctx, agent, "environment_writer_acquire", args); !r.IsError {
		t.Fatal("overlapping lease unexpectedly acquired")
	}
	diag := call("environment_writer_diagnostics", map[string]any{"environment_id": target.ID}, true)
	if !strings.Contains(diag, `"active":false`) || !strings.Contains(diag, `"overlapping_writers":[{`) || !strings.Contains(diag, parent.ID) || strings.Contains(diag, secret) {
		t.Fatalf("incorrect diagnostics: %s", diag)
	}
	decoded, err := adminClient.WriterDiagnostics(target.ID)
	if err != nil || decoded.Active || len(decoded.OverlappingWriters) != 1 || decoded.OverlappingWriters[0].EnvironmentID != parent.ID {
		t.Fatalf("Desktop admin client failed to decode actual blocking scope: %+v %v", decoded, err)
	}
	if _, err := adminClient.WriterRelease(parent.ID, "", true); err != nil {
		t.Fatal(err)
	}
	decoded, err = adminClient.WriterDiagnostics(target.ID)
	if err != nil || decoded.Active || len(decoded.OverlappingWriters) != 0 || decoded.HandoffPending {
		t.Fatalf("release did not clear diagnostics: %+v %v", decoded, err)
	}
	call("environment_writer_acquire", args, false)
	call("write", map[string]any{"environment_id": target.ID, "writer_owner": "child-owner", "path": "recovered.txt", "content": "ok"}, false)
	data, err := os.ReadFile(filepath.Join(child, "recovered.txt"))
	if err != nil || string(data) != "ok" {
		t.Fatalf("write after release: %q %v", data, err)
	}
}
