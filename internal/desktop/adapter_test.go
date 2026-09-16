package desktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/desktop"
	"ai-dev-manager-v2/internal/management"
)

func TestAdapterExposesManagementBoundaryWithExplicitMemoryReads(t *testing.T) {
	root := t.TempDir()
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("fixture must not be a Git repository")
	}
	marker := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	application := app.New(filepath.Join(t.TempDir(), "state.json"))
	adapter := desktop.NewAdapter(management.New(application))

	ws, err := adapter.AddWorkspace(desktop.WorkspaceInput{Path: root, Name: "projects"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := adapter.CreateEnvironment(desktop.EnvironmentInput{WorkspaceID: ws.ID, Name: "main"})
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := adapter.AddMCP(desktop.MCPInput{Name: "filesystem", Endpoint: "http://127.0.0.1:9999/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	skillRoot := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(skillRoot, "go-project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "go-project", "SKILL.md"), []byte("# Go project\nUse Go tooling and run the relevant tests before finishing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillEntries, err := adapter.AddSkill(desktop.SkillInput{Root: skillRoot})
	if err != nil || len(skillEntries) != 1 {
		t.Fatalf("AddSkill entries=%+v err=%v", skillEntries, err)
	}
	skillEntry := skillEntries[0]
	if _, err := adapter.SetEnvironmentMCP(env.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	updatedMCP, err := adapter.UpdateMCP(mcpEntry.ID, desktop.MCPInput{Name: "filesystem-updated", Transport: "streamable-http", AuthMode: "none", Endpoint: "http://127.0.0.1:9998/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if updatedMCP.ID != mcpEntry.ID || updatedMCP.Name != "filesystem-updated" || updatedMCP.Endpoint != "http://127.0.0.1:9998/mcp" {
		t.Fatalf("Desktop MCP update = %+v", updatedMCP)
	}
	if _, err := adapter.SetEnvironmentSkill(env.ID, skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := adapter.WriteGlobalMemory("global", "global-value"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.WriteEnvironmentMemory(env.ID, "private", "private-value"); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.AllowExecutable("go"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := adapter.GetSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Workspaces) != 1 || len(snapshot.Environments) != 1 || snapshot.GlobalMemoryCount != 1 {
		t.Fatalf("desktop snapshot = %+v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "global-value") || strings.Contains(string(encoded), "private-value") {
		t.Fatalf("desktop overview leaked Memory values: %s", encoded)
	}

	inspection, err := adapter.InspectEnvironment(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Workspace.ID != ws.ID || len(inspection.EnabledMCPs) != 1 || len(inspection.EnabledSkills) != 1 || inspection.Environment.PrivateMemoryCount != 1 {
		t.Fatalf("desktop Environment inspection = %+v", inspection)
	}
	if inspection.EnabledMCPs[0].ID != mcpEntry.ID || inspection.EnabledMCPs[0].Name != "filesystem-updated" {
		t.Fatalf("Desktop MCP update changed Environment selection or stable ID: %+v", inspection.EnabledMCPs)
	}
	if inspection.Environment.PrivateMemory != nil {
		t.Fatalf("desktop Environment inspection leaked private Memory map: %+v", inspection.Environment.PrivateMemory)
	}

	globalMemory, err := adapter.ReadGlobalMemory("global")
	if err != nil || globalMemory.Value != "global-value" {
		t.Fatalf("explicit Global Memory read = %+v err=%v", globalMemory, err)
	}
	privateMemory, err := adapter.ReadEnvironmentMemory(env.ID, "private")
	if err != nil || privateMemory.Value != "private-value" {
		t.Fatalf("explicit Environment Memory read = %+v err=%v", privateMemory, err)
	}
	if items, err := adapter.ListGlobalMemory(); err != nil || len(items) != 1 {
		t.Fatalf("explicit Global Memory list = %+v err=%v", items, err)
	}
	if items, err := adapter.ListEnvironmentMemory(env.ID); err != nil || len(items) != 1 {
		t.Fatalf("explicit Environment Memory list = %+v err=%v", items, err)
	}

	if _, err := adapter.RenameWorkspace(ws.ID, "renamed-workspace"); err != nil {
		t.Fatal(err)
	}
	renamed, err := adapter.RenameEnvironment(env.ID, "renamed-environment")
	if err != nil || renamed.Name != "renamed-environment" || renamed.PrivateMemory != nil || renamed.PrivateMemoryCount != 1 {
		t.Fatalf("desktop Environment rename = %+v err=%v", renamed, err)
	}
	if _, err := adapter.SetMCPDefault(mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.SetSkillDefault(skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := adapter.DeleteEnvironmentMemory(env.ID, "private"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.DeleteGlobalMemory("global"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemoveMCP(mcpEntry.ID); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemoveSkill(skillEntry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.RemoveExecutable("go"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.RemoveEnvironment(env.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.RemoveWorkspace(ws.ID); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep\n" {
		t.Fatalf("desktop adapter removal must not delete project files: data=%q err=%v", data, err)
	}
}

func TestAdapterPreservesManagementErrorsAndRequiresInitialization(t *testing.T) {
	var nilAdapter *desktop.Adapter
	if _, err := nilAdapter.GetSnapshot(); err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("nil desktop adapter error = %v", err)
	}

	root := t.TempDir()
	application := app.New(filepath.Join(t.TempDir(), "state.json"))
	adapter := desktop.NewAdapter(management.New(application))
	ws, err := adapter.AddWorkspace(desktop.WorkspaceInput{Path: root, Name: "projects"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := adapter.CreateEnvironment(desktop.EnvironmentInput{WorkspaceID: ws.ID, Name: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.RenameEnvironment(env.ID, "   "); err == nil || !strings.Contains(err.Error(), "environment name is required") {
		t.Fatalf("desktop rename must preserve management error, got %v", err)
	}
	if _, err := adapter.RemoveWorkspace(ws.ID); err == nil || !strings.Contains(err.Error(), env.ID) {
		t.Fatalf("desktop Workspace remove must preserve Environment guard, got %v", err)
	}
	if _, err := adapter.RemoveExecutable("go"); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("desktop allowlist remove must preserve missing-entry error, got %v", err)
	}
}

func TestAdapterExposesMCPImportAndSkillSourceManagement(t *testing.T) {
	application := app.New(filepath.Join(t.TempDir(), "state.json"))
	adapter := desktop.NewAdapter(management.New(application))

	preview, err := adapter.PreviewMCPImport(desktop.MCPImportInput{
		Format:         app.MCPImportOpenCode,
		Content:        `{"mcp":{"servers":{"remote-ui":{"type":"remote","url":"https://example.test/mcp","headers":{"Authorization":"Bearer ui-secret"}}}}}`,
		SelectedNames:  []string{"remote-ui"},
		ConflictPolicy: "error",
		DefaultInclude: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || preview.Candidates[0].Name != "remote-ui" {
		t.Fatalf("Desktop MCP import preview = %+v", preview)
	}
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(previewJSON), "ui-secret") {
		t.Fatalf("Desktop MCP import preview leaked literal credential: %s", previewJSON)
	}
	applied, err := adapter.ApplyMCPImport(desktop.MCPImportInput{
		Format:         app.MCPImportOpenCode,
		Content:        `{"mcp":{"servers":{"remote-ui":{"type":"remote","url":"https://example.test/mcp","headers":{"Authorization":"Bearer ui-secret"}}}}}`,
		SelectedNames:  []string{"remote-ui"},
		ConflictPolicy: "error",
		DefaultInclude: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(applied.Result.Mutations) != 1 || applied.Result.Mutations[0].Definition.Name != "remote-ui" {
		t.Fatalf("Desktop MCP import apply = %+v", applied)
	}

	skillRoot := filepath.Join(t.TempDir(), "skill-source")
	if err := os.MkdirAll(filepath.Join(skillRoot, "diagnostics"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "diagnostics", "SKILL.md"), []byte("# Diagnostics\nInspect evidence first.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := adapter.AddSkillSource(desktop.SkillSourceInput{Root: skillRoot, DefaultInclude: true})
	if err != nil {
		t.Fatal(err)
	}
	supportRoot := filepath.Join(t.TempDir(), "skill-support")
	if err := os.MkdirAll(supportRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	updatedSource, err := adapter.UpdateSkillSource(source.ID, desktop.SkillSourceInput{Root: skillRoot, SupportRoots: []string{supportRoot}, DefaultInclude: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedSource.SupportRoots) != 1 || !strings.Contains(updatedSource.SupportRoots[0], "skill-support") {
		t.Fatalf("updated source support roots not returned: %+v", updatedSource)
	}
	if source.ID == "" || source.LastRefreshStatus != "pending" {
		t.Fatalf("Desktop Skill source add = %+v", source)
	}
	sources, err := adapter.ListSkillSources()
	if err != nil || len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("Desktop Skill source list = %+v err=%v", sources, err)
	}
	refreshed, err := adapter.RefreshSkillSource(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Skills) != 1 || refreshed.Skills[0].Name != "diagnostics" {
		t.Fatalf("Desktop Skill source refresh = %+v", refreshed)
	}
	removed, err := adapter.RemoveSkillSource(source.ID)
	if err != nil || removed.Removed != 1 {
		t.Fatalf("Desktop Skill source remove = %+v err=%v", removed, err)
	}
}
