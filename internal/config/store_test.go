package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestStoreMissingUserConfigReturnsDefaultWithoutWriting(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	cfg, err := store.LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	if cfg.Version != SchemaVersion || cfg.Global.Scope != model.ScopeGlobal {
		t.Fatalf("unexpected default config: %+v", cfg)
	}
	if len(cfg.Workspaces) != 0 || len(cfg.Global.MCPs) != 0 || len(cfg.Global.Skills) != 0 {
		t.Fatalf("default config should be empty: %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(root, userConfigFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("load should not create config file, stat error = %v", err)
	}
}

func TestStoreUserConfigRoundTripAndOverwrite(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	cfg := UserConfig{
		Version: SchemaVersion,
		Global: model.ConfigLayer{
			Scope: model.ScopeGlobal,
			MCPs: map[string]model.MCPDefinition{
				"github": {
					ID: "github", Enabled: storeBool(true), Transport: "stdio", Command: "github-mcp",
					Env: map[string]string{"MODE": "共享"}, EnvRefs: map[string]string{"SERVICE_CREDENTIAL": "REFERENCE_NAME"},
				},
			},
			Skills: map[string]model.SkillDefinition{
				"gsd": {ID: "gsd", Enabled: storeBool(true), Path: `D:\工具\skills\gsd`},
			},
			SkillRoots: []string{`D:\工具\skills`},
			Policy: &model.Policy{
				Mode:               "workspace-write",
				AllowedExecutables: []string{"go", "php"},
				ToolPaths:          map[string]string{"go": `D:\工具\go.exe`},
			},
		},
		Workspaces: []model.Workspace{
			{ID: "ws_1", Path: `D:\项目\示例`, ProfileID: "work", RuntimeID: "native"},
		},
	}

	if err := store.SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	loaded, err := store.LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, loaded) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", loaded, cfg)
	}

	cfg.Global.Policy = &model.Policy{Mode: "standard"}
	cfg.Workspaces[0].RuntimeID = "devspace"
	if err := store.SaveUserConfig(cfg); err != nil {
		t.Fatalf("second SaveUserConfig() error = %v", err)
	}
	loaded, err = store.LoadUserConfig()
	if err != nil {
		t.Fatalf("second LoadUserConfig() error = %v", err)
	}
	if loaded.Global.Policy == nil || loaded.Global.Policy.Mode != "standard" || loaded.Workspaces[0].RuntimeID != "devspace" {
		t.Fatalf("existing config was not replaced correctly: %+v", loaded)
	}
}

func TestStoreReturnsStructuredErrorsForMalformedAndUnsupportedConfig(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantKind StoreErrorKind
	}{
		{name: "malformed json", content: `{"version":`, wantKind: StoreErrDecode},
		{name: "newer schema", content: `{"version":99,"global":{"mcps":{},"skills":{},"policy":null},"workspaces":[]}`, wantKind: StoreErrUnsupportedVersion},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, userConfigFilename)
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			_, err := NewStore(root).LoadUserConfig()
			assertStoreErrorKind(t, err, tc.wantKind)

			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile() after failed load error = %v", readErr)
			}
			if string(after) != tc.content {
				t.Fatalf("failed load must not modify original file: %q", after)
			}
		})
	}
}

func TestStoreProfileAndProjectLayers(t *testing.T) {
	store := NewStore(t.TempDir())
	profile := model.ConfigLayer{
		Scope: model.ScopeProfile,
		MCPs: map[string]model.MCPDefinition{
			"profile-db": {ID: "profile-db", Command: "profile-db-mcp"},
		},
		Skills: map[string]model.SkillDefinition{
			"review": {ID: "review", Path: `D:\skills\review`},
		},
		Policy: &model.Policy{Mode: "standard"},
	}
	if err := store.SaveProfile("work", profile); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}
	loadedProfile, err := store.LoadProfile("work")
	if err != nil {
		t.Fatalf("LoadProfile() error = %v", err)
	}
	if !reflect.DeepEqual(profile, loadedProfile) {
		t.Fatalf("profile round-trip mismatch\n got: %#v\nwant: %#v", loadedProfile, profile)
	}

	workspacePath := t.TempDir()
	missingProject, err := store.LoadProject(workspacePath)
	if err != nil {
		t.Fatalf("missing LoadProject() error = %v", err)
	}
	if missingProject.Scope != model.ScopeProject || len(missingProject.MCPs) != 0 || len(missingProject.Skills) != 0 {
		t.Fatalf("missing project should be empty project layer: %+v", missingProject)
	}

	project := model.ConfigLayer{
		Scope: model.ScopeProject,
		MCPs: map[string]model.MCPDefinition{
			"github": {ID: "github", Enabled: storeBool(false)},
		},
		Skills: map[string]model.SkillDefinition{
			"gsd": {ID: "gsd", Path: filepath.Join(workspacePath, "skills", "gsd")},
		},
	}
	if err := store.SaveProject(workspacePath, project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	loadedProject, err := store.LoadProject(workspacePath)
	if err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
	if !reflect.DeepEqual(project, loadedProject) {
		t.Fatalf("project round-trip mismatch\n got: %#v\nwant: %#v", loadedProject, project)
	}
}

func TestStoreProfileValidationAndMissingReference(t *testing.T) {
	store := NewStore(t.TempDir())

	for _, id := range []string{"", ".", "..", "../work", `work\\child`, "work/child"} {
		t.Run("reject_"+id, func(t *testing.T) {
			_, err := store.LoadProfile(id)
			assertStoreErrorKind(t, err, StoreErrInvalidID)
		})
	}

	_, err := store.LoadProfile("missing")
	assertStoreErrorKind(t, err, StoreErrNotFound)
}

func TestStoreRejectsWrongPersistentScopeBeforeWriting(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	wrong := model.ConfigLayer{Scope: model.ScopeRuntime}

	err := store.SaveProfile("work", wrong)
	assertStoreErrorKind(t, err, StoreErrInvalidScope)
	if _, statErr := os.Stat(filepath.Join(root, profilesDirname, "work.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid layer should not be written, stat error = %v", statErr)
	}
}

func TestStoreLoadsV1WithoutPhase3OptionalFields(t *testing.T) {
	root := t.TempDir()
	content := `{"version":1,"global":{"mcps":{"github":{"id":"github","command":"github-mcp"}},"skills":{},"policy":null},"workspaces":[]}`
	if err := os.WriteFile(filepath.Join(root, userConfigFilename), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := NewStore(root).LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}
	if len(cfg.Global.SkillRoots) != 0 || len(cfg.Global.MCPs["github"].EnvRefs) != 0 {
		t.Fatalf("missing optional fields should decode empty: %+v", cfg.Global)
	}
}

func TestStorePersistsEnvRefsWithoutResolvingHostEnvironment(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	t.Setenv("ADM_REFERENCE_ENV", "host-value-not-for-config")
	cfg := UserConfig{
		Version: SchemaVersion,
		Global: model.ConfigLayer{
			Scope: model.ScopeGlobal,
			MCPs: map[string]model.MCPDefinition{
				"example": {ID: "example", EnvRefs: map[string]string{"SERVICE_CREDENTIAL": "ADM_REFERENCE_ENV"}},
			},
		},
	}
	if err := store.SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig() error = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, userConfigFilename))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "ADM_REFERENCE_ENV") {
		t.Fatalf("saved config missing EnvRef name: %s", text)
	}
	if strings.Contains(text, "host-value-not-for-config") {
		t.Fatal("saved config resolved host environment value instead of persisting only the reference")
	}
}

func TestStoreVerifierRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	workspacePath := t.TempDir()
	project := model.ConfigLayer{
		Scope: model.ScopeProject,
		Verifiers: map[string]model.VerifierDefinition{
			"test": {
				ID: "test", Kind: "test", Enabled: storeBool(true), Executable: "go",
				Args: []string{"test", "./..."}, Cwd: ".", TimeoutSeconds: 45,
			},
			"build": {
				ID: "build", Kind: "build", Executable: "go", Args: []string{"build", "./..."},
			},
		},
	}
	if err := store.SaveProject(workspacePath, project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	loaded, err := store.LoadProject(workspacePath)
	if err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Verifiers, project.Verifiers) {
		t.Fatalf("verifier round-trip mismatch\n got: %#v\nwant: %#v", loaded.Verifiers, project.Verifiers)
	}
}

func TestStoreRejectsInvalidWorkspaceRecordsBeforeWriting(t *testing.T) {
	validPath := t.TempDir()
	tests := []struct {
		name       string
		workspaces []model.Workspace
		wantKind   StoreErrorKind
	}{
		{name: "empty id", workspaces: []model.Workspace{{Path: validPath}}, wantKind: StoreErrInvalidDefinition},
		{name: "relative path", workspaces: []model.Workspace{{ID: "ws_one", Path: "relative/path"}}, wantKind: StoreErrInvalidDefinition},
		{name: "duplicate id", workspaces: []model.Workspace{{ID: "ws_same", Path: validPath}, {ID: "ws_same", Path: t.TempDir()}}, wantKind: StoreErrInvalidDefinition},
		{name: "duplicate windows path", workspaces: []model.Workspace{{ID: "ws_one", Path: validPath}, {ID: "ws_two", Path: strings.ToUpper(validPath)}}, wantKind: StoreErrInvalidDefinition},
		{name: "invalid profile id", workspaces: []model.Workspace{{ID: "ws_one", Path: validPath, ProfileID: "../bad"}}, wantKind: StoreErrInvalidID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := NewStore(root)
			err := store.SaveUserConfig(UserConfig{
				Version:    SchemaVersion,
				Global:     model.ConfigLayer{Scope: model.ScopeGlobal},
				Workspaces: tc.workspaces,
			})
			assertStoreErrorKind(t, err, tc.wantKind)
			if _, statErr := os.Stat(filepath.Join(root, userConfigFilename)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("invalid user config should not be written, stat error = %v", statErr)
			}
		})
	}
}

func assertStoreErrorKind(t *testing.T, err error, want StoreErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want StoreError kind %q", want)
	}
	var storeErr *StoreError
	if !errors.As(err, &storeErr) {
		t.Fatalf("error type = %T, want *StoreError", err)
	}
	if storeErr.Kind != want {
		t.Fatalf("StoreError.Kind = %q, want %q (error: %v)", storeErr.Kind, want, err)
	}
}

func storeBool(value bool) *bool {
	copy := value
	return &copy
}
