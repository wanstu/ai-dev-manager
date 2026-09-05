package gateway

import (
	"context"
	"errors"
	"strings"

	"ai-dev-manager/internal/environment"
)

type DomainError struct {
	Code          string                `json:"code"`
	Message       string                `json:"message"`
	EnvironmentID string                `json:"environment_id,omitempty"`
	Facts         []environment.Fact    `json:"facts,omitempty"`
	Warnings      []environment.Warning `json:"warnings,omitempty"`
	Hints         []environment.Hint    `json:"hints,omitempty"`
}

func (s *Service) DomainError(ctx context.Context, err error, env *environment.Environment, inspection *environment.InspectResult) DomainError {
	result := DomainError{Code: "gateway_operation_failed", Message: "Gateway operation failed."}
	var envErr *environment.Error
	if errors.As(err, &envErr) {
		result.Code = string(envErr.Code)
		result.EnvironmentID = strings.TrimSpace(envErr.EnvironmentID)
		if message := strings.TrimSpace(envErr.Message); message != "" {
			result.Message = message
		} else {
			result.Message = string(envErr.Code)
		}
	}
	if env != nil {
		if result.EnvironmentID == "" {
			result.EnvironmentID = env.ID
		}
		appendWriterFacts(&result, *env)
	}
	if inspection != nil {
		copyInspectionFacts(&result, *inspection)
	}
	if inspection == nil && result.EnvironmentID != "" {
		if inspected, inspectErr := s.InspectEnvironment(ctx, result.EnvironmentID); inspectErr == nil {
			copyInspectionFacts(&result, inspected)
		}
	}
	return result
}

func copyInspectionFacts(target *DomainError, inspection environment.InspectResult) {
	target.Facts = append([]environment.Fact(nil), inspection.Facts...)
	target.Warnings = append([]environment.Warning(nil), inspection.Warnings...)
	target.Hints = append([]environment.Hint(nil), inspection.Hints...)
	appendWriterFacts(target, inspection.Environment)
}

func appendWriterFacts(target *DomainError, env environment.Environment) {
	if env.Writer == nil {
		return
	}
	for _, fact := range target.Facts {
		if fact.Code == "writer_owner" {
			return
		}
	}
	target.Facts = append(target.Facts,
		environment.Fact{Code: "writer_active", Message: "Environment currently has a writer lease", Value: true},
		environment.Fact{Code: "writer_owner", Message: "Current Environment writer owner", Value: env.Writer.Owner},
	)
}
