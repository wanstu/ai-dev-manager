package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestPHPCodeIntelligenceReturnsReasonsForAIInvestigation(t *testing.T) {
	root := t.TempDir()
	sources := map[string]string{
		"Base.php": `<?php
class Base {
 public function run() {}
}
`,
		"Child.php": `<?php
class Child extends Base {
 public function action(Base $service,$unknown) {
  $this->run();
  $service->run();
  $unknown->run();
 }
}
`,
	}
	for file, src := range sources {
		if err := os.WriteFile(filepath.Join(root, file), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "ai-call-evidence")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "ai-call-evidence", "")
	if err != nil {
		t.Fatal(err)
	}
	const owner = "ai-call-evidence-test"
	if _, err := service.Environments.AcquireWriter(env.ID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(env.ID, owner, 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(env.ID, owner, false); err != nil {
		t.Fatal(err)
	}
	runtime := newRuntimeOwner(service)
	defer runtime.Close()
	session := connectInMemory(t, context.Background(), newServer(service, runtime))
	defer session.Close()
	refs := callGatewayTool(t, context.Background(), session, "code_intelligence_references", map[string]any{
		"environment_id": env.ID, "symbol": map[string]any{
			"name": "Base::run", "path": "Base.php", "language": "PHP", "kind": "method",
		},
	})
	if refs.IsError {
		t.Fatalf("references: %s", toolText(t, refs))
	}
	refText := toolText(t, refs)
	for _, part := range []string{
		`"kind":"inherited_candidate"`,
		`"reason":"direct_extends_clause"`,
		`"reason":"parameter_type_matches_definition"`,
		`"type_hint":"Base"`,
	} {
		if !strings.Contains(refText, part) {
			t.Fatalf("reference missing %q in %s", part, refText)
		}
	}
	graph := callGatewayTool(t, context.Background(), session, "project_call_graph", map[string]any{
		"environment_id": env.ID, "symbol": "Child::action", "direction": "callees", "max_depth": 3,
	})
	if graph.IsError {
		t.Fatalf("graph: %s", toolText(t, graph))
	}
	graphText := toolText(t, graph)
	for _, part := range []string{
		`"kind":"inherited_candidate"`,
		`"reason":"declared_parameter_type_not_runtime"`,
		`"type_hint":"Base"`,
	} {
		if !strings.Contains(graphText, part) {
			t.Fatalf("graph missing %q in %s", part, graphText)
		}
	}
}
