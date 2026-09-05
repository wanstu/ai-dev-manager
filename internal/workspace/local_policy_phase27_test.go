package workspace

import (
	"testing"

	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/model"
)

func TestWorkspaceLocalPolicyComposesProjectPolicyButExplicitRuntimePolicyStillWins(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	store := config.NewStore(configRoot)
	registry := NewRegistry(store)
	service := NewConfigService(store, registry)

	ws, err := registry.Add(Input{Path: workspaceRoot})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Policy: &model.Policy{
			Mode:               "workspace-write",
			AllowedExecutables: []string{"helper"},
			ToolPaths:          map[string]string{"helper": "project-helper.exe"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SetLocalPolicy(ws.ID, &model.Policy{
		Mode:               "standard",
		AllowedExecutables: []string{"git", "go"},
		ToolPaths:          map[string]string{"go": "local-go.exe"},
	}); err != nil {
		t.Fatal(err)
	}

	effective, err := service.ResolveWorkspace(ws.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Policy == nil || effective.Policy.Policy.Mode != "standard" {
		t.Fatalf("local policy mode not applied: %+v", effective.Policy)
	}
	for _, executable := range []string{"helper", "git", "go"} {
		if !containsExecutable(effective.Policy.Policy.AllowedExecutables, executable) {
			t.Fatalf("effective policy missing %q: %+v", executable, effective.Policy.Policy)
		}
	}
	if effective.Policy.Policy.ToolPaths["helper"] != "project-helper.exe" || effective.Policy.Policy.ToolPaths["go"] != "local-go.exe" {
		t.Fatalf("effective tool paths = %+v", effective.Policy.Policy.ToolPaths)
	}

	runtimeOverride := &model.ConfigLayer{
		Scope: model.ScopeRuntime,
		Policy: &model.Policy{
			Mode:               "read-only",
			AllowedExecutables: []string{"runtime-only"},
			ToolPaths:          map[string]string{"runtime-only": "runtime.exe"},
		},
	}
	overridden, err := service.ResolveWorkspace(ws.ID, runtimeOverride)
	if err != nil {
		t.Fatal(err)
	}
	if overridden.Policy == nil || overridden.Policy.Policy.Mode != "read-only" || len(overridden.Policy.Policy.AllowedExecutables) != 1 || overridden.Policy.Policy.AllowedExecutables[0] != "runtime-only" || overridden.Policy.Policy.ToolPaths["runtime-only"] != "runtime.exe" {
		t.Fatalf("explicit runtime policy did not win: %+v", overridden.Policy)
	}
	if _, exists := overridden.Policy.Policy.ToolPaths["go"]; exists {
		t.Fatalf("workspace local policy leaked through explicit runtime override: %+v", overridden.Policy.Policy)
	}
}
