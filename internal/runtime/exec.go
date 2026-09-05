package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultCommandTimeout = 30 * time.Second
	hardCommandTimeout    = 10 * time.Minute
	defaultOutputBytes    = 1 << 20
	hardOutputBytes       = 10 << 20
	hardInputBytes        = 20 << 20
)

type Command struct {
	Executable     string
	Args           []string
	Cwd            string
	Stdin          []byte
	Timeout        time.Duration
	MaxOutputBytes int
}

type CommandResult struct {
	Executable      string
	Args            []string
	Cwd             string
	ExitCode        int
	Stdout          string
	Stderr          string
	Duration        time.Duration
	TimedOut        bool
	StdoutTruncated bool
	StderrTruncated bool
}

func (r *Native) Exec(command Command) (CommandResult, error) {
	if !r.canExec() {
		return CommandResult{}, &RuntimeError{Kind: ErrExecutionDenied}
	}
	if strings.TrimSpace(command.Executable) == "" {
		return CommandResult{}, &RuntimeError{Kind: ErrToolNotFound, Tool: command.Executable}
	}
	if r.Mode() == ModeStandard && !r.allowedExecutable(command.Executable) {
		return CommandResult{}, &RuntimeError{Kind: ErrToolNotAllowed, Tool: command.Executable}
	}

	resolvedExecutable, err := r.resolveExecutable(command.Executable)
	if err != nil {
		return CommandResult{}, err
	}
	cwd := r.root
	if strings.TrimSpace(command.Cwd) != "" {
		resolvedCwd, _, guardErr := r.guard.Existing(command.Cwd)
		if guardErr != nil {
			return CommandResult{}, guardErr
		}
		info, statErr := os.Stat(resolvedCwd)
		if statErr != nil || !info.IsDir() {
			return CommandResult{}, &RuntimeError{Kind: ErrInvalidPath, Path: command.Cwd, Err: statErr}
		}
		cwd = resolvedCwd
	}

	timeout := command.Timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	if timeout > hardCommandTimeout {
		return CommandResult{}, &RuntimeError{Kind: ErrLimitExceeded}
	}
	outputLimit := command.MaxOutputBytes
	if outputLimit <= 0 {
		outputLimit = defaultOutputBytes
	}
	if outputLimit > hardOutputBytes || len(command.Stdin) > hardInputBytes {
		return CommandResult{}, &RuntimeError{Kind: ErrLimitExceeded}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stdout := &limitedBuffer{limit: outputLimit}
	stderr := &limitedBuffer{limit: outputLimit}
	cmd := exec.CommandContext(ctx, resolvedExecutable, command.Args...)
	cmd.Dir = cwd
	if len(command.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(command.Stdin)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)
	result := CommandResult{
		Executable:      resolvedExecutable,
		Args:            append([]string(nil), command.Args...),
		Cwd:             cwd,
		ExitCode:        -1,
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		Duration:        duration,
		TimedOut:        errors.Is(ctx.Err(), context.DeadlineExceeded),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if result.TimedOut {
		return result, &RuntimeError{Kind: ErrTimeout}
	}
	if runErr == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return result, nil
	}
	return result, &RuntimeError{Kind: ErrIO, Path: resolvedExecutable, Err: runErr}
}

func (r *Native) allowedExecutable(requested string) bool {
	base := filepath.Base(requested)
	for _, allowed := range r.policy.AllowedExecutables {
		if strings.EqualFold(strings.TrimSpace(allowed), requested) || strings.EqualFold(strings.TrimSpace(allowed), base) {
			return true
		}
	}
	return false
}

func (r *Native) resolveExecutable(requested string) (string, error) {
	if configured, ok := lookupFold(r.policy.ToolPaths, requested); ok {
		if !filepath.IsAbs(configured) {
			return "", &RuntimeError{Kind: ErrToolNotFound, Tool: requested}
		}
		info, err := os.Stat(configured)
		if err != nil || info.IsDir() {
			return "", &RuntimeError{Kind: ErrToolNotFound, Tool: requested, Err: err}
		}
		return filepath.Clean(configured), nil
	}
	if base := filepath.Base(requested); base != requested {
		if configured, ok := lookupFold(r.policy.ToolPaths, base); ok {
			if !filepath.IsAbs(configured) {
				return "", &RuntimeError{Kind: ErrToolNotFound, Tool: requested}
			}
			info, err := os.Stat(configured)
			if err != nil || info.IsDir() {
				return "", &RuntimeError{Kind: ErrToolNotFound, Tool: requested, Err: err}
			}
			return filepath.Clean(configured), nil
		}
	}
	resolved, err := exec.LookPath(requested)
	if err != nil {
		return "", &RuntimeError{Kind: ErrToolNotFound, Tool: requested, Err: err}
	}
	return resolved, nil
}

func lookupFold(values map[string]string, key string) (string, bool) {
	for candidate, value := range values {
		if strings.EqualFold(candidate, key) || strings.EqualFold(candidate, filepath.Base(key)) {
			return value, true
		}
	}
	return "", false
}

type limitedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			b.data = append(b.data, p[:remaining]...)
			b.truncated = true
		} else {
			b.data = append(b.data, p...)
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return originalLen, nil
}

func (b *limitedBuffer) String() string { return string(b.data) }

var _ io.Writer = (*limitedBuffer)(nil)
