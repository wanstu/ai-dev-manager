//go:build windows

package cli

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func findListeningProcess(listen string) (int, string, error) {
	netstat := exec.Command("netstat", "-ano", "-p", "tcp")
	netstat.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := netstat.Output()
	if err != nil {
		return 0, "", fmt.Errorf("执行 netstat 失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !sameListenEndpoint(fields[1], listen) {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || pid <= 0 {
			continue
		}
		path, err := processExecutablePath(pid)
		if err != nil {
			return 0, "", fmt.Errorf("已找到监听进程 PID %d，但读取进程路径失败: %w", pid, err)
		}
		return pid, path, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, "", fmt.Errorf("读取 netstat 输出失败: %w", err)
	}
	return 0, "", fmt.Errorf("没有找到监听 %s 的进程", listen)
}

func sameListenEndpoint(actual, expected string) bool {
	actualHost, actualPort, err := net.SplitHostPort(actual)
	if err != nil {
		return false
	}
	expectedHost, expectedPort, err := net.SplitHostPort(expected)
	if err != nil || actualPort != expectedPort {
		return false
	}

	actualHost = strings.Trim(strings.ToLower(actualHost), "[]")
	expectedHost = strings.Trim(strings.ToLower(expectedHost), "[]")
	if expectedHost == "localhost" {
		return actualHost == "127.0.0.1" || actualHost == "::1" || actualHost == "localhost"
	}
	actualIP := net.ParseIP(actualHost)
	expectedIP := net.ParseIP(expectedHost)
	if actualIP != nil && expectedIP != nil {
		return actualIP.Equal(expectedIP)
	}
	return actualHost == expectedHost
}

func processExecutablePath(pid int) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func startDetachedGatewayProcess(listen string) (*os.Process, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前 ADM V2 可执行文件路径失败: %w", err)
	}
	cmd := exec.Command(executable, "gateway", "start", "--listen", listen)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, CreationFlags: 0x00000008 | 0x00000200 | 0x08000000, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
