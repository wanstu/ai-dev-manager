//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func findListeningProcess(listen string) (int, string, error) {
	return 0, "", fmt.Errorf("当前平台暂不支持自动定位监听 %s 的旧版 Gateway 进程", listen)
}

func startDetachedGatewayProcess(listen string) (*os.Process, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前 ADM V2 可执行文件路径失败: %w", err)
	}
	cmd := exec.Command(executable, "gateway", "start", "--listen", listen)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
