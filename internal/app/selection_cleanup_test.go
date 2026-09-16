package app_test

import (
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
)

func TestDisablingMissingEnvironmentSelectionsIsAllowed(t *testing.T) {
	service := app.New(t.TempDir() + "/state.json")
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "environment", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.SetMCP(environment.ID, "mcp-missing", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.SetSkill(environment.ID, "skill-missing", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Workspaces.SetMCP(workspace.ID, "workspace-mcp-missing", true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Workspaces.SetSkill(workspace.ID, "workspace-skill-missing", true); err != nil {
		t.Fatal(err)
	}

	if _, err := service.SetWorkspaceMCP(workspace.ID, "workspace-mcp-missing", false); err != nil {
		t.Fatalf("disable missing Workspace MCP selection: %v", err)
	}
	if _, err := service.SetWorkspaceSkill(workspace.ID, "workspace-skill-missing", false); err != nil {
		t.Fatalf("disable missing Workspace Skill selection: %v", err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, "mcp-missing", false); err != nil {
		t.Fatalf("disable missing MCP selection: %v", err)
	}
	if _, err := service.SetEnvironmentSkill(environment.ID, "skill-missing", false); err != nil {
		t.Fatalf("disable missing Skill selection: %v", err)
	}

	updated, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.EnabledMCPIDs) != 0 || len(updated.EnabledSkillIDs) != 0 {
		t.Fatalf("missing selections not cleared: %+v", updated)
	}
	updatedWorkspace, err := service.Workspaces.Get(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedWorkspace.EnabledMCPIDs) != 0 || len(updatedWorkspace.EnabledSkillIDs) != 0 {
		t.Fatalf("missing Workspace selections not cleared: %+v", updatedWorkspace)
	}
}

func TestCatalogDeleteCleansEnvironmentSelections(t *testing.T) {
	service := app.New(t.TempDir() + "/state.json")
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "environment", "")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("cleanup-mcp", "http://127.0.0.1:9000/mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	skillEntry, err := service.Skills.Add("cleanup-skill", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentSkill(environment.ID, skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceMCP(workspace.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceSkill(workspace.ID, skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}

	if err := service.MCPs.Remove(mcpEntry.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Skills.Remove(skillEntry.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.EnabledMCPIDs) != 0 || len(updated.EnabledSkillIDs) != 0 {
		t.Fatalf("catalog delete left Environment references: %+v", updated)
	}
	updatedWorkspace, err := service.Workspaces.Get(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedWorkspace.EnabledMCPIDs) != 0 || len(updatedWorkspace.EnabledSkillIDs) != 0 {
		t.Fatalf("catalog delete left Workspace references: %+v", updatedWorkspace)
	}
}

func TestSkillSourceDeleteCleansRemovedSkillSelections(t *testing.T) {
	service := app.New(t.TempDir() + "/state.json")
	root := t.TempDir()
	workspace, err := service.Workspaces.Add(root, "workspace")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "environment", "")
	if err != nil {
		t.Fatal(err)
	}

	const sourceID = "source-cleanup"
	const skillID = "skill-source-cleanup"
	if err := service.Store.Update(func(state *model.State) error {
		state.SkillSources = append(state.SkillSources, model.SkillSource{ID: sourceID, Root: root})
		state.Skills = append(state.Skills, model.CatalogEntry{ID: skillID, Name: "source skill", SourceID: sourceID})
		for i := range state.Environments {
			if state.Environments[i].ID == environment.ID {
				state.Environments[i].EnabledSkillIDs = []string{skillID}
			}
		}
		for i := range state.Workspaces {
			if state.Workspaces[i].ID == workspace.ID {
				state.Workspaces[i].EnabledSkillIDs = []string{skillID}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	result, err := service.Skills.RemoveSkillSource(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed != 1 {
		t.Fatalf("removed=%d want 1", result.Removed)
	}
	updated, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.EnabledSkillIDs) != 0 {
		t.Fatalf("source delete left Environment Skill references: %+v", updated.EnabledSkillIDs)
	}
	updatedWorkspace, err := service.Workspaces.Get(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedWorkspace.EnabledSkillIDs) != 0 {
		t.Fatalf("source delete left Workspace Skill references: %+v", updatedWorkspace.EnabledSkillIDs)
	}
}
