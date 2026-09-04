package runtime

import (
	"os"
	"strings"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestModifyDiffAndVerifyEndToEnd(t *testing.T) {
	root, gitPath := initGitRepository(t)
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	verifiers := map[string]model.ResolvedVerifier{
		"test": {
			VerifierDefinition: model.VerifierDefinition{
				ID: "test", Kind: "test", Executable: "helper", Args: helperArgs("echo", "test-ok"),
			},
			Source: model.ScopeProject,
		},
		"build": {
			VerifierDefinition: model.VerifierDefinition{
				ID: "build", Kind: "build", Executable: "helper", Args: helperArgs("echo", "build-ok"),
			},
			Source: model.ScopeProject,
		},
	}

	runtime := mustNativeWithID(t, "ws_e2e", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"git", "helper"},
		ToolPaths: map[string]string{
			"git":    gitPath,
			"helper": helper,
		},
	}, verifiers)

	if _, err := runtime.Edit("source.txt", "hello", "hello verified", 1); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}

	status, err := runtime.GitStatus()
	if err != nil {
		t.Fatalf("GitStatus() error = %v", err)
	}
	if len(status) != 1 || status[0].Path != "source.txt" {
		t.Fatalf("unexpected status after edit: %+v", status)
	}

	diff, err := runtime.GitDiff()
	if err != nil {
		t.Fatalf("GitDiff() error = %v", err)
	}
	if len(diff.Files) != 1 || diff.Files[0] != "source.txt" || !strings.Contains(diff.Patch, "hello verified") {
		t.Fatalf("unexpected diff after edit: files=%+v patch=%q", diff.Files, diff.Patch)
	}

	results, err := runtime.RunVerifiers("test", "build")
	if err != nil {
		t.Fatalf("RunVerifiers() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("RunVerifiers() returned %d results, want 2", len(results))
	}
	if results[0].ID != "test" || results[0].Status != VerifierPassed || strings.TrimSpace(results[0].Stdout) != "test-ok" {
		t.Fatalf("unexpected test verifier result: %+v", results[0])
	}
	if results[1].ID != "build" || results[1].Status != VerifierPassed || strings.TrimSpace(results[1].Stdout) != "build-ok" {
		t.Fatalf("unexpected build verifier result: %+v", results[1])
	}
}
