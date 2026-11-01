package desktop

import (
	"errors"
	"strings"
	"testing"
)

func TestActionableSandboxACLBrokerErrorPreservesCauseAndGuidance(t *testing.T) {
	err := actionableSandboxACLBrokerError(errSandboxACLBrokerUnavailable)
	if !errors.Is(err, errSandboxACLBrokerUnavailable) {
		t.Fatalf("wrapped error must preserve broker unavailable cause: %v", err)
	}
	for _, want := range []string{"ADM Desktop Settings", "repair Windows strong sandbox Host Prep", "administrator confirmation"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing guidance %q", err, want)
		}
	}
}

func TestActionableSandboxACLBrokerErrorLeavesOtherErrorsUntouched(t *testing.T) {
	original := errors.New("different failure")
	if got := actionableSandboxACLBrokerError(original); got != original {
		t.Fatalf("non-broker error changed: got %v want %v", got, original)
	}
}
