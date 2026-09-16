package catalog_test

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/catalog"
	"ai-dev-manager-v2/internal/store"
)

func TestSkillSourcesAllowSameNameSkillsAndStableArtifactIdentity(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	skills := catalog.New(state, catalog.KindSkill)
	rootA := writeSkillFixture(t, "shared", "# shared A\n")
	rootB := writeSkillFixture(t, "shared", "# shared B\n")

	sourceA, err := skills.AddSkillSource(rootA, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	refreshA, err := skills.RefreshSkillSource(sourceA.ID)
	if err != nil {
		t.Fatal(err)
	}
	sourceB, err := skills.AddSkillSource(rootB, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	refreshB, err := skills.RefreshSkillSource(sourceB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshA.Skills[0].Name != refreshB.Skills[0].Name || refreshA.Skills[0].ID == refreshB.Skills[0].ID {
		t.Fatalf("same-name source identities not preserved: a=%+v b=%+v", refreshA.Skills, refreshB.Skills)
	}
	if !refreshA.Skills[0].DefaultIncludeInEnv || refreshB.Skills[0].DefaultIncludeInEnv {
		t.Fatalf("source default_include was not applied: a=%+v b=%+v", refreshA.Skills[0], refreshB.Skills[0])
	}

	again, err := skills.RefreshSkillSource(sourceA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Added != 0 || again.Removed != 0 || again.Skills[0].ID != refreshA.Skills[0].ID {
		t.Fatalf("same artifact path did not preserve identity on refresh: %+v", again)
	}
}

func TestSkillSourceRefreshAddUpdateRemoveAndFailurePreservesSnapshot(t *testing.T) {
	state := store.New(filepath.Join(t.TempDir(), "state.json"))
	skills := catalog.New(state, catalog.KindSkill)
	root := writeSkillFixture(t, "alpha", "# alpha\n")
	source, err := skills.AddSkillSource(root, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := skills.RefreshSkillSource(source.ID)
	if err != nil || initial.Added != 1 || len(initial.Skills) != 1 {
		t.Fatalf("initial refresh=%+v err=%v", initial, err)
	}
	alphaID := initial.Skills[0].ID

	writeSkillFile(t, root, "beta", "# beta\n")
	added, err := skills.RefreshSkillSource(source.ID)
	if err != nil || added.Added != 1 || added.Removed != 0 || len(added.Skills) != 2 {
		t.Fatalf("add refresh=%+v err=%v", added, err)
	}

	if err := os.Remove(filepath.Join(root, "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	removed, err := skills.RefreshSkillSource(source.ID)
	if err != nil || removed.Removed != 1 || len(removed.Skills) != 1 {
		t.Fatalf("remove refresh=%+v err=%v", removed, err)
	}
	for _, entry := range removed.Skills {
		if entry.ID == alphaID {
			t.Fatalf("removed artifact identity survived successful refresh: %+v", removed.Skills)
		}
	}

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := skills.RefreshSkillSource(source.ID); err == nil {
		t.Fatal("missing source root refresh unexpectedly succeeded")
	}
	persisted, err := skills.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].Name != "beta" {
		t.Fatalf("failed refresh mutated previous snapshot: %+v", persisted)
	}
	sources, err := skills.ListSkillSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].LastRefreshStatus != "error" || sources[0].LastRefreshError == "" {
		t.Fatalf("failed refresh status not persisted: %+v", sources)
	}
}

func writeSkillFixture(t *testing.T, name, content string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "skills")
	writeSkillFile(t, root, name, content)
	return root
}

func writeSkillFile(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
