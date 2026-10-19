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
	"ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/sandbox"
	"ai-dev-manager-v2/internal/verifier"
)

const verifierExecutionModeSandbox = "sandbox"

func defaultSandboxRoot(statePath string) string {
	root := filepath.Join(filepath.Dir(statePath), "sandbox-runtime")
	if absolute, err := filepath.Abs(root); err == nil {
		return absolute
	}
	return filepath.Clean(root)
}

func (s *Service) SandboxVerifierSupport() sandbox.Support {
	return sandbox.PlatformSupport()
}

func (s *Service) SandboxVerifierAvailable() bool {
	support := s.SandboxVerifierSupport()
	return support.Supported && support.StrongFileBoundarySupported && support.ProcessTreeBoundary
}

// RunPreparedVerifierSandbox executes an already-authorized verifier command
// inside the strong OS sandbox. It never falls back to host execution.
func (s *Service) RunPreparedVerifierSandbox(
	ctx context.Context,
	prepared PreparedVerifierExecution,
	executionID string,
	maxOutputBytes int,
	stdout, stderr io.Writer,
) (runtime.CommandResult, error) {
	if !s.SandboxVerifierAvailable() {
		support := s.SandboxVerifierSupport()
		return runtime.CommandResult{}, fmt.Errorf("strong verifier sandbox is unavailable on %s", support.Platform)
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
	}

	result, err := sandbox.Execute(ctx, sandbox.ExecutionRequest{
		Scope:            prepared.Environment.ID + ":" + executionID,
		RuntimeRoot:      filepath.Join(s.SandboxRoot, prepared.Environment.ID, executionID),
		WorktreeRoot:     prepared.Environment.Root,
		Executable:       prepared.ExecutablePath,
		Args:             append([]string(nil), prepared.Definition.Args...),
		Cwd:              prepared.CwdPath,
		BaseEnvironment:  baseEnvironment,
		ExtraEnvironment: extraEnvironment,
		ToolRoots:        verifierSandboxToolRoots(baseEnvironment, prepared.ExecutablePath),
		MaxOutputBytes:   normalizeVerifierOutputLimit(maxOutputBytes),
		Stdout:           stdout,
		Stderr:           stderr,
	})
	command := runtime.CommandResult{
		Started:  result.Started,
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}
	if command.Started {
		_ = s.RecordExecUsage(prepared.Environment.ID, prepared.Definition.Executable, "verifier_sandbox")
	}
	return command, err
}

func (s *Service) RunVerifierSandboxed(ctx context.Context, environmentID, owner, verifierID string, maxOutputBytes int) (verifier.Result, error) {
	prepared, err := s.PrepareVerifierExecution(ctx, environmentID, owner, verifierID)
	if err != nil {
		return verifier.Result{}, err
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

	verifierCtx, timeoutCancel := context.WithTimeout(commandCtx, prepared.Timeout)
	started := time.Now()
	commandResult, execErr := s.RunPreparedVerifierSandbox(verifierCtx, prepared, "", maxOutputBytes, nil, nil)
	duration := time.Since(started)
	callerInterrupted := ctx.Err() != nil && (execErr != nil || commandResult.ExitCode != 0)
	timedOut := !callerInterrupted && verifierCtx.Err() == context.DeadlineExceeded
	timeoutCancel()
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return verifier.Result{}, fmt.Errorf("writer heartbeat failed: %w", heartbeatErr)
	}
	if callerInterrupted {
		return verifier.Result{}, BlockingVerifierRequestInterruptedError{}
	}

	result, classifyErr := verifier.Classify(prepared.Definition, commandResult, duration, timedOut, execErr)
	if classifyErr != nil {
		return verifier.Result{}, classifyErr
	}
	result.ExecutionMode = verifierExecutionModeSandbox
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return verifier.Result{}, err
	}
	return result, nil
}

func normalizeVerifierOutputLimit(value int) int {
	if value <= 0 {
		return 120000
	}
	if value > 4*1024*1024 {
		return 4 * 1024 * 1024
	}
	return value
}

func verifierSandboxToolRoots(environment []string, executable string) []string {
	seen := map[string]struct{}{}
	var roots []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || !filepath.IsAbs(path) {
			return
		}
		resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
		if err != nil {
			return
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return
		}
		path = filepath.Clean(resolved)
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		roots = append(roots, path)
	}

	executable = strings.TrimSpace(executable)
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable)))
	switch name {
	case "go":
		add(verifierSandboxGoRoot(environment, executable))
	case "java", "javac":
		add(environmentLookup(environment, "JAVA_HOME"))
	case "dotnet":
		add(environmentLookup(environment, "DOTNET_ROOT"))
	case "mvn":
		add(environmentLookup(environment, "MAVEN_HOME"))
	case "gradle", "gradlew":
		add(environmentLookup(environment, "GRADLE_HOME"))
	}
	return roots
}

func verifierSandboxGoRoot(environment []string, executable string) string {
	executable = strings.TrimSpace(executable)
	if strings.ToLower(strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))) != "go" {
		return ""
	}
	candidates := []string{environmentLookup(environment, "GOROOT")}
	if filepath.IsAbs(executable) {
		candidates = append(candidates, filepath.Dir(filepath.Dir(executable)))
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !filepath.IsAbs(candidate) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err == nil && info.IsDir() {
			return filepath.Clean(resolved)
		}
	}
	return ""
}

func environmentLookup(environment []string, key string) string {
	for _, item := range environment {
		index := strings.IndexByte(item, '=')
		if index <= 0 || !strings.EqualFold(strings.TrimSpace(item[:index]), key) {
			continue
		}
		return strings.TrimSpace(item[index+1:])
	}
	return ""
}
