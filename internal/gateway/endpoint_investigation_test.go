package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayInvestigateEndpointReturnsEvidence(t *testing.T) {
	root := t.TempDir()
	routePath := filepath.Join(root, "api", "routes.go")
	if err := os.MkdirAll(filepath.Dir(routePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routePath, []byte("package api\n\nfunc register() {\n\tmux.Handle(\"GET\", \"/v1/accounts/{id}\", handler)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "gateway-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "gateway-endpoint", "")
	if err != nil {
		t.Fatal(err)
	}

	session := connectInMemory(t, context.Background(), newServer(service, nil))
	defer session.Close()
	result := callGatewayTool(t, context.Background(), session, "investigate_endpoint", map[string]any{
		"environment_id": environment.ID,
		"target":         "/v1/accounts/123",
		"method":         "GET",
	})
	if result.IsError {
		t.Fatalf("investigate_endpoint failed: %s", toolText(t, result))
	}
	text := toolText(t, result)
	for _, required := range []string{"/v1/accounts/{id}", "api/routes.go", "confidence", "uncertainties", "static_literal_search_only"} {
		if !strings.Contains(text, required) {
			t.Fatalf("investigate_endpoint output missing %q:\n%s", required, text)
		}
	}
}
