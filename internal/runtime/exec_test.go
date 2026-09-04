package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-dev-manager/internal/model"
)

func TestExecPolicyMatrixAndToolPaths(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	root := t.TempDir()

	readOnly := mustNative(t, root, model.Policy{Mode: string(ModeReadOnly)})
	_, err = readOnly.Exec(Command{Executable: "helper"})
	assertRuntimeErrorKind(t, err, ErrExecutionDenied)

	workspaceWrite := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})
	_, err = workspaceWrite.Exec(Command{Executable: "helper"})
	assertRuntimeErrorKind(t, err, ErrExecutionDenied)

	standard := mustNative(t, root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	})
	_, err = standard.Exec(Command{Executable: "blocked"})
	assertRuntimeErrorKind(t, err, ErrToolNotAllowed)

	result, err := standard.Exec(Command{
		Executable: "helper",
		Args:       helperArgs("echo", "toolpaths-ok"),
	})
	if err != nil {
		t.Fatalf("Exec(helper) error = %v", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "toolpaths-ok" {
		t.Fatalf("unexpected helper result: %+v", result)
	}
	if filepath.Clean(result.Executable) != filepath.Clean(helper) {
		t.Fatalf("ToolPaths was not used: got %q want %q", result.Executable, helper)
	}
}

func TestFullModeExecDoesNotRequireAllowlist(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	runtime := mustNative(t, t.TempDir(), model.Policy{
		Mode:      string(ModeFull),
		ToolPaths: map[string]string{"helper": helper},
	})
	result, err := runtime.Exec(Command{Executable: "helper", Args: helperArgs("echo", "full-ok")})
	if err != nil {
		t.Fatalf("Full Exec() error = %v", err)
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "full-ok" {
		t.Fatalf("unexpected Full result: %+v", result)
	}
}

func TestExecRejectsOutsideCwdAndReportsNonZeroExit(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root error = %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll outside error = %v", err)
	}
	runtime := mustNative(t, root, model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	})

	_, err = runtime.Exec(Command{Executable: "helper", Args: helperArgs("echo", "x"), Cwd: outside})
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)

	result, err := runtime.Exec(Command{Executable: "helper", Args: helperArgs("exit", "7")})
	if err != nil {
		t.Fatalf("non-zero exit should be a result, got error = %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7; result=%+v", result.ExitCode, result)
	}
}

func TestExecTimeoutAndOutputLimit(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	runtime := mustNative(t, t.TempDir(), model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": helper},
	})

	result, err := runtime.Exec(Command{
		Executable: "helper",
		Args:       helperArgs("sleep", "300"),
		Timeout:    40 * time.Millisecond,
	})
	assertRuntimeErrorKind(t, err, ErrTimeout)
	if !result.TimedOut {
		t.Fatalf("TimedOut = false, result=%+v", result)
	}

	result, err = runtime.Exec(Command{
		Executable:     "helper",
		Args:           helperArgs("spam", "4096"),
		MaxOutputBytes: 128,
	})
	if err != nil {
		t.Fatalf("Exec(spam) error = %v", err)
	}
	if !result.StdoutTruncated || len(result.Stdout) != 128 {
		t.Fatalf("output limit not enforced: len=%d truncated=%v", len(result.Stdout), result.StdoutTruncated)
	}
}

func TestExecMissingConfiguredToolIsStructuredError(t *testing.T) {
	runtime := mustNative(t, t.TempDir(), model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"missing"},
		ToolPaths:          map[string]string{"missing": filepath.Join(t.TempDir(), "missing.exe")},
	})
	_, err := runtime.Exec(Command{Executable: "missing"})
	assertRuntimeErrorKind(t, err, ErrToolNotFound)
}

func TestRuntimeHelperProcess(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "--" {
			marker = i
			break
		}
	}
	if marker < 0 || marker+1 >= len(os.Args) {
		return
	}
	args := os.Args[marker+1:]
	switch args[0] {
	case "echo":
		if len(args) > 1 {
			fmt.Print(args[1])
		}
		os.Exit(0)
	case "exit":
		code := 1
		if len(args) > 1 && args[1] == "7" {
			code = 7
		}
		os.Exit(code)
	case "sleep":
		duration := 300 * time.Millisecond
		if len(args) > 1 && args[1] == "1500" {
			duration = 1500 * time.Millisecond
		}
		time.Sleep(duration)
		fmt.Print("done")
		os.Exit(0)
	case "spam":
		fmt.Print(strings.Repeat("x", 4096))
		os.Exit(0)
	}
}

func helperArgs(mode string, values ...string) []string {
	args := []string{"-test.run=^TestRuntimeHelperProcess$", "--", mode}
	return append(args, values...)
}
