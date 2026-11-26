//go:build !windows && !linux

package runtime

import (
	"fmt"
	"os/exec"
)

type unobservedManagedProcess struct{}

func startManagedProcess(cmd *exec.Cmd) (ManagedProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return unobservedManagedProcess{}, nil
}
func (unobservedManagedProcess) Close() {}
func (unobservedManagedProcess) Sample() (ProcessResources, error) {
	return ProcessResources{}, fmt.Errorf("resource observation unavailable on this platform")
}
