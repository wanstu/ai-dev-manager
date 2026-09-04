package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestVerifierRunnerPassFailSkipTimeoutAndOrdering(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	verifiers := map[string]model.ResolvedVerifier{
		"test": {
			VerifierDefinition: model.VerifierDefinition{ID: "test", Kind: "test", Executable: "helper", Args: helperArgs("echo", "tests-ok")},
			Source:             model.ScopeProject,
		},
		"build": {
			VerifierDefinition: model.VerifierDefinition{ID: "build", Kind: "build", Executable: "helper", Args: helperArgs("echo", "build-ok")},
			Source:             model.ScopeProject,
		},
		"fail": {
			VerifierDefinition: model.VerifierDefinition{ID: "fail", Kind: "custom", Executable: "helper", Args: helperArgs("exit", "7")},
			Source:             model.ScopeProject,
		},
		"skip": {
			VerifierDefinition: model.VerifierDefinition{ID: "skip", Kind: "lint", Enabled: verifierBool(false), Executable: "helper", Args: helperArgs("exit", "7")},
			Source:             model.ScopeProject,
			EnabledSource:      model.ScopeProject,
		},
		"timeout": {
			VerifierDefinition: model.VerifierDefinition{ID: "timeout", Kind: "test", Executable: "helper", Args: helperArgs("sleep", "1500"), TimeoutSeconds: 1},
			Source:             model.ScopeProject,
		},
	}
	runtime := mustNativeWithID(t, "ws_verifiers", t.TempDir(), model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	}, verifiers)

	passed, err := runtime.RunVerifier("test")
	if err != nil {
		t.Fatalf("RunVerifier(test) error = %v", err)
	}
	if passed.Status != VerifierPassed || passed.ExitCode != 0 || strings.TrimSpace(passed.Stdout) != "tests-ok" {
		t.Fatalf("unexpected test verifier result: %+v", passed)
	}

	failed, err := runtime.RunVerifier("fail")
	if err != nil {
		t.Fatalf("RunVerifier(fail) error = %v", err)
	}
	if failed.Status != VerifierFailed || failed.ExitCode != 7 || !strings.Contains(failed.Summary, `verifier "fail" failed`) {
		t.Fatalf("unexpected fail verifier result: %+v", failed)
	}

	skipped, err := runtime.RunVerifier("skip")
	if err != nil {
		t.Fatalf("RunVerifier(skip) error = %v", err)
	}
	if skipped.Status != VerifierSkipped || skipped.ExitCode != -1 {
		t.Fatalf("unexpected skipped verifier result: %+v", skipped)
	}

	timedOut, err := runtime.RunVerifier("timeout")
	if err != nil {
		t.Fatalf("RunVerifier(timeout) error = %v", err)
	}
	if timedOut.Status != VerifierFailed || !timedOut.TimedOut {
		t.Fatalf("unexpected timeout verifier result: %+v", timedOut)
	}

	all, err := runtime.RunVerifiers()
	if err != nil {
		t.Fatalf("RunVerifiers() error = %v", err)
	}
	gotIDs := make([]string, 0, len(all))
	for _, result := range all {
		gotIDs = append(gotIDs, result.ID)
	}
	wantIDs := []string{"build", "fail", "skip", "test", "timeout"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("RunVerifiers() order = %+v, want %+v", gotIDs, wantIDs)
	}
}

func TestVerifierRunnerRejectsOutsideCwdAndPolicyDeniedTool(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root error = %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll outside error = %v", err)
	}
	verifiers := map[string]model.ResolvedVerifier{
		"outside": {
			VerifierDefinition: model.VerifierDefinition{ID: "outside", Kind: "test", Executable: "helper", Args: helperArgs("echo", "x"), Cwd: outside},
		},
		"blocked": {
			VerifierDefinition: model.VerifierDefinition{ID: "blocked", Kind: "build", Executable: "blocked-tool"},
		},
	}
	runtime := mustNativeWithID(t, "ws_verifier_guard", root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	}, verifiers)

	_, err = runtime.RunVerifier("outside")
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
	_, err = runtime.RunVerifier("blocked")
	assertRuntimeErrorKind(t, err, ErrToolNotAllowed)
}

func TestNativeSnapshotsVerifierConfigurationAndExposesVerifyCapability(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	enabled := verifierBool(true)
	verifier := model.ResolvedVerifier{
		VerifierDefinition: model.VerifierDefinition{ID: "test", Kind: "test", Enabled: enabled, Executable: "helper", Args: helperArgs("echo", "original")},
		Source:             model.ScopeProject,
		EnabledSource:      model.ScopeProject,
	}
	cfg := map[string]model.ResolvedVerifier{"test": verifier}
	runtime := mustNativeWithID(t, "ws_verifier_snapshot", t.TempDir(), model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	}, cfg)

	changed := cfg["test"]
	changed.Args[3] = "changed"
	*changed.Enabled = false
	cfg["test"] = changed

	result, err := runtime.RunVerifier("test")
	if err != nil {
		t.Fatalf("RunVerifier() error = %v", err)
	}
	if result.Status != VerifierPassed || strings.TrimSpace(result.Stdout) != "original" {
		t.Fatalf("runtime verifier snapshot changed through caller mutation: %+v", result)
	}
	if !containsCapability(runtime.Capabilities(), CapabilityVerify) {
		t.Fatalf("verify capability missing: %+v", runtime.Capabilities())
	}
}

func verifierBool(value bool) *bool {
	copy := value
	return &copy
}

func containsCapability(values []Capability, target Capability) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
