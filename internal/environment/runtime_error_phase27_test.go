package environment

import (
	"errors"
	"strings"
	"testing"

	admruntime "ai-dev-manager/internal/runtime"
)

func TestEnvironmentRuntimeErrorIncludesStableRuntimeKind(t *testing.T) {
	cause := &admruntime.RuntimeError{Kind: admruntime.ErrLimitExceeded}
	err := environmentRuntimeError("env_a", `invoke environment read operation "files.read"`, cause)

	if err.Code != ErrRuntime || err.EnvironmentID != "env_a" {
		t.Fatalf("environment error = %+v", err)
	}
	if !strings.Contains(err.Message, "runtime limit exceeded") || !strings.Contains(err.Message, string(admruntime.ErrLimitExceeded)) {
		t.Fatalf("runtime diagnostic message = %q", err.Message)
	}
	var runtimeErr *admruntime.RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Kind != admruntime.ErrLimitExceeded {
		t.Fatalf("runtime cause not preserved: %v", err)
	}
}
