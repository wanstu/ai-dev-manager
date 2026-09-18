//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func startDetachedGatewayProcess(listen string) (*os.Process, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前 ADM 可执行文件路径失败: %w", err)
	}
	cmd := exec.Command(executable, "gateway", "start", "--listen", listen)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
