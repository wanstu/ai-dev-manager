package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/model"
)

func TestEnvironmentInjectionPlanSelectionSourcesAndInjectability(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "state.json"))
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "inject")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "inject", "")
	if err != nil {
		t.Fatal(err)
	}

	explicitMCP, err := service.MCPs.AddMCP("explicit-http", "http://127.0.0.1:18080/mcp", true)
	if err != nil {
		t.Fatal(err)
	}
	inheritedMCP, err := service.MCPs.AddMCP("workspace-http", "http://127.0.0.1:18081/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	blockedMCP, err := service.MCPs.AddMCP("blocked-http", "http://${INJECTION_PLAN_MISSING}/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, explicitMCP.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceMCP(workspace.ID, inheritedMCP.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, blockedMCP.ID, true); err != nil {
		t.Fatal(err)
	}

	skillRoot := filepath.Join(t.TempDir(), "skills")
	writeAvailabilitySkill(t, skillRoot, "explicit-skill", "# explicit\n")
	writeAvailabilitySkill(t, skillRoot, "workspace-skill", "# workspace\n")
	writeAvailabilitySkill(t, skillRoot, "blocked-skill", "# blocked\n")
	source, err := service.Skills.AddSkillSource(skillRoot, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Skills.RefreshSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	artifactByName := map[string]string{}
	for _, entry := range refreshed.Skills {
		byName[entry.Name] = entry.ID
		artifactByName[entry.Name] = entry.ArtifactPath
	}
	if _, err := service.SetEnvironmentSkill(environment.ID, byName["explicit-skill"], true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceSkill(workspace.ID, byName["workspace-skill"], true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentSkill(environment.ID, byName["blocked-skill"], true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifactByName["blocked-skill"]); err != nil {
		t.Fatal(err)
	}

	plan, err := service.EnvironmentInjectionPlan(context.Background(), environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.SelectedMCPs != 3 || plan.Summary.InjectableMCPs != 2 || plan.Summary.BlockedMCPs != 1 {
		t.Fatalf("MCP summary = %+v", plan.Summary)
	}
	if plan.Summary.SelectedSkills != 3 || plan.Summary.InjectableSkills != 2 || plan.Summary.BlockedSkills != 1 {
		t.Fatalf("Skill summary = %+v", plan.Summary)
	}

	explicitMCPPlan := injectionMCPByID(t, plan, explicitMCP.ID)
	assertInjectionSources(t, explicitMCPPlan.SelectionSources, "environment")
	if !explicitMCPPlan.Injectable || explicitMCPPlan.NextAction != "environment_mcp_tools" {
		t.Fatalf("explicit MCP plan = %+v", explicitMCPPlan)
	}
	if !explicitMCPPlan.DefaultIncludeInEnvironment {
		t.Fatalf("catalog default flag was lost: %+v", explicitMCPPlan)
	}

	inheritedMCPPlan := injectionMCPByID(t, plan, inheritedMCP.ID)
	assertInjectionSources(t, inheritedMCPPlan.SelectionSources, "workspace")
	if !inheritedMCPPlan.Injectable {
		t.Fatalf("workspace MCP should be injectable: %+v", inheritedMCPPlan)
	}

	blockedMCPPlan := injectionMCPByID(t, plan, blockedMCP.ID)
	assertInjectionSources(t, blockedMCPPlan.SelectionSources, "environment")
	if blockedMCPPlan.Injectable || blockedMCPPlan.ReasonCode != "unresolved_secret_reference" || blockedMCPPlan.NextAction != "environment_mcp_inspect" {
		t.Fatalf("blocked MCP plan = %+v", blockedMCPPlan)
	}

	explicitSkillPlan := injectionSkillByID(t, plan, byName["explicit-skill"])
	assertInjectionSources(t, explicitSkillPlan.SelectionSources, "environment")
	if !explicitSkillPlan.Injectable || explicitSkillPlan.NextAction != "environment_skill_read" || !explicitSkillPlan.DefaultIncludeInEnvironment {
		t.Fatalf("explicit Skill plan = %+v", explicitSkillPlan)
	}

	inheritedSkillPlan := injectionSkillByID(t, plan, byName["workspace-skill"])
	assertInjectionSources(t, inheritedSkillPlan.SelectionSources, "workspace")
	if !inheritedSkillPlan.Injectable {
		t.Fatalf("workspace Skill should be injectable: %+v", inheritedSkillPlan)
	}

	blockedSkillPlan := injectionSkillByID(t, plan, byName["blocked-skill"])
	assertInjectionSources(t, blockedSkillPlan.SelectionSources, "environment")
	if blockedSkillPlan.Injectable || blockedSkillPlan.NextAction != "environment_skill_inspect" {
		t.Fatalf("blocked Skill plan = %+v", blockedSkillPlan)
	}

	bundle, err := service.EnvironmentContextBundle(context.Background(), environment.ID, model.EnvironmentContextRequest{})
	if err != nil {
		t.Fatal(err)
	}
	contextMCP := contextMCPByID(t, bundle, inheritedMCP.ID)
	if !contextMCP.Selected || !contextMCP.Injectable || contextMCP.NextAction != inheritedMCPPlan.NextAction {
		t.Fatalf("context MCP did not reuse injection plan: %+v", contextMCP)
	}
	assertInjectionSources(t, contextMCP.SelectionSources, "workspace")

	contextSkill := contextSkillByID(t, bundle, byName["blocked-skill"])
	if !contextSkill.Selected || contextSkill.Injectable || contextSkill.NextAction != "environment_skill_inspect" {
		t.Fatalf("context Skill did not reuse injection plan: %+v", contextSkill)
	}
}

func injectionMCPByID(t *testing.T, plan model.EnvironmentInjectionPlan, id string) model.EnvironmentInjectionMCP {
	t.Helper()
	for _, item := range plan.MCPs {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("MCP %s not found in plan", id)
	return model.EnvironmentInjectionMCP{}
}

func injectionSkillByID(t *testing.T, plan model.EnvironmentInjectionPlan, id string) model.EnvironmentInjectionSkill {
	t.Helper()
	for _, item := range plan.Skills {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Skill %s not found in plan", id)
	return model.EnvironmentInjectionSkill{}
}

func contextMCPByID(t *testing.T, bundle model.EnvironmentContextBundle, id string) model.EnvironmentContextMCP {
	t.Helper()
	for _, item := range bundle.MCPs {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("MCP %s not found in context bundle", id)
	return model.EnvironmentContextMCP{}
}

func contextSkillByID(t *testing.T, bundle model.EnvironmentContextBundle, id string) model.EnvironmentContextSkill {
	t.Helper()
	for _, item := range bundle.Skills {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Skill %s not found in context bundle", id)
	return model.EnvironmentContextSkill{}
}

func assertInjectionSources(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("selection_sources=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selection_sources=%v want=%v", got, want)
		}
	}
}
