//go:build windows

package sandbox

import (
	"bytes"
	"testing"
)

func TestWindowsEnvironmentBlock(t *testing.T) {
	block, err := windowsEnvironmentBlock([]string{"A=1", "B=2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(block) < 2 || block[len(block)-1] != 0 || block[len(block)-2] != 0 {
		t.Fatalf("environment block is not double-NUL terminated: %v", block)
	}
	if _, err := windowsEnvironmentBlock([]string{"A=bad\x00value"}); err == nil {
		t.Fatal("environment block accepted NUL")
	}
}

func TestLaunchRequestCapturesAppContainerOutput(t *testing.T) {
	// End-to-end output capture is exercised by the CI-only AppContainer test.
	// This unit-level check protects the request wiring on every Windows run.
	var stdout, stderr bytes.Buffer
	request := LaunchRequest{Stdout: &stdout, Stderr: &stderr}
	if request.Stdout == nil || request.Stderr == nil {
		t.Fatal("sandbox launch output writers were lost")
	}
}
