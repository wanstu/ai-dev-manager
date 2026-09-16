package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/verifier"
)

func TestEmptyDirectoryCanBeRegisteredAndOpened(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "empty")
	if err != nil {
		t.Fatalf("register empty directory: %v", err)
	}
	env, err := service.Environments.Create(ws.ID, "empty-task", "")
	if err != nil {
		t.Fatalf("create environment for empty directory: %v", err)
	}
	items, err := service.Tree(env.ID, ".", 2, 20)
	if err != nil {
		t.Fatalf("tree empty directory: %v", err)
	}
	if got := items.([]runtime.FileInfo); len(got) != 0 {
		t.Fatalf("empty directory tree = %#v", got)
	}
}

func TestEmptyWorkspaceAndEnvironmentListsAreNonNil(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))

	workspaces, err := service.Workspaces.List()
	if err != nil {
		t.Fatal(err)
	}
	if workspaces == nil || len(workspaces) != 0 {
		t.Fatalf("empty workspace list = %#v; want non-nil empty slice", workspaces)
	}

	environments, err := service.Environments.List()
	if err != nil {
		t.Fatal(err)
	}
	if environments == nil || len(environments) != 0 {
		t.Fatalf("empty environment list = %#v; want non-nil empty slice", environments)
	}
}

func TestPlainDirectoryDevelopmentDoesNotRequireGit(t *testing.T) {
	root := t.TempDir()
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("fixture must not be a git repository")
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatalf("register plain directory: %v", err)
	}
	env, err := service.Environments.Create(ws.ID, "task-a", "")
	if err != nil {
		t.Fatalf("create environment without git: %v", err)
	}
	if !pathutil.Same(env.Root, root) {
		t.Fatalf("root=%q want %q", env.Root, root)
	}

	caps, err := service.Capabilities(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"files.tree", "files.read", "search.text", "files.write", "files.edit", "files.delete"} {
		if !contains(caps, required) {
			t.Fatalf("missing core capability %q in %v", required, caps)
		}
	}
	for _, gitCap := range []string{"git.status", "git.diff", "git.branch"} {
		if contains(caps, gitCap) {
			t.Fatalf("plain directory unexpectedly exposes %q", gitCap)
		}
	}

	if _, err := service.Environments.AcquireWriter(env.ID, "session-a"); err != nil {
		t.Fatalf("acquire writer: %v", err)
	}
	if _, err := service.Write(env.ID, "session-a", "nested/new.txt", "alpha beta\n", true); err != nil {
		t.Fatalf("write: %v", err)
	}
	readValue, err := service.Read(env.ID, "nested/new.txt", 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if readValue.(string) != "alpha beta\n" {
		t.Fatalf("read=%q", readValue)
	}
	if _, err := service.Edit(env.ID, "session-a", "nested/new.txt", "beta", "gamma", 1); err != nil {
		t.Fatalf("edit: %v", err)
	}
	searchValue, err := service.Search(env.ID, ".", "gamma", 0, 0, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(strings.TrimSpace(toText(searchValue)), "gamma") {
		t.Fatalf("search result does not contain edited text: %#v", searchValue)
	}
	if _, err := service.Delete(env.ID, "session-a", "nested/new.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "nested", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists")
	}

	if _, err := service.GitStatus(context.Background(), env.ID); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("git status should fail locally as unsupported, got %v", err)
	}

	reloaded := app.New(statePath)
	persisted, err := reloaded.Environments.Get(env.ID)
	if err != nil {
		t.Fatalf("environment did not survive restart: %v", err)
	}
	if persisted.Writer == nil || persisted.Writer.Owner != "session-a" {
		t.Fatalf("writer lease did not survive restart: %#v", persisted.Writer)
	}
}

func TestDevelopmentContextCatalogSelectionsAndMemoryPersistWithoutLeaking(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}

	mcpEntry, err := service.MCPs.AddMCP("filesystem-extra", "http://example.test/filesystem-extra", true)
	if err != nil {
		t.Fatal(err)
	}
	skillEntry, err := service.Skills.Add("go-project", false)
	if err != nil {
		t.Fatal(err)
	}

	envA, err := service.Environments.Create(ws.ID, "a", "")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(envA.EnabledMCPIDs, mcpEntry.ID) || contains(envA.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("environment A defaults = mcp:%v skill:%v", envA.EnabledMCPIDs, envA.EnabledSkillIDs)
	}

	if _, err := service.MCPs.SetDefault(mcpEntry.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Skills.SetDefault(skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	envB, err := service.Environments.Create(ws.ID, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if contains(envB.EnabledMCPIDs, mcpEntry.ID) || !contains(envB.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("environment B defaults = mcp:%v skill:%v", envB.EnabledMCPIDs, envB.EnabledSkillIDs)
	}

	envA, err = service.Environments.Get(envA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(envA.EnabledMCPIDs, mcpEntry.ID) || contains(envA.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("global default change rewrote existing environment A: %+v", envA)
	}

	if _, err := service.SetEnvironmentMCP(envB.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetEnvironmentSkill(envA.ID, skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.GlobalWrite("machine", "windows"); err != nil {
		t.Fatal(err)
	}
	if err := service.Memory.EnvironmentWrite(envA.ID, "task", "login"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Memory.EnvironmentRead(envB.ID, "task"); err == nil {
		t.Fatal("environment B must not read environment A private memory")
	}
	if got, err := service.Memory.GlobalRead("machine"); err != nil || got.Value != "windows" {
		t.Fatalf("global memory read = %+v err=%v", got, err)
	}

	if err := service.MCPs.Remove(mcpEntry.ID); err != nil {
		t.Fatal(err)
	}
	envB, err = service.Environments.Get(envB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(envB.EnabledMCPIDs, mcpEntry.ID) {
		t.Fatalf("removed global MCP should clear Environment selection, got %v", envB.EnabledMCPIDs)
	}

	reloaded := app.New(statePath)
	persistedA, err := reloaded.Environments.Get(envA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(persistedA.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("environment skill selection did not persist: %v", persistedA.EnabledSkillIDs)
	}
	if got, err := reloaded.Memory.EnvironmentRead(envA.ID, "task"); err != nil || got.Value != "login" {
		t.Fatalf("environment memory did not persist: %+v err=%v", got, err)
	}
	if got, err := reloaded.Memory.GlobalRead("machine"); err != nil || got.Value != "windows" {
		t.Fatalf("global memory did not persist: %+v err=%v", got, err)
	}
}

func TestWorkspaceLifecycleManagementProtectsReferencesAndProjectFiles(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "before")
	if err != nil {
		t.Fatal(err)
	}

	inspected, err := service.Workspaces.Get(ws.ID)
	if err != nil || inspected.ID != ws.ID || !pathutil.Same(inspected.Path, root) {
		t.Fatalf("workspace inspect = %+v err=%v", inspected, err)
	}
	renamed, err := service.Workspaces.Rename(ws.ID, "after")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != ws.ID || !pathutil.Same(renamed.Path, root) || renamed.Name != "after" {
		t.Fatalf("workspace rename changed identity/path unexpectedly: %+v", renamed)
	}
	if _, err := service.Workspaces.Rename(ws.ID, "   "); err == nil {
		t.Fatal("empty workspace name must be rejected")
	}

	env, err := service.Environments.Create(ws.ID, "guard", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Workspaces.Remove(ws.ID); err == nil || !strings.Contains(err.Error(), env.ID) {
		t.Fatalf("workspace removal with Environment reference must fail clearly, got %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("blocked workspace removal touched project file: %v", err)
	}

	if _, err := service.Environments.Remove(env.ID); err != nil {
		t.Fatal(err)
	}
	removed, err := service.Workspaces.Remove(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != ws.ID || !pathutil.Same(removed.Path, root) {
		t.Fatalf("removed workspace = %+v", removed)
	}
	if _, err := service.Workspaces.Get(ws.ID); err == nil {
		t.Fatal("removed workspace must no longer be registered")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep me\n" {
		t.Fatalf("workspace removal must not delete project files: data=%q err=%v", data, err)
	}

	reloaded := app.New(statePath)
	if items, err := reloaded.Workspaces.List(); err != nil || len(items) != 0 {
		t.Fatalf("workspace removal did not persist: items=%+v err=%v", items, err)
	}
}

func TestEnvironmentManagementViewsResolveContextWithoutLeakingPrivateMemory(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "projects")
	if err != nil {
		t.Fatal(err)
	}
	resolvedMCP, err := service.MCPs.AddMCP("filesystem", "http://example.test/filesystem", false)
	if err != nil {
		t.Fatal(err)
	}
	removedMCP, err := service.MCPs.AddMCP("removed-mcp", "http://example.test/removed-mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	resolvedSkill, err := service.Skills.Add("go-project", false)
	if err != nil {
		t.Fatal(err)
	}
	removedSkill, err := service.Skills.Add("removed-skill", false)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "inspect", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{resolvedMCP.ID, removedMCP.ID} {
		if _, err := service.SetEnvironmentMCP(env.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{resolvedSkill.ID, removedSkill.ID} {
		if _, err := service.SetEnvironmentSkill(env.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Memory.EnvironmentWrite(env.ID, "secret", "do-not-list"); err != nil {
		t.Fatal(err)
	}
	if err := service.MCPs.Remove(removedMCP.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Skills.Remove(removedSkill.ID); err != nil {
		t.Fatal(err)
	}
	// Deletion now clears normal selections; seed stale references directly so
	// the management inspection still covers legacy unresolved state.
	if _, err := service.Environments.SetMCP(env.ID, removedMCP.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.SetSkill(env.ID, removedSkill.ID, true); err != nil {
		t.Fatal(err)
	}

	summaries, err := service.EnvironmentSummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("environment summaries = %+v", summaries)
	}
	if summaries[0].PrivateMemoryCount != 1 || summaries[0].PrivateMemory != nil {
		t.Fatalf("summary must expose only private Memory count: %+v", summaries[0])
	}
	encodedSummary, err := json.Marshal(summaries[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedSummary), "do-not-list") || strings.Contains(string(encodedSummary), "private_memory\"") {
		t.Fatalf("summary leaked private Memory values: %s", encodedSummary)
	}

	info, err := service.InspectEnvironment(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Workspace.ID != ws.ID || info.Workspace.Name != "projects" {
		t.Fatalf("inspect workspace = %+v", info.Workspace)
	}
	if info.Environment.PrivateMemoryCount != 1 || info.Environment.PrivateMemory != nil {
		t.Fatalf("inspect must expose private Memory count without values: %+v", info.Environment)
	}
	if len(info.EnabledMCPs) != 1 || info.EnabledMCPs[0].ID != resolvedMCP.ID || info.EnabledMCPs[0].Name != "filesystem" {
		t.Fatalf("resolved MCPs = %+v", info.EnabledMCPs)
	}
	if len(info.EnabledSkills) != 1 || info.EnabledSkills[0].ID != resolvedSkill.ID || info.EnabledSkills[0].Name != "go-project" {
		t.Fatalf("resolved Skills = %+v", info.EnabledSkills)
	}
	if len(info.UnresolvedMCPIDs) != 1 || info.UnresolvedMCPIDs[0] != removedMCP.ID {
		t.Fatalf("unresolved MCP IDs = %+v", info.UnresolvedMCPIDs)
	}
	if len(info.UnresolvedSkillIDs) != 1 || info.UnresolvedSkillIDs[0] != removedSkill.ID {
		t.Fatalf("unresolved Skill IDs = %+v", info.UnresolvedSkillIDs)
	}
	if len(info.Capabilities) == 0 {
		t.Fatal("inspect capabilities must not be empty for a normal directory")
	}
	encodedInfo, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedInfo), "do-not-list") {
		t.Fatalf("inspect leaked private Memory values: %s", encodedInfo)
	}
}

func TestNoCatalogOrMemoryDataIsNotADevelopmentPrerequisite(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "no-context-data", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(env.EnabledMCPIDs) != 0 || len(env.EnabledSkillIDs) != 0 || len(env.PrivateMemory) != 0 {
		t.Fatalf("expected empty optional context data: %+v", env)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "session"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Write(env.ID, "session", "works.txt", "still develops\n", false); err != nil {
		t.Fatalf("development must work without MCP/Skill/Memory data: %v", err)
	}
}

func TestSingleWriterIsScopedByPhysicalRoot(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	envA, err := service.Environments.Create(ws.ID, "a", "")
	if err != nil {
		t.Fatal(err)
	}
	envB, err := service.Environments.Create(ws.ID, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(envA.ID, "session-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(envB.ID, "session-b"); err == nil {
		t.Fatal("expected writer conflict for two environments sharing one physical root")
	}
	if _, err := service.Write(envB.ID, "session-b", "should-not-exist.txt", "x", true); err == nil {
		t.Fatal("mutation without writer should fail")
	}
}

func TestExecIsOptionalAndExplicitlyAllowlisted(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "exec", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "session-exec"); err != nil {
		t.Fatal(err)
	}

	caps, err := service.Capabilities(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(caps, "shell.exec") {
		t.Fatalf("shell.exec should not exist before allowlisting: %v", caps)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	caps, err = service.Capabilities(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(caps, "shell.exec") {
		t.Fatalf("shell.exec missing after allowlisting: %v", caps)
	}
	value, err := service.Exec(context.Background(), env.ID, "session-exec", exe, []string{"-test.run=TestExecHelperProcess"}, "", 10000, 20000)
	if err != nil {
		t.Fatalf("exec allowlisted test binary: %v", err)
	}
	if !strings.Contains(toText(value), "ADM_V2_EXEC_OK") {
		t.Fatalf("unexpected exec result: %#v", value)
	}

	if err := service.RemoveAllowedExecutable(exe); err != nil {
		t.Fatalf("remove allowlisted executable: %v", err)
	}
	allowed, err := service.AllowedExecutables()
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 0 {
		t.Fatalf("allowlist after removal = %v; want empty", allowed)
	}
	caps, err = service.Capabilities(context.Background(), env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(caps, "shell.exec") {
		t.Fatalf("shell.exec must disappear after removing last allowed executable: %v", caps)
	}
	if _, err := service.Exec(context.Background(), env.ID, "session-exec", exe, []string{"-test.run=TestExecHelperProcess"}, "", 10000, 20000); err == nil {
		t.Fatal("removed executable must no longer be executable")
	}
	if err := service.RemoveAllowedExecutable(exe); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("removing missing executable must fail clearly, got %v", err)
	}
}

func TestVerifierDefinitionsPersistPerEnvironmentAndZeroConfigReloadsCleanly(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	service := app.New(statePath)
	ws, err := service.Workspaces.Add(root, "verifier-persistence")
	if err != nil {
		t.Fatal(err)
	}
	envA, err := service.Environments.Create(ws.ID, "a", "")
	if err != nil {
		t.Fatal(err)
	}
	envB, err := service.Environments.Create(ws.ID, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := service.AddVerifier(envA.ID, model.VerifierDefinition{
		Name:           "go tests",
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     "go",
		Args:           []string{"test", "./..."},
		TimeoutSeconds: 90,
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded := app.New(statePath)
	itemsA, err := reloaded.ListVerifiers(envA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(itemsA) != 1 || itemsA[0].ID != definition.ID || itemsA[0].Executable != "go" || len(itemsA[0].Args) != 2 {
		t.Fatalf("persisted verifier definitions = %+v", itemsA)
	}
	itemsB, err := reloaded.ListVerifiers(envB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if itemsB == nil || len(itemsB) != 0 {
		t.Fatalf("zero-config Environment reload = %#v; want non-nil empty slice", itemsB)
	}
	if _, err := reloaded.RemoveVerifier(envB.ID, definition.ID); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("removing missing verifier must fail clearly, got %v", err)
	}
	if itemsA, err := reloaded.ListVerifiers(envA.ID); err != nil || len(itemsA) != 1 {
		t.Fatalf("Environment B operation affected Environment A definitions: %+v err=%v", itemsA, err)
	}
}

func TestRunVerifierBoundsOutputAndPreservesStructuredPass(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "verifier-bounds")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "bounds", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "bounds-owner"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_HELPER", "1")
	t.Setenv("ADM_V2_VERIFIER_MODE", "chatty")
	definition, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     exe,
		Args:           []string{"-test.run=TestVerifierHelperProcess$"},
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RunVerifier(context.Background(), env.ID, "bounds-owner", definition.ID, 128)
	if err != nil {
		t.Fatalf("run chatty verifier: %v", err)
	}
	if result.Status != verifier.StatusPassed || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("chatty verifier result = %+v", result)
	}
	if len(result.Stdout) != 128 || len(result.Stderr) != 128 {
		t.Fatalf("bounded verifier output lengths = stdout:%d stderr:%d; want 128 each", len(result.Stdout), len(result.Stderr))
	}
}

func TestRunVerifierUsesRuntimeCwdContainment(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "verifier-cwd")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "cwd", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "cwd-owner"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_HELPER", "1")
	t.Setenv("ADM_V2_VERIFIER_MODE", "cwd")
	newDefinition := func(cwd string) model.VerifierDefinition {
		return model.VerifierDefinition{
			Kind:           verifier.KindCustom,
			Enabled:        true,
			Executable:     exe,
			Args:           []string{"-test.run=TestVerifierHelperProcess$"},
			Cwd:            cwd,
			TimeoutSeconds: 5,
		}
	}

	contained, err := service.AddVerifier(env.ID, newDefinition("inside"))
	if err != nil {
		t.Fatal(err)
	}
	containedResult, err := service.RunVerifier(context.Background(), env.ID, "cwd-owner", contained.ID, 4096)
	if err != nil {
		t.Fatalf("contained verifier cwd: %v", err)
	}
	stdoutLines := strings.Split(strings.TrimSpace(containedResult.Stdout), "\n")
	if containedResult.Status != verifier.StatusPassed || len(stdoutLines) == 0 || !pathutil.Same(strings.TrimSpace(stdoutLines[0]), inside) {
		t.Fatalf("contained verifier cwd result = %+v", containedResult)
	}

	for name, cwd := range map[string]string{
		"parent escape": "..",
		"absolute":      outside,
	} {
		definition, err := service.AddVerifier(env.ID, newDefinition(cwd))
		if err != nil {
			t.Fatalf("store %s verifier definition: %v", name, err)
		}
		if _, err := service.RunVerifier(context.Background(), env.ID, "cwd-owner", definition.ID, 4096); err == nil {
			t.Fatalf("%s verifier cwd must be rejected", name)
		}
	}

	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, link); err == nil {
		definition, err := service.AddVerifier(env.ID, newDefinition("outside-link"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.RunVerifier(context.Background(), env.ID, "cwd-owner", definition.ID, 4096); err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Fatalf("symlink-outside verifier cwd must be rejected clearly, got %v", err)
		}
	}
}

func TestZeroVerifierConfigurationDoesNotBlockNormalDevelopment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "zero-verifier")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "zero", "")
	if err != nil {
		t.Fatal(err)
	}
	verifiers, err := service.ListVerifiers(env.ID)
	if err != nil {
		t.Fatal(err)
	}
	if verifiers == nil || len(verifiers) != 0 {
		t.Fatalf("zero verifier list = %#v; want non-nil empty slice", verifiers)
	}
	if _, err := service.Tree(env.ID, ".", 2, 20); err != nil {
		t.Fatalf("tree with zero verifier config: %v", err)
	}
	if value, err := service.Read(env.ID, "existing.txt", 0); err != nil || value.(string) != "alpha\n" {
		t.Fatalf("read with zero verifier config = %#v err=%v", value, err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "zero-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Write(env.ID, "zero-owner", "work.txt", "one two\n", false); err != nil {
		t.Fatalf("write with zero verifier config: %v", err)
	}
	if _, err := service.Edit(env.ID, "zero-owner", "work.txt", "two", "three", 1); err != nil {
		t.Fatalf("edit with zero verifier config: %v", err)
	}
	if matches, err := service.Search(env.ID, ".", "three", 0, 0, 0); err != nil || len(matches.([]runtime.SearchMatch)) == 0 {
		t.Fatalf("search with zero verifier config = %#v err=%v", matches, err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Exec(context.Background(), env.ID, "zero-owner", exe, []string{"-test.run=TestExecHelperProcess"}, "", 10000, 4096); err != nil {
		t.Fatalf("exec with zero verifier config: %v", err)
	}
}

func TestRunVerifierRejectsMissingAndDisabledDefinitionsLocally(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "verifier-errors")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "errors", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "errors-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunVerifier(context.Background(), env.ID, "errors-owner", "vf_missing", 4096); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing verifier must fail locally, got %v", err)
	}
	disabled, err := service.AddVerifier(env.ID, model.VerifierDefinition{
		Kind:       verifier.KindCustom,
		Enabled:    false,
		Executable: "anything",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunVerifier(context.Background(), env.ID, "errors-owner", disabled.ID, 4096); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled verifier must fail locally, got %v", err)
	}
}

func TestRunVerifierTracerPassFailTimeoutAndPolicyError(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	ws, err := service.Workspaces.Add(root, "verifier")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(ws.ID, "verifier-task", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "verifier-writer"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_HELPER", "1")
	helperArgs := []string{"-test.run=TestVerifierHelperProcess$"}

	passDefinition, err := service.Verifiers.Add(env.ID, model.VerifierDefinition{
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     exe,
		Args:           helperArgs,
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_MODE", "pass")
	passed, err := service.RunVerifier(context.Background(), env.ID, "verifier-writer", passDefinition.ID, 4096)
	if err != nil {
		t.Fatalf("run passing verifier: %v", err)
	}
	if passed.ID != passDefinition.ID || passed.Kind != verifier.KindTest || passed.Status != verifier.StatusPassed || passed.ExitCode != 0 || passed.TimedOut {
		t.Fatalf("passing verifier result = %+v", passed)
	}
	if !strings.Contains(passed.Stdout, "VERIFIER_PASS") || !strings.Contains(passed.Stderr, "VERIFIER_STDERR") {
		t.Fatalf("passing verifier output = %+v", passed)
	}

	failDefinition, err := service.Verifiers.Add(env.ID, model.VerifierDefinition{
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     exe,
		Args:           helperArgs,
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_MODE", "fail")
	failed, err := service.RunVerifier(context.Background(), env.ID, "verifier-writer", failDefinition.ID, 4096)
	if err != nil {
		t.Fatalf("run failing verifier: %v", err)
	}
	if failed.Status != verifier.StatusFailed || failed.ExitCode != 7 || failed.TimedOut {
		t.Fatalf("failing verifier result = %+v", failed)
	}
	if !strings.Contains(failed.Stdout, "VERIFIER_FAIL") {
		t.Fatalf("failing verifier output = %+v", failed)
	}

	timeoutDefinition, err := service.Verifiers.Add(env.ID, model.VerifierDefinition{
		Kind:           verifier.KindTest,
		Enabled:        true,
		Executable:     exe,
		Args:           helperArgs,
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_MODE", "timeout")
	timedOut, err := service.RunVerifier(context.Background(), env.ID, "verifier-writer", timeoutDefinition.ID, 4096)
	if err != nil {
		t.Fatalf("run timeout verifier: %v", err)
	}
	if timedOut.Status != verifier.StatusFailed || !timedOut.TimedOut || timedOut.ExitCode != -1 || timedOut.DurationMs < 900 {
		t.Fatalf("timeout verifier result = %+v", timedOut)
	}

	policyDefinition, err := service.Verifiers.Add(env.ID, model.VerifierDefinition{
		Kind:       verifier.KindCustom,
		Enabled:    true,
		Executable: "adm-v2-not-allowlisted-verifier",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADM_V2_VERIFIER_MODE", "pass")
	if _, err := service.RunVerifier(context.Background(), env.ID, "verifier-writer", policyDefinition.ID, 4096); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("policy failure must remain a local error, got %v", err)
	}
}

func TestVerifierHelperProcess(t *testing.T) {
	if os.Getenv("ADM_V2_VERIFIER_HELPER") != "1" {
		return
	}
	switch os.Getenv("ADM_V2_VERIFIER_MODE") {
	case "pass":
		fmt.Fprintln(os.Stdout, "VERIFIER_PASS")
		fmt.Fprintln(os.Stderr, "VERIFIER_STDERR")
	case "fail":
		fmt.Fprintln(os.Stdout, "VERIFIER_FAIL")
		os.Exit(7)
	case "timeout":
		time.Sleep(2 * time.Second)
		fmt.Fprintln(os.Stdout, "VERIFIER_TOO_LATE")
	case "chatty":
		fmt.Fprint(os.Stdout, strings.Repeat("O", 4096))
		fmt.Fprint(os.Stderr, strings.Repeat("E", 4096))
	case "cwd":
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(os.Stdout, cwd)
	default:
		os.Exit(2)
	}
}

func TestWorkspaceSelectionsAreInheritedWithoutCopyingIntoEnvironmentSelections(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(t.TempDir(), "workspace-inheritance")
	if err != nil {
		t.Fatal(err)
	}
	mcpEntry, err := service.MCPs.AddMCP("workspace-mcp", "http://example.test/workspace-mcp", false)
	if err != nil {
		t.Fatal(err)
	}
	skillEntry, err := service.Skills.Add("workspace-skill", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceMCP(workspace.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceSkill(workspace.ID, skillEntry.ID, true); err != nil {
		t.Fatal(err)
	}

	environment, err := service.Environments.Create(workspace.ID, "child", "")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(environment.EnabledMCPIDs, mcpEntry.ID) || !contains(environment.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("workspace selections were not inherited: %+v", environment)
	}
	if len(environment.ExplicitMCPIDs) != 0 || len(environment.ExplicitSkillIDs) != 0 || !contains(environment.InheritedMCPIDs, mcpEntry.ID) || !contains(environment.InheritedSkillIDs, skillEntry.ID) {
		t.Fatalf("workspace inheritance was copied into explicit Environment state: %+v", environment)
	}
	state, err := service.Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Environments) != 1 || len(state.Environments[0].EnabledMCPIDs) != 0 || len(state.Environments[0].EnabledSkillIDs) != 0 {
		t.Fatalf("persisted Environment should keep only explicit selections: %+v", state.Environments)
	}

	if _, err := service.SetEnvironmentMCP(environment.ID, mcpEntry.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetWorkspaceMCP(workspace.ID, mcpEntry.ID, false); err != nil {
		t.Fatal(err)
	}
	afterWorkspaceDisable, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(afterWorkspaceDisable.EnabledMCPIDs, mcpEntry.ID) || !contains(afterWorkspaceDisable.ExplicitMCPIDs, mcpEntry.ID) || contains(afterWorkspaceDisable.InheritedMCPIDs, mcpEntry.ID) {
		t.Fatalf("explicit Environment selection should survive Workspace disable: %+v", afterWorkspaceDisable)
	}
	if _, err := service.SetEnvironmentMCP(environment.ID, mcpEntry.ID, false); err != nil {
		t.Fatal(err)
	}
	fullyDisabled, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(fullyDisabled.EnabledMCPIDs, mcpEntry.ID) {
		t.Fatalf("MCP remained effectively enabled after both layers were disabled: %+v", fullyDisabled)
	}

	if err := service.Skills.Remove(skillEntry.ID); err != nil {
		t.Fatal(err)
	}
	workspaceAfterDelete, err := service.Workspaces.Get(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(workspaceAfterDelete.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("Skill deletion left Workspace selection behind: %+v", workspaceAfterDelete.EnabledSkillIDs)
	}
	environmentAfterDelete, err := service.Environments.Get(environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if contains(environmentAfterDelete.EnabledSkillIDs, skillEntry.ID) {
		t.Fatalf("Skill deletion left effective Environment selection behind: %+v", environmentAfterDelete.EnabledSkillIDs)
	}
}

func TestExecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "" {
		return
	}
	if strings.Contains(strings.Join(os.Args, " "), "TestExecHelperProcess") {
		fmtPrintln("ADM_V2_EXEC_OK")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toText(value any) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(fmtSprint(value)), "{", ""), "}", ""))
}

func fmtSprint(value any) string { return fmt.Sprint(value) }
func fmtPrintln(value any)       { fmt.Println(value) }

func TestExecDenialsRecordCountsAndClearWhenAllowed(t *testing.T) {
	root := t.TempDir()
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(root, "exec-denials")
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.Environments.Create(workspace.ID, "exec-denials", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Environments.AcquireWriter(env.ID, "writer"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		_, err = service.Exec(context.Background(), env.ID, "writer", "adm-v2-denied-tool", []string{"secret-arg-must-not-be-persisted"}, "", 1000, 1024)
		if err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("Exec denied err = %v", err)
		}
	}

	denials, err := service.ExecDenials()
	if err != nil {
		t.Fatal(err)
	}
	if len(denials) != 1 || denials[0].Executable != "adm-v2-denied-tool" || denials[0].Count != 2 {
		t.Fatalf("denials = %+v", denials)
	}
	if denials[0].LastEnvironmentID != env.ID || denials[0].LastSurface != "exec" || !strings.Contains(denials[0].LastReason, "not allowed") {
		t.Fatalf("denial metadata = %+v", denials[0])
	}
	persisted, err := os.ReadFile(filepath.Join(filepath.Dir(service.Store.Path()), "state.json"))
	if err != nil {
		persisted, err = os.ReadFile(service.Store.Path())
	}
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "secret-arg-must-not-be-persisted") {
		t.Fatalf("denial persisted command args: %s", persisted)
	}

	if err := service.AllowExecutable("adm-v2-denied-tool"); err != nil {
		t.Fatal(err)
	}
	denials, err = service.ExecDenials()
	if err != nil {
		t.Fatal(err)
	}
	if len(denials) != 0 {
		t.Fatalf("allowing executable should clear denial observation, got %+v", denials)
	}
}
