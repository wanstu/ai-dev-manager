package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewaySkillSourceRemovalCleansEnvironmentSelections(t *testing.T) {
	root := filepath.Join(t.TempDir(), "skills")
	writeGatewaySkill(t, root, "agent-skill", "# agent skill\n")
	support := filepath.Join(t.TempDir(), "support")
	if err := os.MkdirAll(support, 0o755); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()
	owner := newRuntimeOwner(service)
	defer owner.Close()
	session := connectInMemory(t, ctx, newServerForSurface(service, owner, serverSurfaceAdmin))
	defer session.Close()

	added := callGatewayTool(t, ctx, session, "skill_source_add", map[string]any{
		"root":                           root,
		"default_include_in_environment": true,
	})
	if added.IsError || !strings.Contains(toolText(t, added), "skill_source_id") {
		t.Fatalf("skill_source_add failed: %s", toolText(t, added))
	}
	sources, err := service.Skills.ListSkillSources()
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	refreshed := callGatewayTool(t, ctx, session, "skill_source_refresh", map[string]any{"id": sources[0].ID})
	if refreshed.IsError || !strings.Contains(toolText(t, refreshed), `"added":1`) || !strings.Contains(toolText(t, refreshed), "agent-skill") {
		t.Fatalf("skill_source_refresh failed: %s", toolText(t, refreshed))
	}
	listedSources := callGatewayTool(t, ctx, session, "skill_source_list", map[string]any{})
	if listedSources.IsError || !strings.Contains(toolText(t, listedSources), sources[0].ID) {
		t.Fatalf("skill_source_list failed: %s", toolText(t, listedSources))
	}
	updated := callGatewayTool(t, ctx, session, "skill_source_update", map[string]any{
		"id":                             sources[0].ID,
		"root":                           root,
		"support_roots":                  []string{support},
		"default_include_in_environment": true,
	})
	if updated.IsError || !strings.Contains(toolText(t, updated), "skill_source_id") || !strings.Contains(toolText(t, updated), "support") {
		t.Fatalf("skill_source_update failed: %s", toolText(t, updated))
	}
	entries, err := service.Skills.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	if len(entries[0].SupportRoots) != 1 || !strings.Contains(entries[0].SupportRoots[0], "support") {
		t.Fatalf("source update did not propagate support roots to discovered Skill: %+v", entries[0])
	}

	workspace, err := service.Workspaces.Add(t.TempDir(), "gateway-skill-source")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "gateway-skill-source", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(env.EnabledSkillIDs) != 1 || env.EnabledSkillIDs[0] != entries[0].ID {
		t.Fatalf("source default selection missing: %+v", env.EnabledSkillIDs)
	}
	removed := callGatewayTool(t, ctx, session, "skill_source_remove", map[string]any{"id": sources[0].ID})
	if removed.IsError || !strings.Contains(toolText(t, removed), `"removed":1`) {
		t.Fatalf("skill_source_remove failed: %s", toolText(t, removed))
	}
	shown := callGatewayTool(t, ctx, session, "environment_skill_list", map[string]any{"environment_id": env.ID})
	shownText := toolText(t, shown)
	if shown.IsError || strings.Contains(shownText, entries[0].ID) || !strings.Contains(shownText, `"skills":[]`) {
		t.Fatalf("removed Skill selection was not cleaned from Environment: %s", shownText)
	}
}

func writeGatewaySkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
