package app

import (
	"fmt"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
)

type ExecAuthorizationStatus struct {
	FullAuthorization bool `json:"full_authorization"`
}

func (s *Service) ExecAuthorizationStatus() (ExecAuthorizationStatus, error) {
	state, err := s.Store.Load()
	if err != nil {
		return ExecAuthorizationStatus{}, err
	}
	return ExecAuthorizationStatus{FullAuthorization: state.ExecFullAuthorization}, nil
}

func (s *Service) ExecFullAuthorizationSet(enabled bool) (ExecAuthorizationStatus, error) {
	return s.SetExecFullAuthorization(enabled)
}

func (s *Service) SetExecFullAuthorization(enabled bool) (ExecAuthorizationStatus, error) {
	if err := s.Store.Update(func(state *model.State) error {
		state.ExecFullAuthorization = enabled
		return nil
	}); err != nil {
		return ExecAuthorizationStatus{}, err
	}
	return ExecAuthorizationStatus{FullAuthorization: enabled}, nil
}

func (s *Service) RecordFullAuthorizationBypass(rt *runtime.Runtime, environmentID, executable, surface string) {
	s.recordFullAuthorizationBypass(rt, environmentID, executable, surface)
}

func (s *Service) recordFullAuthorizationBypass(rt *runtime.Runtime, environmentID, executable, surface string) {
	if rt == nil || !rt.FullAuthorizationEnabled() || rt.IsExplicitlyAllowed(executable) {
		return
	}
	_ = s.RecordExecDenial(
		environmentID,
		executable,
		surface,
		fmt.Sprintf("full_authorization_bypass: executable %q is not allowed by the explicit allowlist; execution was permitted by full authorization", executable),
	)
}
