package verifier

import (
	"fmt"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/store"
)

const (
	KindTest   = "test"
	KindLint   = "lint"
	KindBuild  = "build"
	KindCustom = "custom"

	StatusPassed  = "passed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"

	DefaultTimeout = 30 * time.Second
)

type Service struct {
	store *store.Store
}

type Result struct {
	ID         string `json:"verifier_id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	TimedOut   bool   `json:"timed_out"`
	Summary    string `json:"summary,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
}

func New(s *store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) List(environmentID string) ([]model.VerifierDefinition, error) {
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	idx := findEnvironment(state.Environments, environmentID)
	if idx < 0 {
		return nil, fmt.Errorf("environment %q not found", strings.TrimSpace(environmentID))
	}
	definitions := state.Environments[idx].Verifiers
	items := make([]model.VerifierDefinition, len(definitions))
	for i := range definitions {
		items[i] = cloneDefinition(definitions[i])
	}
	return items, nil
}

func (s *Service) Get(environmentID, verifierID string) (model.VerifierDefinition, error) {
	state, err := s.store.Load()
	if err != nil {
		return model.VerifierDefinition{}, err
	}
	idx := findEnvironment(state.Environments, environmentID)
	if idx < 0 {
		return model.VerifierDefinition{}, fmt.Errorf("environment %q not found", strings.TrimSpace(environmentID))
	}
	verifierID = strings.TrimSpace(verifierID)
	for _, definition := range state.Environments[idx].Verifiers {
		if definition.ID == verifierID {
			return cloneDefinition(definition), nil
		}
	}
	return model.VerifierDefinition{}, fmt.Errorf("verifier %q not found for environment %q", verifierID, strings.TrimSpace(environmentID))
}

func (s *Service) Add(environmentID string, definition model.VerifierDefinition) (model.VerifierDefinition, error) {
	environmentID = strings.TrimSpace(environmentID)
	definition, err := normalizeDefinition(definition)
	if err != nil {
		return model.VerifierDefinition{}, err
	}
	definition.ID = ""

	var result model.VerifierDefinition
	err = s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, environmentID)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", environmentID)
		}
		id, err := identity.New("vf")
		if err != nil {
			return err
		}
		definition.ID = id
		result = cloneDefinition(definition)
		env := &state.Environments[idx]
		env.Verifiers = append(env.Verifiers, cloneDefinition(definition))
		env.UpdatedAt = time.Now().UTC()
		return nil
	})
	return result, err
}

func (s *Service) Remove(environmentID, verifierID string) (model.VerifierDefinition, error) {
	environmentID = strings.TrimSpace(environmentID)
	verifierID = strings.TrimSpace(verifierID)
	var removed model.VerifierDefinition
	err := s.store.Update(func(state *model.State) error {
		idx := findEnvironment(state.Environments, environmentID)
		if idx < 0 {
			return fmt.Errorf("environment %q not found", environmentID)
		}
		env := &state.Environments[idx]
		for i, definition := range env.Verifiers {
			if definition.ID != verifierID {
				continue
			}
			removed = cloneDefinition(definition)
			env.Verifiers = append(env.Verifiers[:i], env.Verifiers[i+1:]...)
			env.UpdatedAt = time.Now().UTC()
			return nil
		}
		return fmt.Errorf("verifier %q not found for environment %q", verifierID, environmentID)
	})
	return removed, err
}

func Timeout(definition model.VerifierDefinition) time.Duration {
	if definition.TimeoutSeconds > 0 {
		return time.Duration(definition.TimeoutSeconds) * time.Second
	}
	return DefaultTimeout
}

func Classify(definition model.VerifierDefinition, command runtime.CommandResult, duration time.Duration, timedOut bool, execErr error) (Result, error) {
	if duration < 0 {
		duration = 0
	}
	result := Result{
		ID:         definition.ID,
		Kind:       definition.Kind,
		ExitCode:   command.ExitCode,
		DurationMs: duration.Milliseconds(),
		TimedOut:   timedOut,
		Stdout:     command.Stdout,
		Stderr:     command.Stderr,
	}
	if timedOut {
		result.Status = StatusFailed
		result.ExitCode = -1
		result.Summary = "verifier timed out"
		return result, nil
	}
	if execErr != nil {
		return Result{}, fmt.Errorf("verifier %q execution failed: %w", definition.ID, execErr)
	}
	if command.ExitCode == 0 {
		result.Status = StatusPassed
		result.Summary = "verifier passed"
		return result, nil
	}
	result.Status = StatusFailed
	result.Summary = fmt.Sprintf("verifier exited with code %d", command.ExitCode)
	return result, nil
}

func normalizeDefinition(definition model.VerifierDefinition) (model.VerifierDefinition, error) {
	definition = cloneDefinition(definition)
	definition.Kind = strings.ToLower(strings.TrimSpace(definition.Kind))
	switch definition.Kind {
	case KindTest, KindLint, KindBuild, KindCustom:
	default:
		return model.VerifierDefinition{}, fmt.Errorf("verifier kind must be one of test, lint, build, custom")
	}
	definition.Executable = strings.TrimSpace(definition.Executable)
	if definition.Executable == "" {
		return model.VerifierDefinition{}, fmt.Errorf("verifier executable is required")
	}
	if definition.TimeoutSeconds < 0 {
		return model.VerifierDefinition{}, fmt.Errorf("verifier timeout_seconds must be nonnegative")
	}
	definition.Cwd = strings.TrimSpace(definition.Cwd)
	definition.Name = strings.TrimSpace(definition.Name)
	return definition, nil
}

func findEnvironment(environments []model.Environment, id string) int {
	id = strings.TrimSpace(id)
	for i := range environments {
		if environments[i].ID == id {
			return i
		}
	}
	return -1
}

func cloneDefinition(definition model.VerifierDefinition) model.VerifierDefinition {
	definition.Args = append([]string(nil), definition.Args...)
	return definition
}
