package verifier_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/store"
	"ai-dev-manager-v2/internal/verifier"
)

func TestDefinitionServiceUsesEnvironmentScopedStableIDs(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := store.New(statePath)
	if err := s.Update(func(state *model.State) error {
		state.Environments = append(state.Environments,
			model.Environment{ID: "env_a", Name: "a"},
			model.Environment{ID: "env_b", Name: "b"},
		)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	service := verifier.New(s)
	added, err := service.Add("env_a", model.VerifierDefinition{
		Kind:       verifier.KindTest,
		Enabled:    true,
		Executable: "go",
		Args:       []string{"test", "./..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(added.ID, "vf_") {
		t.Fatalf("verifier id = %q; want vf_ prefix", added.ID)
	}
	if added.ID == "" {
		t.Fatal("verifier id must be stable and non-empty")
	}

	itemsA, err := service.List("env_a")
	if err != nil {
		t.Fatal(err)
	}
	itemsB, err := service.List("env_b")
	if err != nil {
		t.Fatal(err)
	}
	if len(itemsA) != 1 || itemsA[0].ID != added.ID {
		t.Fatalf("environment A verifiers = %+v", itemsA)
	}
	if len(itemsB) != 0 {
		t.Fatalf("environment B must not see environment A verifiers: %+v", itemsB)
	}

	got, err := service.Get("env_a", added.ID)
	if err != nil || got.ID != added.ID {
		t.Fatalf("get verifier = %+v err=%v", got, err)
	}
	removed, err := service.Remove("env_a", added.ID)
	if err != nil || removed.ID != added.ID {
		t.Fatalf("remove verifier = %+v err=%v", removed, err)
	}
}

func TestDefinitionValidationIsLocalAndDoesNotGrantExecutionAuthority(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := store.New(statePath)
	if err := s.Update(func(state *model.State) error {
		state.Environments = append(state.Environments, model.Environment{ID: "env_a", Name: "a"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	service := verifier.New(s)

	for _, test := range []struct {
		name       string
		definition model.VerifierDefinition
		want       string
	}{
		{name: "kind", definition: model.VerifierDefinition{Kind: "unknown", Executable: "go"}, want: "kind"},
		{name: "executable", definition: model.VerifierDefinition{Kind: verifier.KindTest, Executable: "   "}, want: "executable"},
		{name: "timeout", definition: model.VerifierDefinition{Kind: verifier.KindTest, Executable: "go", TimeoutSeconds: -1}, want: "nonnegative"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Add("env_a", test.definition); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid definition error = %v; want substring %q", err, test.want)
			}
		})
	}

	definition, err := service.Add("env_a", model.VerifierDefinition{
		Kind:       " TEST ",
		Enabled:    true,
		Executable: " definitely-not-allowlisted ",
		Cwd:        " subdir ",
		Name:       " checks ",
	})
	if err != nil {
		t.Fatalf("definition creation must not inspect executable allowlist or Git: %v", err)
	}
	if definition.Kind != verifier.KindTest || definition.Executable != "definitely-not-allowlisted" || definition.Cwd != "subdir" || definition.Name != "checks" {
		t.Fatalf("normalized definition = %+v", definition)
	}
}

func TestClassifyRuntimeOutcomes(t *testing.T) {
	definition := model.VerifierDefinition{ID: "vf_1", Kind: verifier.KindTest}

	passed, err := verifier.Classify(definition, runtime.CommandResult{ExitCode: 0, Stdout: "ok\n"}, 12*time.Millisecond, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if passed.Status != verifier.StatusPassed || passed.ExitCode != 0 || passed.TimedOut || passed.ID != definition.ID {
		t.Fatalf("passed result = %+v", passed)
	}

	failed, err := verifier.Classify(definition, runtime.CommandResult{ExitCode: 7, Stderr: "failed\n"}, 13*time.Millisecond, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != verifier.StatusFailed || failed.ExitCode != 7 || failed.TimedOut {
		t.Fatalf("failed result = %+v", failed)
	}

	timedOut, err := verifier.Classify(definition, runtime.CommandResult{Stdout: "partial\n"}, time.Second, true, errors.New("process killed"))
	if err != nil {
		t.Fatal(err)
	}
	if timedOut.Status != verifier.StatusFailed || !timedOut.TimedOut || timedOut.ExitCode != -1 {
		t.Fatalf("timeout result = %+v", timedOut)
	}

	if _, err := verifier.Classify(definition, runtime.CommandResult{}, 2*time.Millisecond, false, errors.New("not allowed")); err == nil || !strings.Contains(err.Error(), "execution failed") {
		t.Fatalf("start/policy error must remain local, got %v", err)
	}
}

func TestVerifierTimeoutUsesRuntimeDefaultWhenUnset(t *testing.T) {
	if got := verifier.Timeout(model.VerifierDefinition{}); got != verifier.DefaultTimeout {
		t.Fatalf("default timeout = %s; want %s", got, verifier.DefaultTimeout)
	}
	if got := verifier.Timeout(model.VerifierDefinition{TimeoutSeconds: 3}); got != 3*time.Second {
		t.Fatalf("explicit timeout = %s", got)
	}
}
