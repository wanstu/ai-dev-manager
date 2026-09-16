package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentSkillAvailabilityStatesAndIsolation(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "skills")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "skills", "")
	if err != nil {
		t.Fatal(err)
	}

	availableRoot := filepath.Join(t.TempDir(), "available")
	availableSupport := filepath.Join(t.TempDir(), "support")
	writeAvailabilitySkill(t, availableRoot, "healthy", "# healthy\n")
	writeText(t, filepath.Join(availableRoot, "healthy", "guide.md"), "# guide\n")
	writeText(t, filepath.Join(availableSupport, "workflow.md"), "# workflow\n")
	availableSource, err := service.Skills.AddSkillSource(availableRoot, []string{availableSupport}, false)
	if err != nil {
		t.Fatal(err)
	}
	availableRefresh, err := service.Skills.RefreshSkillSource(availableSource.ID)
	if err != nil || len(availableRefresh.Skills) != 1 {
		t.Fatalf("available refresh=%+v err=%v", availableRefresh, err)
	}
	available := availableRefresh.Skills[0]

	disabledRoot := filepath.Join(t.TempDir(), "disabled")
	writeAvailabilitySkill(t, disabledRoot, "disabled", "# disabled\n")
	disabledSource, err := service.Skills.AddSkillSource(disabledRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	disabledRefresh, err := service.Skills.RefreshSkillSource(disabledSource.ID)
	if err != nil || len(disabledRefresh.Skills) != 1 {
		t.Fatalf("disabled refresh=%+v err=%v", disabledRefresh, err)
	}
	disabled := disabledRefresh.Skills[0]

	missingSourceRoot := filepath.Join(t.TempDir(), "source-missing")
	writeAvailabilitySkill(t, missingSourceRoot, "source-missing", "# missing source\n")
	missingSource, err := service.Skills.AddSkillSource(missingSourceRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	missingSourceRefresh, err := service.Skills.RefreshSkillSource(missingSource.ID)
	if err != nil || len(missingSourceRefresh.Skills) != 1 {
		t.Fatalf("missing source refresh=%+v err=%v", missingSourceRefresh, err)
	}
	missingSourceSkill := missingSourceRefresh.Skills[0]

	artifactMissingRoot := filepath.Join(t.TempDir(), "artifact-missing")
	writeAvailabilitySkill(t, artifactMissingRoot, "artifact-missing", "# missing artifact\n")
	artifactMissingSource, err := service.Skills.AddSkillSource(artifactMissingRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	artifactMissingRefresh, err := service.Skills.RefreshSkillSource(artifactMissingSource.ID)
	if err != nil || len(artifactMissingRefresh.Skills) != 1 {
		t.Fatalf("missing artifact refresh=%+v err=%v", artifactMissingRefresh, err)
	}
	artifactMissingSkill := artifactMissingRefresh.Skills[0]

	artifactUnreadableRoot := filepath.Join(t.TempDir(), "artifact-unreadable")
	writeAvailabilitySkill(t, artifactUnreadableRoot, "artifact-unreadable", "# unreadable artifact\n")
	artifactUnreadableSource, err := service.Skills.AddSkillSource(artifactUnreadableRoot, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	artifactUnreadableRefresh, err := service.Skills.RefreshSkillSource(artifactUnreadableSource.ID)
	if err != nil || len(artifactUnreadableRefresh.Skills) != 1 {
		t.Fatalf("unreadable artifact refresh=%+v err=%v", artifactUnreadableRefresh, err)
	}
	artifactUnreadableSkill := artifactUnreadableRefresh.Skills[0]

	supportMissingRoot := filepath.Join(t.TempDir(), "support-missing")
	supportMissingSupport := filepath.Join(t.TempDir(), "support-missing-root")
	writeAvailabilitySkill(t, supportMissingRoot, "support-missing", "# support missing\n")
	writeText(t, filepath.Join(supportMissingSupport, "support.md"), "# support\n")
	supportMissingSource, err := service.Skills.AddSkillSource(supportMissingRoot, []string{supportMissingSupport}, false)
	if err != nil {
		t.Fatal(err)
	}
	supportMissingRefresh, err := service.Skills.RefreshSkillSource(supportMissingSource.ID)
	if err != nil || len(supportMissingRefresh.Skills) != 1 {
		t.Fatalf("support missing refresh=%+v err=%v", supportMissingRefresh, err)
	}
	supportMissingSkill := supportMissingRefresh.Skills[0]

	for _, id := range []string{available.ID, missingSourceSkill.ID, artifactMissingSkill.ID, artifactUnreadableSkill.ID, supportMissingSkill.ID} {
		if _, err := service.SetEnvironmentSkill(environment.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(missingSourceRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifactMissingSkill.ArtifactPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactUnreadableSkill.ArtifactPath, []byte{'#', ' ', 'b', 'a', 'd', 0, '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(supportMissingSupport); err != nil {
		t.Fatal(err)
	}

	assertSkillState(t, service, environment.ID, available.ID, SkillAvailabilityAvailable)
	assertSkillState(t, service, environment.ID, disabled.ID, SkillAvailabilityDisabled)
	assertSkillState(t, service, environment.ID, missingSourceSkill.ID, SkillAvailabilitySourceMissing)
	assertSkillState(t, service, environment.ID, artifactMissingSkill.ID, SkillAvailabilityArtifactMissing)
	assertSkillState(t, service, environment.ID, artifactUnreadableSkill.ID, SkillAvailabilityArtifactUnreadable)
	assertSkillState(t, service, environment.ID, supportMissingSkill.ID, SkillAvailabilitySupportRootMissing)
	if err := os.Remove(disabled.ArtifactPath); err != nil {
		t.Fatal(err)
	}
	disabledBroken, err := service.InspectEnvironmentSkill(environment.ID, disabled.ID)
	if err != nil {
		t.Fatal(err)
	}
	if disabledBroken.Enabled || disabledBroken.State != SkillAvailabilityArtifactMissing {
		t.Fatalf("disabled broken skill should retain enabled=false and report structural failure: %+v", disabledBroken)
	}
	global, err := service.SkillAvailabilities()
	if err != nil {
		t.Fatal(err)
	}
	globalStates := map[string]string{}
	for _, item := range global.Skills {
		if item.EnvironmentID != "" {
			t.Fatalf("global availability must not bind to an Environment: %+v", item)
		}
		globalStates[item.SkillID] = item.State
	}
	if global.Scope != "catalog" || globalStates[disabled.ID] != SkillAvailabilityArtifactMissing || globalStates[artifactMissingSkill.ID] != SkillAvailabilityArtifactMissing {
		t.Fatalf("global availability should expose catalog structural failures independent of Environment: scope=%q states=%+v", global.Scope, globalStates)
	}

	removed, err := service.Skills.RemoveSkillSource(availableSource.ID)
	if err != nil || removed.Removed != 1 {
		t.Fatalf("remove source=%+v err=%v", removed, err)
	}
	// Source deletion clears ordinary Environment selections. Seed a stale
	// reference directly so unresolved inspection remains covered.
	if _, err := service.Environments.SetSkill(environment.ID, available.ID, true); err != nil {
		t.Fatal(err)
	}
	assertSkillState(t, service, environment.ID, available.ID, SkillAvailabilityUnresolved)

	list, err := service.EnvironmentSkillAvailabilities(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, item := range list.Skills {
		states[item.SkillID] = item.State
	}
	if states[disabled.ID] != SkillAvailabilityArtifactMissing || states[artifactMissingSkill.ID] != SkillAvailabilityArtifactMissing {
		t.Fatalf("list did not expose structural failures for disabled/enabled Skills: %+v", list.Skills)
	}
}

func TestEnvironmentSkillFilesAndStructuredReadFailures(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "skills")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "skills", "")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "skills")
	support := filepath.Join(t.TempDir(), "support")
	writeAvailabilitySkill(t, root, "reader", "# reader\n")
	writeText(t, filepath.Join(root, "reader", "notes.md"), "# notes\n")
	writeText(t, filepath.Join(support, "workflow.md"), "# workflow\n")
	source, err := service.Skills.AddSkillSource(root, []string{support}, false)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil || len(refresh.Skills) != 1 {
		t.Fatalf("refresh=%+v err=%v", refresh, err)
	}
	skillID := refresh.Skills[0].ID
	if _, err := service.SetEnvironmentSkill(environment.ID, skillID, true); err != nil {
		t.Fatal(err)
	}

	artifactFiles, err := service.EnvironmentSkillFiles(environment.ID, skillID, "artifact", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !inventoryHas(artifactFiles, "SKILL.md") || !inventoryHas(artifactFiles, "notes.md") {
		t.Fatalf("artifact inventory missing files: %+v", artifactFiles.Files)
	}
	supportFiles, err := service.EnvironmentSkillFiles(environment.ID, skillID, "support", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !inventoryHas(supportFiles, "workflow.md") {
		t.Fatalf("support inventory missing files: %+v", supportFiles.Files)
	}

	if _, err := service.ReadEnvironmentSkill(environment.ID, skillID, filepath.Join(support, "workflow.md"), 0); err != nil {
		t.Fatalf("support read failed: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeText(t, outside, "# outside\n")
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, outside, 0)
	var skillErr *SkillError
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "outside_allowed_roots" {
		t.Fatalf("outside read error=%v structured=%+v", err, skillErr)
	}
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, "missing.md", 0)
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "file_missing" {
		t.Fatalf("missing read error=%v structured=%+v", err, skillErr)
	}
	if err := os.Remove(filepath.Join(root, "reader", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, "", 0)
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "artifact_missing" {
		t.Fatalf("artifact missing read error=%v structured=%+v", err, skillErr)
	}
	writeAvailabilitySkill(t, root, "reader", "# reader\n")
	if err := os.RemoveAll(support); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, "notes.md", 0)
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "support_root_missing" {
		t.Fatalf("support root missing read error=%v structured=%+v", err, skillErr)
	}
	writeText(t, filepath.Join(support, "workflow.md"), "# workflow\n")
	binary := filepath.Join(root, "reader", "binary.bin")
	if err := os.WriteFile(binary, []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, "binary.bin", 0)
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "binary_file" {
		t.Fatalf("binary read error=%v structured=%+v", err, skillErr)
	}
	_, err = service.ReadEnvironmentSkill(environment.ID, skillID, "notes.md", 2)
	if !errors.As(err, &skillErr) || skillErr.ErrorKind != "file_oversize" {
		t.Fatalf("oversize read error=%v structured=%+v", err, skillErr)
	}
}

func assertSkillState(t *testing.T, service *Service, environmentID, skillID, want string) {
	t.Helper()
	status, err := service.InspectEnvironmentSkill(environmentID, skillID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != want {
		t.Fatalf("skill %s state=%q want %q status=%+v", skillID, status.State, want, status)
	}
}

func writeAvailabilitySkill(t *testing.T, root, name, content string) {
	t.Helper()
	writeText(t, filepath.Join(root, name, "SKILL.md"), content)
}

func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func inventoryHas(inventory SkillFileInventory, suffix string) bool {
	for _, item := range inventory.Files {
		if item.Readable && strings.EqualFold(item.Path, suffix) {
			return true
		}
	}
	return false
}
