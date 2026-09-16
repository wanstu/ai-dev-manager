package management_test

import (
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/management"
)

func TestManagementSkillAvailabilityDelegatesToApplicationBoundary(t *testing.T) {
	application := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := application.Workspaces.Add(t.TempDir(), "management-skill")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := application.Environments.Create(workspace.ID, "management-skill", "")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "skills")
	support := filepath.Join(t.TempDir(), "support")
	writeManagementText(t, filepath.Join(root, "reader", "SKILL.md"), "# reader\n")
	writeManagementText(t, filepath.Join(root, "reader", "notes.md"), "# notes\n")
	writeManagementText(t, filepath.Join(support, "workflow.md"), "# workflow\n")
	source, err := application.Skills.AddSkillSource(root, []string{support}, false)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := application.Skills.RefreshSkillSource(source.ID)
	if err != nil || len(refreshed.Skills) != 1 {
		t.Fatalf("refresh=%+v err=%v", refreshed, err)
	}
	skillID := refreshed.Skills[0].ID
	if _, err := application.SetEnvironmentSkill(environment.ID, skillID, true); err != nil {
		t.Fatal(err)
	}

	service := management.New(application)
	global, err := service.SkillAvailabilityList()
	if err != nil || global.Scope != "catalog" || len(global.Skills) != 1 || global.Skills[0].EnvironmentID != "" || global.Skills[0].State != app.SkillAvailabilityAvailable {
		t.Fatalf("SkillAvailabilityList=%+v err=%v", global, err)
	}
	listed, err := service.EnvironmentSkillList(environment.ID)
	if err != nil || len(listed.Skills) != 1 || listed.Skills[0].State != app.SkillAvailabilityAvailable {
		t.Fatalf("EnvironmentSkillList=%+v err=%v", listed, err)
	}
	inspected, err := service.EnvironmentSkillInspect(environment.ID, skillID)
	if err != nil || inspected.SourceID != source.ID || inspected.State != app.SkillAvailabilityAvailable {
		t.Fatalf("EnvironmentSkillInspect=%+v err=%v", inspected, err)
	}
	files, err := service.EnvironmentSkillFiles(environment.ID, skillID, "support", 10)
	if err != nil || len(files.Files) != 1 || files.Files[0].Path != "workflow.md" {
		t.Fatalf("EnvironmentSkillFiles=%+v err=%v", files, err)
	}
}

func writeManagementText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
