package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestCodeIntelligencePHPReferenceCandidatesWithoutExternalProvider(t *testing.T) {
	root := t.TempDir()
	source := `<?php
class User_goods {
  public function getDealBaseInfo() {}
  public function action() {
    $this->getDealBaseInfo();
  }
}
`
	if err := os.WriteFile(filepath.Join(root, "User_goods.php"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "static-php-reference")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "static-php-reference", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "static-php-reference-index"
	if _, err := service.Environments.AcquireWriter(environment.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(environment.ID, writer, 200, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(environment.ID, writer, false); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()

	call := func() string {
		t.Helper()
		result := callGatewayTool(t, context.Background(), session, "code_intelligence_references", map[string]any{
			"environment_id": environment.ID,
			"symbol": map[string]any{
				"path": "User_goods.php", "name": "User_goods::getDealBaseInfo",
				"kind": "method", "language": "PHP",
			},
			"max_results": 20,
		})
		if result.IsError {
			t.Fatalf("PHP references failed: %s", toolText(t, result))
		}
		return toolText(t, result)
	}
	got := call()
	for _, part := range []string{
		`"provider_id":"adm_static_index"`, `"available":true`,
		`"kind":"candidate_call"`, `"name":"getDealBaseInfo"`,
		`"returned":1`,
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("PHP reference result missing %q: %s", part, got)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "User_goods.php"), []byte(source+"\n// update"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := call()
	if !strings.Contains(stale, `"available":false`) {
		t.Fatalf("stale reference index must not be considered available: %s", stale)
	}
}
