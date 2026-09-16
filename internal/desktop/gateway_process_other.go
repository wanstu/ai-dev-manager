//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func startDetachedGatewayProcess(listen string) (*os.Process, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get desktop executable path: %w", err)
	}
	cmd := exec.Command(executable, "--gateway-child", "--listen", listen)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
