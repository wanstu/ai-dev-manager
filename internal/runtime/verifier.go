package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ai-dev-manager/internal/model"
)

type VerifierStatus string

const (
	VerifierPassed  VerifierStatus = "passed"
	VerifierFailed  VerifierStatus = "failed"
	VerifierSkipped VerifierStatus = "skipped"
)

type VerifierResult struct {
	ID       string
	Kind     string
	Status   VerifierStatus
	ExitCode int
	Duration time.Duration
	Summary  string
	Stdout   string
	Stderr   string
	TimedOut bool
}

func (r *Native) RunVerifier(id string) (VerifierResult, error) {
	resolved, ok := r.verifiers[id]
	if !ok {
		return VerifierResult{}, &RuntimeError{Kind: ErrNotFound, Path: id}
	}
	definition := resolved.VerifierDefinition
	if !validVerifierKind(definition.Kind) || strings.TrimSpace(definition.Executable) == "" {
		return VerifierResult{}, &RuntimeError{Kind: ErrInvalidVerifier, Path: id}
	}
	if definition.Enabled != nil && !*definition.Enabled {
		return VerifierResult{
			ID:       definition.ID,
			Kind:     definition.Kind,
			Status:   VerifierSkipped,
			ExitCode: -1,
			Summary:  fmt.Sprintf("%s verifier %q skipped", definition.Kind, definition.ID),
		}, nil
	}

	timeout := time.Duration(0)
	if definition.TimeoutSeconds > 0 {
		timeout = time.Duration(definition.TimeoutSeconds) * time.Second
	}
	commandResult, err := r.Exec(Command{
		Executable: definition.Executable,
		Args:       append([]string(nil), definition.Args...),
		Cwd:        definition.Cwd,
		Timeout:    timeout,
	})
	if err != nil {
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) && runtimeErr.Kind == ErrTimeout {
			result := verifierResultFromCommand(definition, commandResult)
			result.Status = VerifierFailed
			result.TimedOut = true
			result.Summary = fmt.Sprintf("%s verifier %q timed out", definition.Kind, definition.ID)
			return result, nil
		}
		return VerifierResult{}, err
	}

	result := verifierResultFromCommand(definition, commandResult)
	if commandResult.ExitCode == 0 {
		result.Status = VerifierPassed
		result.Summary = fmt.Sprintf("%s verifier %q passed", definition.Kind, definition.ID)
	} else {
		result.Status = VerifierFailed
		result.Summary = fmt.Sprintf("%s verifier %q failed with exit code %d", definition.Kind, definition.ID, commandResult.ExitCode)
	}
	return result, nil
}

func (r *Native) RunVerifiers(ids ...string) ([]VerifierResult, error) {
	selected := append([]string(nil), ids...)
	if len(selected) == 0 {
		selected = make([]string, 0, len(r.verifiers))
		for id := range r.verifiers {
			selected = append(selected, id)
		}
		sort.Strings(selected)
	}
	results := make([]VerifierResult, 0, len(selected))
	for _, id := range selected {
		result, err := r.RunVerifier(id)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func verifierResultFromCommand(definition model.VerifierDefinition, command CommandResult) VerifierResult {
	return VerifierResult{
		ID:       definition.ID,
		Kind:     definition.Kind,
		ExitCode: command.ExitCode,
		Duration: command.Duration,
		Stdout:   command.Stdout,
		Stderr:   command.Stderr,
		TimedOut: command.TimedOut,
	}
}

func validVerifierKind(kind string) bool {
	switch kind {
	case "test", "lint", "build", "custom":
		return true
	default:
		return false
	}
}
