package app

import (
	"context"
	"fmt"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/verifier"
)

// PreparedVerifierExecution is the shared verifier authority snapshot used by
// both blocking execution and Gateway-owner asynchronous verifier runs. It is
// intentionally process-local and contains no persisted runtime observation.
type PreparedVerifierExecution struct {
	Definition model.VerifierDefinition
	Runtime    *runtime.Runtime
	Timeout    time.Duration
}

const BlockingVerifierRequestInterruptedKind = "blocking_request_interrupted"

type BlockingVerifierRequestInterruptedError struct{}

func (BlockingVerifierRequestInterruptedError) Error() string {
	return BlockingVerifierRequestInterruptedKind + ": blocking verifier request was interrupted by its caller; use environment_verifier_run_start for long or heavy verification"
}

// PrepareVerifierExecution resolves and validates one verifier through the
// existing Environment writer and Runtime authority. PrepareCommand is called
// before returning so forbidden executables and escaped cwd values fail before
// an asynchronous owner resource can be installed.
func (s *Service) PrepareVerifierExecution(ctx context.Context, environmentID, owner, verifierID string) (PreparedVerifierExecution, error) {
	definition, err := s.Verifiers.Get(environmentID, verifierID)
	if err != nil {
		return PreparedVerifierExecution{}, err
	}
	if !definition.Enabled {
		return PreparedVerifierExecution{}, fmt.Errorf("verifier %q is disabled", verifierID)
	}
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return PreparedVerifierExecution{}, err
	}
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return PreparedVerifierExecution{}, err
	}
	s.recordFullAuthorizationBypass(rt, environmentID, definition.Executable, "verifier")
	if _, err := rt.PrepareCommand(ctx, definition.Executable, definition.Args, definition.Cwd); err != nil {
		if isExecutableNotAllowedError(err) {
			s.recordExecDenial(environmentID, definition.Executable, "verifier", err.Error())
		}
		return PreparedVerifierExecution{}, err
	}
	return PreparedVerifierExecution{
		Definition: definition,
		Runtime:    rt,
		Timeout:    verifier.Timeout(definition),
	}, nil
}
