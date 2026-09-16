package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewaySkillInspectFilesAndReadDiagnostics(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	support := filepath.Join(t.TempDir(), "support")
	writeGatewaySkill(t, root, "reader", "# reader\n")
	writeGatewayText(t, filepath.Join(root, "reader", "notes.md"), "# notes\n")
	writeGatewayText(t, filepath.Join(support, "workflow.md"), "# workflow\n")

	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "gateway-skill-availability")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "gateway-skill-availability", "")
	if err != nil {
		t.Fatal(err)
	}
	source, err := service.Skills.AddSkillSource(root, []string{support}, false)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil || len(refreshed.Skills) != 1 {
		t.Fatalf("refresh=%+v err=%v", refreshed, err)
	}
	skillID := refreshed.Skills[0].ID
	if _, err := service.SetEnvironmentSkill(environment.ID, skillID, true); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, ctx, newServer(service, owner))
	defer session.Close()

	inspected := callGatewayTool(t, ctx, session, "environment_skill_inspect", map[string]any{
		"environment_id": environment.ID,
		"skill_id":       skillID,
	})
	if inspected.IsError || !strings.Contains(toolText(t, inspected), `"state":"available"`) || !strings.Contains(toolText(t, inspected), source.ID) {
		t.Fatalf("environment_skill_inspect failed: %s", toolText(t, inspected))
	}
	artifactFiles := callGatewayTool(t, ctx, session, "environment_skill_files", map[string]any{
		"environment_id": environment.ID,
		"skill_id":       skillID,
		"root_kind":      "artifact",
		"max_entries":    10,
	})
	if artifactFiles.IsError || !strings.Contains(toolText(t, artifactFiles), "SKILL.md") || !strings.Contains(toolText(t, artifactFiles), "notes.md") {
		t.Fatalf("artifact files failed: %s", toolText(t, artifactFiles))
	}
	supportFiles := callGatewayTool(t, ctx, session, "environment_skill_files", map[string]any{
		"environment_id": environment.ID,
		"skill_id":       skillID,
		"root_kind":      "support",
		"max_entries":    10,
	})
	if supportFiles.IsError || !strings.Contains(toolText(t, supportFiles), "workflow.md") {
		t.Fatalf("support files failed: %s", toolText(t, supportFiles))
	}
	readSupport := callGatewayTool(t, ctx, session, "environment_skill_read", map[string]any{
		"environment_id": environment.ID,
		"skill_id":       skillID,
		"path":           filepath.Join(support, "workflow.md"),
	})
	if readSupport.IsError || !strings.Contains(toolText(t, readSupport), "# workflow") {
		t.Fatalf("support read failed: %s", toolText(t, readSupport))
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeGatewayText(t, outside, "# outside\n")
	readOutside := callGatewayTool(t, ctx, session, "environment_skill_read", map[string]any{
		"environment_id": environment.ID,
		"skill_id":       skillID,
		"path":           outside,
	})
	if !readOutside.IsError || !strings.Contains(toolText(t, readOutside), "error_kind=outside_allowed_roots") {
		t.Fatalf("outside read was not structured: %s", toolText(t, readOutside))
	}
}

func writeGatewayText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
