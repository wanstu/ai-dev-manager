package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestProjectInvestigatePHPReturnsDefinitionAndCallEvidenceInOneTool(t *testing.T) {
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("User_goods.php", `<?php
class User_goods {
 public function getDealBaseInfo() {
   return true;
 }
}
`+strings.Repeat("// unrelated source\n", 1500))
	write("User_goods_test.php", `<?php
class User_goods_test extends User_goods {
 public function check() {
  return $this->getDealBaseInfo();
 }
}
`)
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "investigate-php")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "investigate-php", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "investigate-php-test"
	if _, err := service.Environments.AcquireWriter(env.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(env.ID, writer, 300, 300); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(env.ID, writer, false); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()
	args := map[string]any{
		"environment_id": env.ID, "symbol": "User_goods::getDealBaseInfo",
		"path": "User_goods.php", "direction": "callers",
		"max_excerpts": 3, "context_lines": 2,
	}
	got := callGatewayTool(t, context.Background(), session, "project_investigate_php", args)
	if got.IsError {
		t.Fatalf("investigate: %s", toolText(t, got))
	}
	body := toolText(t, got)
	for _, want := range []string{
		`"role":"definition"`, `"role":"call_site"`,
		`"reason":"direct_extends_clause"`,
		`"kind":"inherited_candidate"`,
		`"name":"User_goods::getDealBaseInfo"`,
		`"path":"User_goods_test.php"`,
		`"start_line":3`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
	if strings.Count(body, "// unrelated source") > 1 {
		t.Fatalf("tool returned too much unrelated source instead of bounded evidence")
	}
	if len(body) > 9000 {
		t.Fatalf("tool response not bounded: %d bytes", len(body))
	}

	// Preserve the same stale-index rejection as the underlying graph tool.
	write("User_goods_test.php", `<?php
class User_goods_test extends User_goods {
 public function check() { return 1; }
}
`)
	stale := callGatewayTool(t, context.Background(), session, "project_investigate_php", args)
	if !stale.IsError || !strings.Contains(toolText(t, stale), "stale") {
		t.Fatalf("stale index must not be treated as reliable: %+v", stale)
	}
}
