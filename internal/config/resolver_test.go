package config

import (
	"errors"
	"reflect"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestResolveLayerPrecedenceAndInheritance(t *testing.T) {
	tests := []struct {
		name    string
		global  model.ConfigLayer
		profile *model.ConfigLayer
		project *model.ConfigLayer
		runtime *model.ConfigLayer
		check   func(t *testing.T, got model.EffectiveConfig)
	}{
		{
			name: "global only is inherited unchanged",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{
					"github": {ID: "github", Enabled: boolp(true), Transport: "stdio", Command: "github-mcp", Env: map[string]string{"MODE": "global"}},
				},
				map[string]model.SkillDefinition{
					"gsd": {ID: "gsd", Enabled: boolp(true), Path: "C:/skills/gsd"},
				},
				&model.Policy{Mode: "workspace-write"},
			),
			check: func(t *testing.T, got model.EffectiveConfig) {
				assertBool(t, got.MCPs["github"].Enabled, true)
				if got.MCPs["github"].Source != model.ScopeGlobal || got.MCPs["github"].EnabledSource != model.ScopeGlobal {
					t.Fatalf("unexpected github trace: %+v", got.MCPs["github"])
				}
				if got.Skills["gsd"].Path != "C:/skills/gsd" {
					t.Fatalf("unexpected gsd path: %q", got.Skills["gsd"].Path)
				}
				if got.Policy == nil || got.Policy.Policy.Mode != "workspace-write" || got.Policy.Source != model.ScopeGlobal {
					t.Fatalf("unexpected policy: %+v", got.Policy)
				}
			},
		},
		{
			name:    "profile overrides global policy",
			global:  layer(model.ScopeGlobal, nil, nil, &model.Policy{Mode: "workspace-write"}),
			profile: layerp(model.ScopeProfile, nil, nil, &model.Policy{Mode: "standard"}),
			check: func(t *testing.T, got model.EffectiveConfig) {
				if got.Policy == nil || got.Policy.Policy.Mode != "standard" || got.Policy.Source != model.ScopeProfile {
					t.Fatalf("unexpected policy: %+v", got.Policy)
				}
			},
		},
		{
			name: "project adds private mcp",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{"github": {ID: "github", Command: "github-mcp"}}, nil, nil),
			project: layerp(model.ScopeProject,
				map[string]model.MCPDefinition{"wm-db": {ID: "wm-db", Command: "wm-db-mcp"}}, nil, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				if len(got.MCPs) != 2 || got.MCPs["wm-db"].Source != model.ScopeProject {
					t.Fatalf("unexpected MCPs: %+v", got.MCPs)
				}
			},
		},
		{
			name: "project disables inherited mcp without replacing its body source",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{"github": {ID: "github", Enabled: boolp(true), Command: "github-mcp"}}, nil, nil),
			project: layerp(model.ScopeProject,
				map[string]model.MCPDefinition{"github": {ID: "github", Enabled: boolp(false)}}, nil, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				github := got.MCPs["github"]
				assertBool(t, github.Enabled, false)
				if github.Command != "github-mcp" || github.Source != model.ScopeGlobal || github.EnabledSource != model.ScopeProject {
					t.Fatalf("unexpected github resolution: %+v", github)
				}
			},
		},
		{
			name: "runtime can re-enable mcp disabled by project",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{"github": {ID: "github", Enabled: boolp(true), Command: "github-mcp"}}, nil, nil),
			project: layerp(model.ScopeProject,
				map[string]model.MCPDefinition{"github": {ID: "github", Enabled: boolp(false)}}, nil, nil),
			runtime: layerp(model.ScopeRuntime,
				map[string]model.MCPDefinition{"github": {ID: "github", Enabled: boolp(true)}}, nil, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				github := got.MCPs["github"]
				assertBool(t, github.Enabled, true)
				if github.Source != model.ScopeGlobal || github.EnabledSource != model.ScopeRuntime {
					t.Fatalf("unexpected github trace: %+v", github)
				}
			},
		},
		{
			name: "project overrides global skill path",
			global: layer(model.ScopeGlobal, nil,
				map[string]model.SkillDefinition{"gsd": {ID: "gsd", Enabled: boolp(true), Path: "C:/global/gsd"}}, nil),
			project: layerp(model.ScopeProject, nil,
				map[string]model.SkillDefinition{"gsd": {ID: "gsd", Path: "D:/project/skills/gsd"}}, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				gsd := got.Skills["gsd"]
				if gsd.Path != "D:/project/skills/gsd" || gsd.Source != model.ScopeProject || gsd.EnabledSource != model.ScopeGlobal {
					t.Fatalf("unexpected gsd resolution: %+v", gsd)
				}
			},
		},
		{
			name: "runtime disables project skill",
			global: layer(model.ScopeGlobal, nil,
				map[string]model.SkillDefinition{"gsd": {ID: "gsd", Enabled: boolp(true), Path: "C:/global/gsd"}}, nil),
			project: layerp(model.ScopeProject, nil,
				map[string]model.SkillDefinition{"gsd": {ID: "gsd", Path: "D:/project/gsd"}}, nil),
			runtime: layerp(model.ScopeRuntime, nil,
				map[string]model.SkillDefinition{"gsd": {ID: "gsd", Enabled: boolp(false)}}, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				gsd := got.Skills["gsd"]
				assertBool(t, gsd.Enabled, false)
				if gsd.Source != model.ScopeProject || gsd.EnabledSource != model.ScopeRuntime {
					t.Fatalf("unexpected gsd trace: %+v", gsd)
				}
			},
		},
		{
			name:   "all optional layers may be nil",
			global: layer(model.ScopeGlobal, nil, nil, nil),
			check: func(t *testing.T, got model.EffectiveConfig) {
				if len(got.MCPs) != 0 || len(got.Skills) != 0 || got.Policy != nil {
					t.Fatalf("expected empty effective config, got %+v", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.global, tc.profile, tc.project, tc.runtime)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			tc.check(t, got)
		})
	}
}

func TestResolveRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		global   model.ConfigLayer
		profile  *model.ConfigLayer
		project  *model.ConfigLayer
		runtime  *model.ConfigLayer
		wantKind ErrorKind
	}{
		{
			name:     "global in wrong scope",
			global:   layer(model.ScopeProject, nil, nil, nil),
			wantKind: ErrInvalidScope,
		},
		{
			name:     "profile in wrong scope",
			global:   layer(model.ScopeGlobal, nil, nil, nil),
			profile:  layerp(model.ScopeRuntime, nil, nil, nil),
			wantKind: ErrInvalidScope,
		},
		{
			name: "empty mcp id",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{"bad": {ID: ""}}, nil, nil),
			wantKind: ErrEmptyID,
		},
		{
			name: "empty skill id",
			global: layer(model.ScopeGlobal, nil,
				map[string]model.SkillDefinition{"bad": {ID: ""}}, nil),
			wantKind: ErrEmptyID,
		},
		{
			name: "definition id must match stable map key",
			global: layer(model.ScopeGlobal,
				map[string]model.MCPDefinition{"github": {ID: "other"}}, nil, nil),
			wantKind: ErrIDMismatch,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.global, tc.profile, tc.project, tc.runtime)
			if err == nil {
				t.Fatal("Resolve() error = nil, want error")
			}
			var resolveErr *ResolveError
			if !errors.As(err, &resolveErr) {
				t.Fatalf("error type = %T, want *ResolveError", err)
			}
			if resolveErr.Kind != tc.wantKind {
				t.Fatalf("error kind = %q, want %q", resolveErr.Kind, tc.wantKind)
			}
		})
	}
}

func TestResolveDoesNotMutateOrAliasInputs(t *testing.T) {
	global := layer(model.ScopeGlobal,
		map[string]model.MCPDefinition{
			"github": {
				ID:      "github",
				Enabled: boolp(true),
				Command: "github-mcp",
				Env:     map[string]string{"BASE": "shared", "MODE": "global"},
			},
		},
		map[string]model.SkillDefinition{
			"gsd": {ID: "gsd", Enabled: boolp(true), Path: "C:/global/gsd"},
		},
		&model.Policy{Mode: "workspace-write"},
	)
	project := layerp(model.ScopeProject,
		map[string]model.MCPDefinition{
			"github": {ID: "github", Env: map[string]string{"MODE": "project"}},
		}, nil, nil)

	originalGlobal := deepCopyLayer(global)
	originalProject := deepCopyLayer(*project)

	got, err := Resolve(global, nil, project, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !reflect.DeepEqual(global, originalGlobal) {
		t.Fatalf("global input mutated\n got: %#v\nwant: %#v", global, originalGlobal)
	}
	if !reflect.DeepEqual(*project, originalProject) {
		t.Fatalf("project input mutated\n got: %#v\nwant: %#v", *project, originalProject)
	}

	resolved := got.MCPs["github"]
	resolved.Env["MODE"] = "mutated-output"
	*resolved.Enabled = false
	got.MCPs["github"] = resolved

	if global.MCPs["github"].Env["MODE"] != "global" {
		t.Fatalf("output env aliases global input: %+v", global.MCPs["github"].Env)
	}
	if project.MCPs["github"].Env["MODE"] != "project" {
		t.Fatalf("output env aliases project input: %+v", project.MCPs["github"].Env)
	}
	assertBool(t, global.MCPs["github"].Enabled, true)

	if got.MCPs["github"].Env["BASE"] != "shared" {
		t.Fatalf("higher layer env should merge by key, got %+v", got.MCPs["github"].Env)
	}
}

func TestResolveIsDeterministic(t *testing.T) {
	global := layer(model.ScopeGlobal,
		map[string]model.MCPDefinition{
			"zeta":  {ID: "zeta", Command: "z"},
			"alpha": {ID: "alpha", Command: "a"},
		},
		map[string]model.SkillDefinition{
			"zeta":  {ID: "zeta", Path: "z"},
			"alpha": {ID: "alpha", Path: "a"},
		}, nil)
	project := layerp(model.ScopeProject,
		map[string]model.MCPDefinition{"alpha": {ID: "alpha", Enabled: boolp(false)}},
		map[string]model.SkillDefinition{"zeta": {ID: "zeta", Enabled: boolp(true)}}, nil)

	first, err := Resolve(global, nil, project, nil)
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	for i := 0; i < 25; i++ {
		next, err := Resolve(global, nil, project, nil)
		if err != nil {
			t.Fatalf("Resolve() iteration %d error = %v", i, err)
		}
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("Resolve() is not deterministic\nfirst: %#v\n next: %#v", first, next)
		}
	}
}

func TestResolveMergesAndCopiesEnvRefs(t *testing.T) {
	global := layer(model.ScopeGlobal,
		map[string]model.MCPDefinition{
			"github": {ID: "github", EnvRefs: map[string]string{"SERVICE_CREDENTIAL": "GLOBAL_REFERENCE", "SHARED": "GLOBAL_SHARED"}},
		}, nil, nil)
	project := layerp(model.ScopeProject,
		map[string]model.MCPDefinition{
			"github": {ID: "github", EnvRefs: map[string]string{"SHARED": "PROJECT_SHARED"}},
		}, nil, nil)

	got, err := Resolve(global, nil, project, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	resolved := got.MCPs["github"]
	if resolved.EnvRefs["SERVICE_CREDENTIAL"] != "GLOBAL_REFERENCE" || resolved.EnvRefs["SHARED"] != "PROJECT_SHARED" {
		t.Fatalf("unexpected EnvRefs merge: %+v", resolved.EnvRefs)
	}
	if resolved.Source != model.ScopeProject {
		t.Fatalf("EnvRefs override should update source: %q", resolved.Source)
	}

	resolved.EnvRefs["SERVICE_CREDENTIAL"] = "CHANGED"
	got.MCPs["github"] = resolved
	if global.MCPs["github"].EnvRefs["SERVICE_CREDENTIAL"] != "GLOBAL_REFERENCE" {
		t.Fatalf("resolved EnvRefs aliases input: %+v", global.MCPs["github"].EnvRefs)
	}
}

func TestResolvePolicyDeepCopiesAndHigherLayerWins(t *testing.T) {
	globalPolicy := &model.Policy{
		Mode:               "standard",
		AllowedExecutables: []string{"go"},
		ToolPaths:          map[string]string{"go": `D:\tools\go.exe`},
	}
	projectPolicy := &model.Policy{
		Mode:               "workspace-write",
		AllowedExecutables: []string{"php"},
		ToolPaths:          map[string]string{"php": `D:\tools\php.exe`},
	}
	got, err := Resolve(
		layer(model.ScopeGlobal, nil, nil, globalPolicy),
		nil,
		layerp(model.ScopeProject, nil, nil, projectPolicy),
		nil,
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Policy == nil || got.Policy.Source != model.ScopeProject || got.Policy.Policy.Mode != "workspace-write" {
		t.Fatalf("project policy did not win as a whole: %+v", got.Policy)
	}
	got.Policy.Policy.AllowedExecutables[0] = "changed"
	got.Policy.Policy.ToolPaths["php"] = "changed"
	if projectPolicy.AllowedExecutables[0] != "php" || projectPolicy.ToolPaths["php"] != `D:\tools\php.exe` {
		t.Fatalf("resolved policy aliases project input: %+v", projectPolicy)
	}
}

func TestResolveVerifierPrecedenceDisableAndDeepCopy(t *testing.T) {
	global := layer(model.ScopeGlobal, nil, nil, nil)
	global.Verifiers = map[string]model.VerifierDefinition{
		"test": {
			ID: "test", Kind: "test", Executable: "go", Args: []string{"test", "./..."}, Cwd: ".", TimeoutSeconds: 30,
		},
	}
	project := layerp(model.ScopeProject, nil, nil, nil)
	project.Verifiers = map[string]model.VerifierDefinition{
		"test":  {ID: "test", Args: []string{"test", "./internal/..."}, TimeoutSeconds: 60, Enabled: boolp(false)},
		"build": {ID: "build", Kind: "build", Executable: "go", Args: []string{"build", "./..."}},
	}
	runtimeLayer := layerp(model.ScopeRuntime, nil, nil, nil)
	runtimeLayer.Verifiers = map[string]model.VerifierDefinition{
		"test": {ID: "test", Enabled: boolp(true)},
	}

	got, err := Resolve(global, nil, project, runtimeLayer)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	testVerifier := got.Verifiers["test"]
	if testVerifier.Kind != "test" || testVerifier.Executable != "go" || testVerifier.TimeoutSeconds != 60 {
		t.Fatalf("verifier body merge failed: %+v", testVerifier)
	}
	if !reflect.DeepEqual(testVerifier.Args, []string{"test", "./internal/..."}) || testVerifier.Source != model.ScopeProject {
		t.Fatalf("project verifier override missing: %+v", testVerifier)
	}
	assertBool(t, testVerifier.Enabled, true)
	if testVerifier.EnabledSource != model.ScopeRuntime {
		t.Fatalf("runtime re-enable source = %q, want runtime", testVerifier.EnabledSource)
	}
	if got.Verifiers["build"].Source != model.ScopeProject {
		t.Fatalf("project build verifier missing: %+v", got.Verifiers["build"])
	}

	testVerifier.Args[0] = "changed"
	*testVerifier.Enabled = false
	got.Verifiers["test"] = testVerifier
	if global.Verifiers["test"].Args[0] != "test" || project.Verifiers["test"].Args[0] != "test" || runtimeLayer.Verifiers["test"].Enabled == nil || !*runtimeLayer.Verifiers["test"].Enabled {
		t.Fatal("resolved verifier aliases input layers")
	}
}

func layer(scope model.Scope, mcps map[string]model.MCPDefinition, skills map[string]model.SkillDefinition, policy *model.Policy) model.ConfigLayer {
	return model.ConfigLayer{Scope: scope, MCPs: mcps, Skills: skills, Policy: policy}
}

func layerp(scope model.Scope, mcps map[string]model.MCPDefinition, skills map[string]model.SkillDefinition, policy *model.Policy) *model.ConfigLayer {
	value := layer(scope, mcps, skills, policy)
	return &value
}

func boolp(value bool) *bool {
	copy := value
	return &copy
}

func assertBool(t *testing.T, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("bool pointer = nil, want %v", want)
	}
	if *got != want {
		t.Fatalf("bool = %v, want %v", *got, want)
	}
}

func deepCopyLayer(source model.ConfigLayer) model.ConfigLayer {
	copy := model.ConfigLayer{Scope: source.Scope, SkillRoots: append([]string(nil), source.SkillRoots...)}
	if source.Policy != nil {
		policy := *source.Policy
		policy.AllowedExecutables = append([]string(nil), source.Policy.AllowedExecutables...)
		if source.Policy.ToolPaths != nil {
			policy.ToolPaths = make(map[string]string, len(source.Policy.ToolPaths))
			for key, value := range source.Policy.ToolPaths {
				policy.ToolPaths[key] = value
			}
		}
		copy.Policy = &policy
	}
	if source.MCPs != nil {
		copy.MCPs = make(map[string]model.MCPDefinition, len(source.MCPs))
		for key, value := range source.MCPs {
			mcp := value
			if value.Enabled != nil {
				mcp.Enabled = boolp(*value.Enabled)
			}
			if value.Env != nil {
				mcp.Env = make(map[string]string, len(value.Env))
				for envKey, envValue := range value.Env {
					mcp.Env[envKey] = envValue
				}
			}
			if value.EnvRefs != nil {
				mcp.EnvRefs = make(map[string]string, len(value.EnvRefs))
				for envKey, envValue := range value.EnvRefs {
					mcp.EnvRefs[envKey] = envValue
				}
			}
			copy.MCPs[key] = mcp
		}
	}
	if source.Skills != nil {
		copy.Skills = make(map[string]model.SkillDefinition, len(source.Skills))
		for key, value := range source.Skills {
			skill := value
			if value.Enabled != nil {
				skill.Enabled = boolp(*value.Enabled)
			}
			copy.Skills[key] = skill
		}
	}
	return copy
}
