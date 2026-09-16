package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillSourceRefreshRemovalPreservesUnresolvedEnvironmentSelection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	writeAppSkill(t, root, "shared", "# shared\n")
	service := New(filepath.Join(t.TempDir(), "state.json"))
	source, err := service.Skills.AddSkillSource(root, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil || len(refresh.Skills) != 1 {
		t.Fatalf("refresh=%+v err=%v", refresh, err)
	}
	selectedID := refresh.Skills[0].ID

	workspace, err := service.Workspaces.Add(t.TempDir(), "skill-source")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "skill-source", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(env.EnabledSkillIDs) != 1 || env.EnabledSkillIDs[0] != selectedID {
		t.Fatalf("default source selection not applied to new Environment: %+v", env.EnabledSkillIDs)
	}

	if err := os.Remove(filepath.Join(root, "shared", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	removed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil || removed.Removed != 1 || len(removed.Skills) != 0 {
		t.Fatalf("remove refresh=%+v err=%v", removed, err)
	}
	configured, unresolved, err := service.EnvironmentSkillEntries(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(configured) != 0 || len(unresolved) != 1 || unresolved[0] != selectedID {
		t.Fatalf("removed enabled Skill did not remain an unresolved selection: configured=%+v unresolved=%+v", configured, unresolved)
	}
}

func writeAppSkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
