package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/model"
)

func TestCLIWorkspacePrepareKeepsUnpreparedWorkspaceReadOnly(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()

	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var added workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &added); err != nil {
		t.Fatal(err)
	}

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Inspect(added.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Runtime.PolicyMode != "read-only" {
		t.Fatalf("unprepared policy mode = %q, want read-only", snapshot.Runtime.PolicyMode)
	}
	if containsString(snapshot.Runtime.Capabilities, "git.worktree") || containsString(snapshot.Runtime.Capabilities, "verify.run") {
		t.Fatalf("unprepared capabilities unexpectedly elevated: %+v", snapshot.Runtime.Capabilities)
	}
}

func TestCLIWorkspacePrepareGoKeepsMachineToolPathOutOfProjectConfig(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := t.TempDir()
	fakeGo := filepath.Join(workspaceRoot, "go.exe")
	if err := os.WriteFile(fakeGo, []byte("test executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	var addOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "add", "--path", workspaceRoot}, &addOut, io.Discard); err != nil {
		t.Fatalf("workspace add error = %v", err)
	}
	var added workspaceOutput
	if err := json.Unmarshal(addOut.Bytes(), &added); err != nil {
		t.Fatal(err)
	}

	service, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(workspaceRoot, "helper.exe")
	if err := service.Store().SaveProject(workspaceRoot, model.ConfigLayer{
		Scope: model.ScopeProject,
		Verifiers: map[string]model.VerifierDefinition{
			"keep": {ID: "keep", Kind: "custom", Executable: "helper", Args: []string{"check"}},
		},
		Policy: &model.Policy{
			Mode:               "workspace-write",
			AllowedExecutables: []string{"helper"},
			ToolPaths:          map[string]string{"helper": helperPath},
		},
	}); err != nil {
		t.Fatal(err)
	}

	var plainOut bytes.Buffer
	if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "prepare", added.ID}, &plainOut, io.Discard); err != nil {
		t.Fatalf("plain workspace prepare error = %v", err)
	}
	var plain struct {
		PolicyMode         string   `json:"policy_mode"`
		AllowedExecutables []string `json:"allowed_executables"`
		Capabilities       []string `json:"capabilities"`
	}
	if err := json.Unmarshal(plainOut.Bytes(), &plain); err != nil {
		t.Fatalf("decode plain prepare output: %v output=%s", err, plainOut.String())
	}
	if plain.PolicyMode != "standard" || !containsString(plain.AllowedExecutables, "git") || containsString(plain.AllowedExecutables, "go") || !containsString(plain.Capabilities, "git.worktree") {
		t.Fatalf("plain prepared workspace = %+v", plain)
	}

	projectAfterPlain, err := service.Store().LoadProject(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if projectAfterPlain.Policy == nil || projectAfterPlain.Policy.Mode != "workspace-write" || len(projectAfterPlain.Policy.AllowedExecutables) != 1 || projectAfterPlain.Policy.AllowedExecutables[0] != "helper" || projectAfterPlain.Policy.ToolPaths["helper"] != helperPath {
		t.Fatalf("plain prepare mutated project policy = %+v", projectAfterPlain.Policy)
	}
	preparedWorkspace, err := service.Registry().Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preparedWorkspace.LocalPolicy == nil || preparedWorkspace.LocalPolicy.Mode != "standard" || !containsString(preparedWorkspace.LocalPolicy.AllowedExecutables, "git") || len(preparedWorkspace.LocalPolicy.ToolPaths) != 0 {
		t.Fatalf("plain local policy = %+v", preparedWorkspace.LocalPolicy)
	}

	if err := run(context.Background(), []string{"--config-root", configRoot, "workspace", "prepare", added.ID, "--go", "go.exe"}, io.Discard, io.Discard); err == nil {
		t.Fatal("relative --go path unexpectedly accepted")
	}

	for i := 0; i < 2; i++ {
		var prepareOut bytes.Buffer
		if err := run(context.Background(), []string{"--config-root", configRoot, "--json", "workspace", "prepare", added.ID, "--go", fakeGo}, &prepareOut, io.Discard); err != nil {
			t.Fatalf("Go workspace prepare error = %v", err)
		}
		var prepared struct {
			PolicyMode         string            `json:"policy_mode"`
			AllowedExecutables []string          `json:"allowed_executables"`
			ToolPaths          map[string]string `json:"tool_paths"`
			Capabilities       []string          `json:"capabilities"`
		}
		if err := json.Unmarshal(prepareOut.Bytes(), &prepared); err != nil {
			t.Fatalf("decode Go prepare output: %v output=%s", err, prepareOut.String())
		}
		if prepared.PolicyMode != "standard" || !containsString(prepared.AllowedExecutables, "git") || !containsString(prepared.AllowedExecutables, "go") || prepared.ToolPaths["go"] != filepath.Clean(fakeGo) || !containsString(prepared.Capabilities, "git.worktree") || !containsString(prepared.Capabilities, "verify.run") {
			t.Fatalf("Go prepared workspace = %+v", prepared)
		}
	}

	project, err := service.Store().LoadProject(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if project.Policy == nil || project.Policy.Mode != "workspace-write" || len(project.Policy.AllowedExecutables) != 1 || project.Policy.AllowedExecutables[0] != "helper" || project.Policy.ToolPaths["helper"] != helperPath {
		t.Fatalf("Go prepare mutated project policy = %+v", project.Policy)
	}
	if _, leaked := project.Policy.ToolPaths["go"]; leaked {
		t.Fatalf("machine Go path leaked into project policy: %+v", project.Policy.ToolPaths)
	}
	if verifier, ok := project.Verifiers["keep"]; !ok || verifier.Executable != "helper" {
		t.Fatalf("project verifier was not preserved: %+v", project.Verifiers)
	}
	if verifier, ok := project.Verifiers["go-test"]; !ok || verifier.Executable != "go" || strings.Join(verifier.Args, " ") != "test ./..." || verifier.Enabled == nil || !*verifier.Enabled {
		t.Fatalf("go-test verifier = %+v", verifier)
	}

	reloadedService, err := controlplane.New(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	reloadedWorkspace, err := reloadedService.Registry().Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedWorkspace.LocalPolicy == nil || reloadedWorkspace.LocalPolicy.ToolPaths["go"] != filepath.Clean(fakeGo) || !containsString(reloadedWorkspace.LocalPolicy.AllowedExecutables, "git") || !containsString(reloadedWorkspace.LocalPolicy.AllowedExecutables, "go") {
		t.Fatalf("reloaded local policy = %+v", reloadedWorkspace.LocalPolicy)
	}
	_, effective, err := reloadedService.Resolve(added.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Policy == nil || effective.Policy.Policy.Mode != "standard" || !containsString(effective.Policy.Policy.AllowedExecutables, "helper") || !containsString(effective.Policy.Policy.AllowedExecutables, "git") || !containsString(effective.Policy.Policy.AllowedExecutables, "go") || effective.Policy.Policy.ToolPaths["helper"] != helperPath || effective.Policy.Policy.ToolPaths["go"] != filepath.Clean(fakeGo) {
		t.Fatalf("effective policy did not compose project + local policy: %+v", effective.Policy)
	}
}
