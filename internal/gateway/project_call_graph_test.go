package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestProjectCallGraphToolUsesADMIndexWithoutExternalProvider(t *testing.T) {
	root := t.TempDir()
	sources := map[string]string{
		"Worker.php": `<?php
namespace Demo;
class Worker {
  public function run() {
    self::helper();
  }
  public static function helper() {}
}
`,
		"Controller.php": `<?php
namespace Demo;
class Controller {
  public function action($unknown) {
    Worker::run();
    $unknown->run();
  }
}
`,
	}
	for name, src := range sources {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "call-graph")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "call-graph", "")
	if err != nil {
		t.Fatal(err)
	}
	const writer = "test-call-graph"
	if _, err := service.Environments.AcquireWriter(env.ID, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnalyzeProject(env.ID, writer, 100, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.ReleaseWriter(env.ID, writer, false); err != nil {
		t.Fatal(err)
	}
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, context.Background(), newServer(service, owner))
	defer session.Close()
	for _, tc := range []struct {
		direction string
		want      []string
	}{
		{"callers", []string{`"returned":2`, `"name":"Demo\\Controller::action"`, `"kind":"candidate_call"`, `"kind":"resolved_call"`}},
		{"callees", []string{`"returned":1`, `"name":"Demo\\Worker::helper"`, `"kind":"resolved_call"`}},
		{"both", []string{`"returned":3`}},
	} {
		result := callGatewayTool(t, context.Background(), session, "project_call_graph", map[string]any{
			"environment_id": env.ID, "symbol": `Demo\Worker::run`,
			"path": "Worker.php", "direction": tc.direction, "max_depth": 1,
		})
		if result.IsError {
			t.Fatalf("%s: %s", tc.direction, toolText(t, result))
		}
		body := toolText(t, result)
		for _, part := range tc.want {
			if !strings.Contains(body, part) {
				t.Fatalf("%s missing %s in %s", tc.direction, part, body)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Controller.php"), []byte(sources["Controller.php"]+"\n// changed"), 0644); err != nil {
		t.Fatal(err)
	}
	result := callGatewayTool(t, context.Background(), session, "project_call_graph", map[string]any{
		"environment_id": env.ID, "symbol": `Demo\Worker::run`, "direction": "callers",
	})
	if !result.IsError || !strings.Contains(toolText(t, result), "stale") {
		t.Fatalf("stale graph should fail: %+v", result)
	}
}
