package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/sandbox"
)

// PreparedSandboxExecution is an Environment-scoped command that has already
// passed the same Runtime executable/cwd authority used by host Exec and Runs.
// The prepared command may only be launched through the strong OS sandbox.
type PreparedSandboxExecution struct {
	Environment    model.Environment
	Runtime        *runtime.Runtime
	Executable     string
	ExecutablePath string
	Args           []string
	CwdPath        string
}

func (s *Service) SandboxExecutionSupport() sandbox.Support {
	return sandbox.PlatformSupport()
}

func (s *Service) SandboxExecutionAvailable() bool {
	support := s.SandboxExecutionSupport()
	return support.Supported && support.StrongFileBoundarySupported && support.ProcessTreeBoundary
}

func (s *Service) PrepareSandboxExecution(
	ctx context.Context,
	environmentID, owner, executable string,
	args []string,
	cwd string,
	surface string,
) (PreparedSandboxExecution, error) {
	if _, err := s.Environments.RequireWriter(environmentID, owner); err != nil {
		return PreparedSandboxExecution{}, err
	}
	rt, env, err := s.Runtime(environmentID)
	if err != nil {
		return PreparedSandboxExecution{}, err
	}
	if s.SandboxExecutionSupport().Platform == "windows" {
		if err := s.EnsureEnvironmentSandboxAccess(env); err != nil {
			return PreparedSandboxExecution{}, fmt.Errorf("prepare Environment strong-sandbox access: %w", err)
		}
	}
	s.recordFullAuthorizationBypass(rt, environmentID, executable, surface)
	cmd, err := rt.PrepareCommand(ctx, executable, args, cwd)
	if err != nil {
		if isExecutableNotAllowedError(err) {
			s.recordExecDenial(environmentID, executable, surface, err.Error())
		}
		return PreparedSandboxExecution{}, err
	}
	return PreparedSandboxExecution{
		Environment:    env,
		Runtime:        rt,
		Executable:     executable,
		ExecutablePath: cmd.Path,
		Args:           append([]string(nil), args...),
		CwdPath:        cmd.Dir,
	}, nil
}

// RunPreparedSandboxExecution launches one already-authorized command inside
// the strong OS sandbox. There is intentionally no host fallback.
func (s *Service) RunPreparedSandboxExecution(
	ctx context.Context,
	prepared PreparedSandboxExecution,
	executionID string,
	maxOutputBytes int,
	stdout, stderr io.Writer,
) (runtime.CommandResult, error) {
	if !s.SandboxExecutionAvailable() {
		support := s.SandboxExecutionSupport()
		return runtime.CommandResult{}, fmt.Errorf("strong sandbox execution is unavailable on %s", support.Platform)
	}
	if strings.TrimSpace(s.SandboxRoot) == "" {
		return runtime.CommandResult{}, fmt.Errorf("sandbox runtime root is not configured")
	}
	if strings.TrimSpace(executionID) == "" {
		generated, err := identity.New("sbx")
		if err != nil {
			return runtime.CommandResult{}, err
		}
		executionID = generated
	}

	baseEnvironment := os.Environ()
	if s.HostEnvironment != nil {
		baseEnvironment = s.HostEnvironment.Environ()
	}
	extraEnvironment := map[string]string{
		"ADM_SANDBOX_MODE":   "strong",
		"ADM_SANDBOX_ACTIVE": "1",
	}
	if goRoot := verifierSandboxGoRoot(baseEnvironment, prepared.ExecutablePath); goRoot != "" {
		extraEnvironment["GOROOT"] = goRoot
		extraEnvironment["GOTELEMETRY"] = "off"
		extraEnvironment["GOTOOLCHAIN"] = "local"
	}

	result, err := sandbox.Execute(ctx, sandbox.ExecutionRequest{
		// Keep the AppContainer identity stable per Environment. The execution ID
		// still isolates runtime/cache directories, while the stable SID allows a
		// one-time administrator ACL preparation to be reused safely.
		Scope:            prepared.Environment.ID,
		RuntimeRoot:      filepath.Join(s.SandboxRoot, prepared.Environment.ID, executionID),
		CacheRoot:        filepath.Join(s.SandboxRoot, prepared.Environment.ID, "cache"),
		WorktreeRoot:     prepared.Environment.Root,
		Executable:       prepared.ExecutablePath,
		Args:             append([]string(nil), prepared.Args...),
		Cwd:              prepared.CwdPath,
		BaseEnvironment:  baseEnvironment,
		ExtraEnvironment: extraEnvironment,
		ToolRoots:        verifierSandboxToolRoots(baseEnvironment, prepared.ExecutablePath),
		Capabilities:     sandboxCapabilitiesForCommand(prepared.ExecutablePath, prepared.Args),
		MaxOutputBytes:   normalizeVerifierOutputLimit(maxOutputBytes),
		Stdout:           stdout,
		Stderr:           stderr,
	})
	command := runtime.CommandResult{
		Started:       result.Started,
		ExitCode:      result.ExitCode,
		Stdout:        result.Stdout,
		Stderr:        result.Stderr,
		ExecutionMode: "sandbox",
	}
	if command.Started {
		_ = s.RecordExecUsage(prepared.Environment.ID, prepared.Executable, "sandbox")
	}
	return command, err
}

func (s *Service) ExecSandbox(
	ctx context.Context,
	environmentID, owner, executable string,
	args []string,
	cwd string,
	timeoutMS int64,
	maxOutputBytes int,
) (any, error) {
	prepared, err := s.PrepareSandboxExecution(ctx, environmentID, owner, executable, args, cwd, "exec_sandbox")
	if err != nil {
		return nil, err
	}
	if timeoutMS <= 0 {
		timeoutMS = 30000
	}

	commandCtx, cancel := context.WithCancel(ctx)
	heartbeatDone := make(chan error, 1)
	go func() {
		interval := s.heartbeatInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-commandCtx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if _, heartbeatErr := s.Environments.HeartbeatWriter(environmentID, owner); heartbeatErr != nil {
					heartbeatDone <- heartbeatErr
					cancel()
					return
				}
			}
		}
	}()

	s.Log("info", "exec.start", map[string]string{
		"environment_id": environmentID,
		"executable":     executable,
		"surface":        "exec",
		"execution_mode": "sandbox",
	})
	execCtx, timeoutCancel := context.WithTimeout(commandCtx, time.Duration(timeoutMS)*time.Millisecond)
	result, execErr := s.RunPreparedSandboxExecution(execCtx, prepared, "", maxOutputBytes, nil, nil)
	timedOut := execCtx.Err() == context.DeadlineExceeded
	timeoutCancel()
	cancel()
	heartbeatErr := <-heartbeatDone

	if heartbeatErr != nil {
		return nil, fmt.Errorf("writer heartbeat failed: %w", heartbeatErr)
	}
	if timedOut {
		if execErr != nil && execErr.Error() != context.DeadlineExceeded.Error() {
			return nil, fmt.Errorf("command timed out after %dms: %v", timeoutMS, execErr)
		}
		return nil, fmt.Errorf("command timed out after %dms", timeoutMS)
	}
	if execErr != nil {
		s.Log("error", "exec.failed", map[string]string{
			"environment_id": environmentID,
			"executable":     executable,
			"surface":        "exec",
			"execution_mode": "sandbox",
			"error":          execErr.Error(),
		})
		return nil, execErr
	}
	s.Log("info", "exec.completed", map[string]string{
		"environment_id": environmentID,
		"executable":     executable,
		"surface":        "exec",
		"execution_mode": "sandbox",
		"exit_code":      fmt.Sprint(result.ExitCode),
	})
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return nil, err
	}
	return result, nil
}
