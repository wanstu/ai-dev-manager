package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ai-dev-manager-v2/internal/runtime"
)

func (s *Service) GlobalRuntime() (*runtime.Runtime, error) {
	state, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(filepath.Dir(s.Store.Path()), "global-runtime")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if s.HostEnvironment != nil {
		return runtime.NewWithExecutionPolicy(root, state.AllowedExecutables, state.BlockedExecutables, s.HostEnvironment.Environ(), state.ExecFullAuthorization)
	}
	return runtime.NewWithExecutionPolicy(root, state.AllowedExecutables, state.BlockedExecutables, nil, state.ExecFullAuthorization)
}

func (s *Service) GlobalExec(ctx context.Context, executable string, args []string, timeoutMS int64, maxOutputBytes int) (runtime.CommandResult, error) {
	rt, err := s.GlobalRuntime()
	if err != nil {
		return runtime.CommandResult{}, err
	}
	s.recordFullAuthorizationBypass(rt, "", executable, "global_exec")
	s.Log("info", "global_exec.start", map[string]string{"executable": executable, "surface": "global_exec"})
	result, err := rt.Exec(ctx, executable, args, "", timeoutMS, maxOutputBytes)
	if result.Started {
		_ = s.RecordExecUsage("", executable, "global_exec")
	}
	if err != nil {
		s.Log("error", "global_exec.failed", map[string]string{"executable": executable, "surface": "global_exec", "error": err.Error()})
	} else {
		s.Log("info", "global_exec.completed", map[string]string{"executable": executable, "surface": "global_exec", "exit_code": fmt.Sprint(result.ExitCode)})
	}
	if isExecutableNotAllowedError(err) {
		s.recordExecDenial("", executable, "global_exec", err.Error())
	}
	return result, err
}
