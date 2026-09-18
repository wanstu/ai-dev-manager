//go:build darwin

package cli

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func findListeningProcess(listen string) (int, string, error) {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, "", fmt.Errorf("解析监听地址 %s: %w", listen, err)
	}
	output, err := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-Fp").Output()
	if err != nil {
		return 0, "", fmt.Errorf("使用 lsof 定位监听 %s 的进程失败: %w", listen, err)
	}
	pids := map[int]struct{}{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "p") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimPrefix(line, "p"))
		if err == nil && pid > 0 {
			pids[pid] = struct{}{}
		}
	}
	if len(pids) == 0 {
		return 0, "", fmt.Errorf("没有找到监听 %s 的进程", listen)
	}
	if len(pids) > 1 {
		return 0, "", fmt.Errorf("监听 %s 对应多个进程，拒绝自动选择 PID", listen)
	}
	var pid int
	for value := range pids {
		pid = value
	}
	path, err := darwinExecutablePath(pid)
	if err != nil {
		return 0, "", err
	}
	return pid, path, nil
}

func darwinExecutablePath(pid int) (string, error) {
	output, err := exec.Command("/usr/sbin/lsof", "-a", "-p", strconv.Itoa(pid), "-d", "txt", "-Fn").Output()
	if err != nil {
		return "", fmt.Errorf("读取进程 %d 可执行文件失败: %w", pid, err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "n/") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}
	return "", fmt.Errorf("无法解析进程 %d 的可执行文件路径", pid)
}
