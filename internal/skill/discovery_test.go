package skill

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestExpandLayerDiscoversNestedSkillsAndResolvesRelativeRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "shared-skills")
	gsdDir := createSkill(t, filepath.Join(root, "gsd"))
	reviewDir := createSkill(t, filepath.Join(root, "nested", "review"))

	layer := model.ConfigLayer{
		Scope:      model.ScopeGlobal,
		SkillRoots: []string{"shared-skills"},
	}
	got, err := ExpandLayer(layer, base)
	if err != nil {
		t.Fatalf("ExpandLayer() error = %v", err)
	}
	if got.Skills["gsd"].Path != gsdDir {
		t.Fatalf("gsd path = %q, want %q", got.Skills["gsd"].Path, gsdDir)
	}
	if got.Skills["review"].Path != reviewDir {
		t.Fatalf("review path = %q, want %q", got.Skills["review"].Path, reviewDir)
	}
	if got.Scope != model.ScopeGlobal {
		t.Fatalf("scope = %q, want global", got.Scope)
	}
}

func TestExpandLayerLaterRootAndExplicitSkillPrecedence(t *testing.T) {
	base := t.TempDir()
	firstRoot := filepath.Join(base, "first")
	secondRoot := filepath.Join(base, "second")
	firstGSD := createSkill(t, filepath.Join(firstRoot, "gsd"))
	secondGSD := createSkill(t, filepath.Join(secondRoot, "gsd"))
	explicitPath := filepath.Join(base, "explicit", "gsd")

	withoutExplicit, err := ExpandLayer(model.ConfigLayer{
		Scope:      model.ScopeGlobal,
		SkillRoots: []string{firstRoot, secondRoot},
	}, base)
	if err != nil {
		t.Fatalf("ExpandLayer() without explicit error = %v", err)
	}
	if withoutExplicit.Skills["gsd"].Path != secondGSD {
		t.Fatalf("later root should win: got %q want %q (first=%q)", withoutExplicit.Skills["gsd"].Path, secondGSD, firstGSD)
	}

	withExplicit, err := ExpandLayer(model.ConfigLayer{
		Scope:      model.ScopeGlobal,
		SkillRoots: []string{firstRoot, secondRoot},
		Skills: map[string]model.SkillDefinition{
			"gsd": {ID: "gsd", Path: explicitPath, Enabled: skillBool(true)},
		},
	}, base)
	if err != nil {
		t.Fatalf("ExpandLayer() with explicit error = %v", err)
	}
	if withExplicit.Skills["gsd"].Path != explicitPath {
		t.Fatalf("explicit skill should win: got %q want %q", withExplicit.Skills["gsd"].Path, explicitPath)
	}
	if withExplicit.Skills["gsd"].Enabled == nil || !*withExplicit.Skills["gsd"].Enabled {
		t.Fatalf("explicit Enabled decision lost: %+v", withExplicit.Skills["gsd"])
	}
}

func TestExpandLayerDoesNotMutateOrAliasInput(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "discovered"))
	layer := model.ConfigLayer{
		Scope:      model.ScopeProject,
		SkillRoots: []string{root},
		MCPs: map[string]model.MCPDefinition{
			"github": {
				ID:      "github",
				Enabled: skillBool(true),
				Env:     map[string]string{"MODE": "project"},
				EnvRefs: map[string]string{"SERVICE_CREDENTIAL": "REFERENCE_NAME"},
			},
		},
		Skills: map[string]model.SkillDefinition{
			"explicit": {ID: "explicit", Path: filepath.Join(root, "explicit"), Enabled: skillBool(true)},
		},
		Policy: &model.Policy{Mode: "workspace-write"},
	}
	before := cloneTestLayer(layer)

	got, err := ExpandLayer(layer, root)
	if err != nil {
		t.Fatalf("ExpandLayer() error = %v", err)
	}
	if !reflect.DeepEqual(layer, before) {
		t.Fatalf("input mutated\n got: %#v\nwant: %#v", layer, before)
	}

	mcp := got.MCPs["github"]
	mcp.Env["MODE"] = "changed"
	mcp.EnvRefs["SERVICE_CREDENTIAL"] = "CHANGED_REFERENCE"
	*mcp.Enabled = false
	got.MCPs["github"] = mcp
	got.SkillRoots[0] = "changed-root"

	if layer.MCPs["github"].Env["MODE"] != "project" || layer.MCPs["github"].EnvRefs["SERVICE_CREDENTIAL"] != "REFERENCE_NAME" {
		t.Fatalf("expanded layer aliases input MCP maps: %+v", layer.MCPs["github"])
	}
	if layer.MCPs["github"].Enabled == nil || !*layer.MCPs["github"].Enabled {
		t.Fatal("expanded layer aliases input Enabled pointer")
	}
	if layer.SkillRoots[0] != root {
		t.Fatal("expanded layer aliases input SkillRoots")
	}
}

func TestExpandLayerMissingAndInvalidRootsReturnStructuredErrors(t *testing.T) {
	base := t.TempDir()
	fileRoot := filepath.Join(base, "file-root")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name string
		root string
		kind ErrorKind
	}{
		{name: "empty", root: "", kind: ErrInvalidRoot},
		{name: "missing", root: filepath.Join(base, "missing"), kind: ErrRootMissing},
		{name: "file", root: fileRoot, kind: ErrInvalidRoot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExpandLayer(model.ConfigLayer{Scope: model.ScopeGlobal, SkillRoots: []string{tc.root}}, base)
			if err == nil {
				t.Fatalf("error = nil, want %q", tc.kind)
			}
			var discoveryErr *DiscoveryError
			if !errors.As(err, &discoveryErr) || discoveryErr.Kind != tc.kind {
				t.Fatalf("error = %v, want DiscoveryError kind %q", err, tc.kind)
			}
		})
	}
}

func TestResolveRootExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	got, err := resolveRoot("~/skills", t.TempDir())
	if err != nil {
		t.Fatalf("resolveRoot() error = %v", err)
	}
	want := filepath.Clean(filepath.Join(home, "skills"))
	if got != want {
		t.Fatalf("resolveRoot() = %q, want %q", got, want)
	}
}

func createSkill(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	return filepath.Clean(dir)
}

func skillBool(value bool) *bool {
	copy := value
	return &copy
}

func cloneTestLayer(source model.ConfigLayer) model.ConfigLayer {
	clone := model.ConfigLayer{
		Scope:      source.Scope,
		MCPs:       make(map[string]model.MCPDefinition, len(source.MCPs)),
		Skills:     make(map[string]model.SkillDefinition, len(source.Skills)),
		SkillRoots: append([]string(nil), source.SkillRoots...),
	}
	if source.Policy != nil {
		clone.Policy = &model.Policy{Mode: source.Policy.Mode}
	}
	for id, definition := range source.MCPs {
		mcp := definition
		mcp.Enabled = skillBoolPtr(definition.Enabled)
		mcp.Env = copyStringMap(definition.Env)
		mcp.EnvRefs = copyStringMap(definition.EnvRefs)
		clone.MCPs[id] = mcp
	}
	for id, definition := range source.Skills {
		skill := definition
		skill.Enabled = skillBoolPtr(definition.Enabled)
		clone.Skills[id] = skill
	}
	return clone
}

func skillBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	return skillBool(*value)
}

func copyStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
