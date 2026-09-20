//go:build !windows

package isolation

import "os/exec"

func configureProcessCommand(cmd *exec.Cmd) {}
