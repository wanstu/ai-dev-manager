package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	runtimepkg "ai-dev-manager-v2/internal/runtime"
	"ai-dev-manager-v2/internal/sandbox"
	"ai-dev-manager-v2/internal/verifier"
)

const sandboxVerifierActiveEnv = "ADM_SANDBOX_ACTIVE"

type PreparedSandboxVerifierExecution struct {
	Definition model.VerifierDefinition
	Timeout    time.Duration
	Request    sandbox.ExecutionRequest
}

func (s *Service) SandboxVerifierAvailable() bool {
	support := sandbox.PlatformSupport()
	return support.Supported && support.StrongFileBoundarySupported && support.ProcessTreeBoundary
}

func (s *Service) PrepareSandboxVerifierExecution(ctx context.Context, environmentID, owner, verifierID string) (PreparedSandboxVerifierExecution, error) {
	if !s.SandboxVerifierAvailable() {
		support := sandbox.PlatformSupport()
		return PreparedSandboxVerifierExecution{}, fmt.Errorf("strong sandbox verifier execution is unavailable on %s", support.Platform)
	}
	prepared, err := s.PrepareVerifierExecution(ctx, environmentID, owner, verifierID)
	if err != nil {
		return PreparedSandboxVerifierExecution{}, err
	}
	cmd, err := prepared.Runtime.PrepareCommand(ctx, prepared.Definition.Executable, prepared.Definition.Args, prepared.Definition.Cwd)
	if err != nil {
		return PreparedSandboxVerifierExecution{}, err
	}
	scopeID, err := identity.New("sbx")
	if err != nil {
		return PreparedSandboxVerifierExecution{}, err
	}
	args := append([]string(nil), prepared.Definition.Args...)
	return PreparedSandboxVerifierExecution{
		Definition: prepared.Definition,
		Timeout:    prepared.Timeout,
		Request: sandbox.ExecutionRequest{
			Scope:           environmentID + ":" + verifierID + ":" + scopeID,
			WorktreeRoot:    prepared.Runtime.Root(),
			Executable:      cmd.Path,
			Args:            args,
			Cwd:             cmd.Dir,
			BaseEnvironment: append([]string(nil), cmd.Env...),
			ReadOnlyRoots:   sandboxVerifierToolRoots(cmd.Env, cmd.Path),
			Environment: map[string]string{
				sandboxVerifierActiveEnv: "1",
			},
		},
	}, nil
}

func (s *Service) RunVerifierSandboxed(ctx context.Context, environmentID, owner, verifierID string, maxOutputBytes int) (verifier.Result, error) {
	prepared, err := s.PrepareSandboxVerifierExecution(ctx, environmentID, owner, verifierID)
	if err != nil {
		return verifier.Result{}, err
	}
	if maxOutputBytes <= 0 {
		maxOutputBytes = 120000
	}
	if maxOutputBytes > 4*1024*1024 {
		maxOutputBytes = 4 * 1024 * 1024
	}
	stdout := newSandboxVerifierOutputBuffer(maxOutputBytes)
	stderr := newSandboxVerifierOutputBuffer(maxOutputBytes)
	prepared.Request.Stdout = stdout
	prepared.Request.Stderr = stderr

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
	launchResult, execErr := sandbox.Execute(verifierCtx, prepared.Request)
	duration := time.Since(started)
	commandResult := runtimepkg.CommandResult{
		Started:  launchResult.Started,
		ExitCode: launchResult.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if commandResult.Started {
		_ = s.RecordExecUsage(environmentID, prepared.Definition.Executable, "verifier_sandbox")
	}

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
	if err := s.Environments.Touch(environmentID, owner); err != nil {
		return verifier.Result{}, err
	}
	return result, nil
}

func sandboxVerifierToolRoots(environment []string, executable string) []string {
	systemRoots := []string{
		sandboxEnvironmentValue(environment, "SYSTEMROOT"),
		sandboxEnvironmentValue(environment, "WINDIR"),
		sandboxEnvironmentValue(environment, "ProgramFiles"),
		sandboxEnvironmentValue(environment, "ProgramFiles(x86)"),
	}
	candidates := []string{
		sandboxEnvironmentValue(environment, "GOROOT"),
		sandboxEnvironmentValue(environment, "JAVA_HOME"),
		sandboxEnvironmentValue(environment, "MAVEN_HOME"),
		sandboxEnvironmentValue(environment, "GRADLE_HOME"),
		sandboxEnvironmentValue(environment, "DOTNET_ROOT"),
	}
	executable = strings.TrimSpace(executable)
	if filepath.IsAbs(executable) {
		name := strings.ToLower(strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable)))
		if name == "go" {
			binDir := filepath.Dir(executable)
			candidates = append(candidates, filepath.Dir(binDir))
		}
	}
	var roots []string
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
		if err != nil || !info.IsDir() {
			continue
		}
		alreadySystemReadable := false
		for _, systemRoot := range systemRoots {
			if sandboxPathWithin(systemRoot, resolved) {
				alreadySystemReadable = true
				break
			}
		}
		if alreadySystemReadable {
			continue
		}
		duplicate := false
		for _, existing := range roots {
			if sandboxSamePath(existing, resolved) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			roots = append(roots, resolved)
		}
	}
	return roots
}

func sandboxEnvironmentValue(environment []string, key string) string {
	for _, item := range environment {
		index := strings.IndexByte(item, '=')
		if index <= 0 || !strings.EqualFold(strings.TrimSpace(item[:index]), key) {
			continue
		}
		return strings.TrimSpace(item[index+1:])
	}
	return ""
}

func sandboxPathWithin(root, target string) bool {
	root = strings.TrimSpace(root)
	target = strings.TrimSpace(target)
	if root == "" || target == "" || !filepath.IsAbs(root) || !filepath.IsAbs(target) {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func sandboxSamePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

type sandboxVerifierOutputBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func newSandboxVerifierOutputBuffer(limit int) *sandboxVerifierOutputBuffer {
	if limit <= 0 {
		limit = 120000
	}
	return &sandboxVerifierOutputBuffer{limit: limit}
}

func (b *sandboxVerifierOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return original, nil
}

func (b *sandboxVerifierOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
