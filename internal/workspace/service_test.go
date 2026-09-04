package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/mcp"
	"ai-dev-manager/internal/model"
)

func TestConfigServiceResolvesSharedGlobalWithIsolatedProjectOverrides(t *testing.T) {
	store := config.NewStore(t.TempDir())
	global := config.UserConfig{
		Version: config.SchemaVersion,
		Global: model.ConfigLayer{
			Scope: model.ScopeGlobal,
			MCPs: map[string]model.MCPDefinition{
				"github": {ID: "github", Enabled: serviceBool(true), Command: "github-mcp"},
			},
			Skills: map[string]model.SkillDefinition{
				"gsd": {ID: "gsd", Enabled: serviceBool(true), Path: `D:\skills\gsd`},
			},
			Policy: &model.Policy{Mode: "workspace-write"},
		},
	}
	if err := store.SaveUserConfig(global); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}

	profile := model.ConfigLayer{
		Scope: model.ScopeProfile,
		MCPs: map[string]model.MCPDefinition{
			"work-db": {ID: "work-db", Command: "work-db-mcp"},
		},
		Policy: &model.Policy{Mode: "standard"},
	}
	if err := store.SaveProfile("work", profile); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	registry := NewRegistry(store)
	pathA := t.TempDir()
	pathB := t.TempDir()
	workspaceA, err := registry.Add(Input{Path: pathA, ProfileID: "work", RuntimeID: "native"})
	if err != nil {
		t.Fatalf("Add A error = %v", err)
	}
	workspaceB, err := registry.Add(Input{Path: pathB, ProfileID: "work", RuntimeID: "native"})
	if err != nil {
		t.Fatalf("Add B error = %v", err)
	}

	projectA := model.ConfigLayer{
		Scope: model.ScopeProject,
		MCPs: map[string]model.MCPDefinition{
			"github":    {ID: "github", Enabled: serviceBool(false)},
			"private-a": {ID: "private-a", Command: "private-a-mcp"},
		},
		Skills: map[string]model.SkillDefinition{
			"gsd": {ID: "gsd", Path: filepath.Join(pathA, "skills", "gsd")},
		},
	}
	if err := store.SaveProject(pathA, projectA); err != nil {
		t.Fatalf("SaveProject(A) error = %v", err)
	}

	service := NewConfigService(store, registry)
	effectiveA, err := service.ResolveWorkspace(workspaceA.ID, nil)
	if err != nil {
		t.Fatalf("ResolveWorkspace(A) error = %v", err)
	}
	effectiveB, err := service.ResolveWorkspace(workspaceB.ID, nil)
	if err != nil {
		t.Fatalf("ResolveWorkspace(B) error = %v", err)
	}

	assertServiceBool(t, effectiveA.MCPs["github"].Enabled, false)
	if effectiveA.MCPs["github"].EnabledSource != model.ScopeProject {
		t.Fatalf("A github enabled source = %q, want project", effectiveA.MCPs["github"].EnabledSource)
	}
	if _, ok := effectiveA.MCPs["private-a"]; !ok {
		t.Fatalf("A missing project-private MCP: %+v", effectiveA.MCPs)
	}
	if effectiveA.Skills["gsd"].Path != filepath.Join(pathA, "skills", "gsd") || effectiveA.Skills["gsd"].Source != model.ScopeProject {
		t.Fatalf("A project skill override missing: %+v", effectiveA.Skills["gsd"])
	}
	if _, ok := effectiveA.MCPs["work-db"]; !ok || effectiveA.Policy == nil || effectiveA.Policy.Source != model.ScopeProfile {
		t.Fatalf("A profile layer not resolved: MCPs=%+v Policy=%+v", effectiveA.MCPs, effectiveA.Policy)
	}

	assertServiceBool(t, effectiveB.MCPs["github"].Enabled, true)
	if effectiveB.MCPs["github"].EnabledSource != model.ScopeGlobal {
		t.Fatalf("B github enabled source = %q, want global", effectiveB.MCPs["github"].EnabledSource)
	}
	if _, ok := effectiveB.MCPs["private-a"]; ok {
		t.Fatalf("Project A private MCP leaked into B: %+v", effectiveB.MCPs)
	}
	if effectiveB.Skills["gsd"].Path != `D:\skills\gsd` || effectiveB.Skills["gsd"].Source != model.ScopeGlobal {
		t.Fatalf("Project A skill override leaked into B: %+v", effectiveB.Skills["gsd"])
	}
}

func TestConfigServiceRuntimeOverrideWinsWithoutPersistence(t *testing.T) {
	store := config.NewStore(t.TempDir())
	if err := store.SaveUserConfig(config.UserConfig{
		Version: config.SchemaVersion,
		Global: model.ConfigLayer{
			Scope: model.ScopeGlobal,
			MCPs: map[string]model.MCPDefinition{
				"github": {ID: "github", Enabled: serviceBool(true), Command: "github-mcp"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}

	workspacePath := t.TempDir()
	project := model.ConfigLayer{
		Scope: model.ScopeProject,
		MCPs: map[string]model.MCPDefinition{
			"github": {ID: "github", Enabled: serviceBool(false)},
		},
	}
	if err := store.SaveProject(workspacePath, project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	registry := NewRegistry(store)
	registered, err := registry.Add(Input{Path: workspacePath})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	runtime := &model.ConfigLayer{
		Scope: model.ScopeRuntime,
		MCPs: map[string]model.MCPDefinition{
			"github": {ID: "github", Enabled: serviceBool(true), Env: map[string]string{"runtime_only": "yes"}},
		},
	}
	service := NewConfigService(store, registry)
	effective, err := service.ResolveWorkspace(registered.ID, runtime)
	if err != nil {
		t.Fatalf("ResolveWorkspace() error = %v", err)
	}
	assertServiceBool(t, effective.MCPs["github"].Enabled, true)
	if effective.MCPs["github"].EnabledSource != model.ScopeRuntime {
		t.Fatalf("runtime did not win: %+v", effective.MCPs["github"])
	}

	reloadedProject, err := store.LoadProject(workspacePath)
	if err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
	assertServiceBool(t, reloadedProject.MCPs["github"].Enabled, false)
	if _, exists := reloadedProject.MCPs["github"].Env["runtime_only"]; exists {
		t.Fatal("runtime override leaked into persisted project config")
	}
}

func TestConfigServiceMissingReferencedProfileIsExplicitError(t *testing.T) {
	store := config.NewStore(t.TempDir())
	registry := NewRegistry(store)
	registered, err := registry.Add(Input{Path: t.TempDir(), ProfileID: "missing"})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	_, err = NewConfigService(store, registry).ResolveWorkspace(registered.ID, nil)
	if err == nil {
		t.Fatal("ResolveWorkspace() error = nil, want missing profile error")
	}
	var storeErr *config.StoreError
	if !errors.As(err, &storeErr) || storeErr.Kind != config.StoreErrNotFound {
		t.Fatalf("error = %v, want StoreErrNotFound", err)
	}
}

func TestConfigServiceDiscoversGlobalAndProjectSkillsForTwoWorkspaces(t *testing.T) {
	storeRoot := t.TempDir()
	store := config.NewStore(storeRoot)
	globalSkillRoot := filepath.Join(storeRoot, "global-skills")
	globalGSD := createSkillDoc(t, filepath.Join(globalSkillRoot, "gsd"))

	if err := store.SaveUserConfig(config.UserConfig{
		Version: config.SchemaVersion,
		Global: model.ConfigLayer{
			Scope:      model.ScopeGlobal,
			SkillRoots: []string{"global-skills"},
			MCPs: map[string]model.MCPDefinition{
				"github": {ID: "github", Enabled: serviceBool(true), Command: "github-mcp"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}

	registry := NewRegistry(store)
	pathA := t.TempDir()
	pathB := t.TempDir()
	workspaceA, err := registry.Add(Input{Path: pathA})
	if err != nil {
		t.Fatalf("Add A error = %v", err)
	}
	workspaceB, err := registry.Add(Input{Path: pathB})
	if err != nil {
		t.Fatalf("Add B error = %v", err)
	}

	projectGSD := createSkillDoc(t, filepath.Join(pathA, "skills", "gsd"))
	createSkillDoc(t, filepath.Join(pathA, "skills", "private"))
	explicitPrivate := filepath.Join(pathA, "explicit", "private")
	if err := store.SaveProject(pathA, model.ConfigLayer{
		Scope:      model.ScopeProject,
		SkillRoots: []string{"skills"},
		Skills: map[string]model.SkillDefinition{
			"private": {ID: "private", Path: explicitPrivate},
		},
		MCPs: map[string]model.MCPDefinition{
			"github":    {ID: "github", Enabled: serviceBool(false)},
			"private-a": {ID: "private-a", Enabled: serviceBool(true), Command: "private-a-mcp"},
		},
	}); err != nil {
		t.Fatalf("SaveProject(A) error = %v", err)
	}

	service := NewConfigService(store, registry)
	effectiveA, err := service.ResolveWorkspace(workspaceA.ID, nil)
	if err != nil {
		t.Fatalf("ResolveWorkspace(A) error = %v", err)
	}
	effectiveB, err := service.ResolveWorkspace(workspaceB.ID, nil)
	if err != nil {
		t.Fatalf("ResolveWorkspace(B) error = %v", err)
	}

	if effectiveA.Skills["gsd"].Path != projectGSD || effectiveA.Skills["gsd"].Source != model.ScopeProject {
		t.Fatalf("Project A discovered gsd did not override global: %+v", effectiveA.Skills["gsd"])
	}
	if effectiveA.Skills["private"].Path != explicitPrivate || effectiveA.Skills["private"].Source != model.ScopeProject {
		t.Fatalf("explicit project Skill did not override project discovery: %+v", effectiveA.Skills["private"])
	}
	if effectiveB.Skills["gsd"].Path != globalGSD || effectiveB.Skills["gsd"].Source != model.ScopeGlobal {
		t.Fatalf("Workspace B did not inherit global gsd: %+v", effectiveB.Skills["gsd"])
	}
	if _, exists := effectiveB.Skills["private"]; exists {
		t.Fatalf("Project A private Skill leaked into B: %+v", effectiveB.Skills)
	}

	catalogA := mcp.FromEffective(effectiveA)
	catalogB := mcp.FromEffective(effectiveB)
	githubA, ok := catalogA.Get("github")
	if !ok || githubA.Definition.Enabled == nil || *githubA.Definition.Enabled || githubA.EnabledSource != model.ScopeProject {
		t.Fatalf("A catalog did not preserve project disable: %+v", githubA)
	}
	githubB, ok := catalogB.Get("github")
	if !ok || githubB.Definition.Enabled == nil || !*githubB.Definition.Enabled || githubB.EnabledSource != model.ScopeGlobal {
		t.Fatalf("B catalog did not preserve global enable: %+v", githubB)
	}
	if _, ok := catalogA.Get("private-a"); !ok {
		t.Fatal("A catalog missing private MCP")
	}
	if _, ok := catalogB.Get("private-a"); ok {
		t.Fatal("A private MCP leaked into B catalog")
	}

	runtimePath := filepath.Join(pathA, "runtime-skill", "gsd")
	runtime := &model.ConfigLayer{
		Scope: model.ScopeRuntime,
		Skills: map[string]model.SkillDefinition{
			"gsd": {ID: "gsd", Path: runtimePath},
		},
	}
	withRuntime, err := service.ResolveWorkspace(workspaceA.ID, runtime)
	if err != nil {
		t.Fatalf("ResolveWorkspace(A, runtime) error = %v", err)
	}
	if withRuntime.Skills["gsd"].Path != runtimePath || withRuntime.Skills["gsd"].Source != model.ScopeRuntime {
		t.Fatalf("runtime explicit Skill did not win: %+v", withRuntime.Skills["gsd"])
	}
}

func createSkillDoc(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	return filepath.Clean(dir)
}

func serviceBool(value bool) *bool {
	copy := value
	return &copy
}

func assertServiceBool(t *testing.T, got *bool, want bool) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("bool = %v, want %v", got, want)
	}
}
