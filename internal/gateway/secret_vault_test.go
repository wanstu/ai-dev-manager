package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestEncryptedSecretAdminOnlyLifecycle(t *testing.T) {
	s := app.New(filepath.Join(t.TempDir(), "state.json"))
	if !isAdminOnlyTool("secret_set") || !isAdminOnlyTool("secret_list") || !isAdminOnlyTool("secret_delete") {
		t.Fatal("vault management tools must never be visible in Agent MCP")
	}
	owner := newRuntimeOwner(s)
	defer owner.Close()
	ctx := context.Background()
	admin := connectInMemory(t, ctx, newServerForSurface(s, owner, serverSurfaceAdmin))
	defer admin.Close()
	raw := "a-sensitive-api-key-not-for-logs"
	put := callGatewayTool(t, ctx, admin, "secret_set", map[string]any{"name": "private-key", "value": raw})
	if put.IsError || strings.Contains(toolText(t, put), raw) {
		t.Fatalf("secret_set leaked or failed: %s", toolText(t, put))
	}
	listed := callGatewayTool(t, ctx, admin, "secret_list", map[string]any{})
	if listed.IsError || !strings.Contains(toolText(t, listed), "private-key") || strings.Contains(toolText(t, listed), raw) {
		t.Fatalf("secret_list leaked or failed: %s", toolText(t, listed))
	}
	state, _ := os.ReadFile(s.Store.Path())
	if strings.Contains(string(state), raw) {
		t.Fatal("secret persisted in ADM state")
	}
	del := callGatewayTool(t, ctx, admin, "secret_delete", map[string]any{"name": "private-key"})
	if del.IsError {
		t.Fatalf("cannot delete unused secret: %s", toolText(t, del))
	}
	agent := connectInMemory(t, ctx, newServerForSurface(s, owner, serverSurfaceAgent))
	defer agent.Close()
	tools, err := agent.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range tools.Tools {
		if strings.HasPrefix(item.Name, "secret_") {
			t.Fatalf("Agent exposed vault management tool %s", item.Name)
		}
	}
}
