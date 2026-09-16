package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestSkillSourceCLIRegistersRefreshesListsAndRemoves(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	root := filepath.Join(t.TempDir(), "skills")
	writeCLISkill(t, root, "cli-skill", "# cli skill\n")

	added := captureStdout(t, func() {
		if err := runCatalog("skill", service, service.Skills, []string{"source-add", "--root", root, "--default"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(added, "skill_source_id") || !strings.Contains(added, "last_refresh_status") {
		t.Fatalf("source-add output = %s", added)
	}
	sources, err := service.Skills.ListSkillSources()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}

	refreshed := captureStdout(t, func() {
		if err := runCatalog("skill", service, service.Skills, []string{"source-refresh", "--id", sources[0].ID}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(refreshed, `"added": 1`) || !strings.Contains(refreshed, "cli-skill") {
		t.Fatalf("source-refresh output = %s", refreshed)
	}
	entries, err := service.Skills.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}

	workspace, err := service.Workspaces.Add(t.TempDir(), "cli-skill")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "cli-skill", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(env.EnabledSkillIDs) != 1 || env.EnabledSkillIDs[0] != entries[0].ID {
		t.Fatalf("default source selection not applied: %+v", env.EnabledSkillIDs)
	}

	listed := captureStdout(t, func() {
		if err := runCatalog("skill", service, service.Skills, []string{"source-list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(listed, sources[0].ID) {
		t.Fatalf("source-list output = %s", listed)
	}

	removed := captureStdout(t, func() {
		if err := runCatalog("skill", service, service.Skills, []string{"source-remove", "--id", sources[0].ID}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(removed, `"removed": 1`) {
		t.Fatalf("source-remove output = %s", removed)
	}
	entries, err = service.Skills.List()
	if err != nil || len(entries) != 0 {
		t.Fatalf("removed source left catalog entries=%+v err=%v", entries, err)
	}
	configured, unresolved, err := service.EnvironmentSkillEntries(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	storedEnv, err := service.Environments.Get(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(configured) != 0 || len(unresolved) != 0 || len(storedEnv.EnabledSkillIDs) != 0 {
		t.Fatalf("source remove should clean Environment selection: configured=%+v unresolved=%+v env=%+v", configured, unresolved, storedEnv.EnabledSkillIDs)
	}
}

func writeCLISkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
