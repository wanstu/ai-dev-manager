package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionRequestValidate(t *testing.T) {
	base := t.TempDir()
	request := ExecutionRequest{
		Scope:        "env:test",
		RuntimeRoot:  filepath.Join(base, "runtime"),
		WorktreeRoot: filepath.Join(base, "worktree"),
		Executable:   filepath.Join(base, "tool.exe"),
		Cwd:          filepath.Join(base, "worktree"),
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid execution request rejected: %v", err)
	}
	for name, mutate := range map[string]func(*ExecutionRequest){
		"empty scope": func(v *ExecutionRequest) { v.Scope = "" },
		"relative runtime": func(v *ExecutionRequest) { v.RuntimeRoot = "runtime" },
		"relative worktree": func(v *ExecutionRequest) { v.WorktreeRoot = "worktree" },
		"relative executable": func(v *ExecutionRequest) { v.Executable = "tool" },
		"relative cwd": func(v *ExecutionRequest) { v.Cwd = "." },
		"negative output": func(v *ExecutionRequest) { v.MaxOutputBytes = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			value := request
			mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("invalid execution request was accepted")
			}
		})
	}
}

func TestValidateToolGrantRejectsBroadRoots(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	if root != string(filepath.Separator) && filepath.VolumeName(root) == "" {
		root = string(filepath.Separator)
	}
	if err := validateToolGrant(root); err == nil {
		t.Fatalf("filesystem root %q was accepted as tool grant", root)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		if err := validateToolGrant(home); err == nil {
			t.Fatalf("user home %q was accepted as tool grant", home)
		}
	}
}

func TestExecuteUnsupportedPlatformFailsClosed(t *testing.T) {
	if PlatformSupport().Supported {
		t.Skip("unsupported-platform behavior only")
	}
	base := t.TempDir()
	worktree := filepath.Join(base, "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(base, "tool")
	if err := os.WriteFile(tool, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(context.Background(), ExecutionRequest{
		Scope:        "test",
		RuntimeRoot:  filepath.Join(base, "runtime"),
		WorktreeRoot: worktree,
		Executable:   tool,
		Cwd:          worktree,
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected fail-closed unsupported-platform error, got %v", err)
	}
}
