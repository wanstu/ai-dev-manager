//go:build linux

package cli

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func findListeningProcess(listen string) (int, string, error) {
	_, portText, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, "", fmt.Errorf("解析监听地址 %s: %w", listen, err)
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return 0, "", fmt.Errorf("解析监听端口 %s: %w", portText, err)
	}
	inodes := map[string]struct{}{}
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		found, readErr := linuxListeningSocketInodes(table, uint16(port))
		if readErr != nil && !os.IsNotExist(readErr) {
			return 0, "", readErr
		}
		for inode := range found {
			inodes[inode] = struct{}{}
		}
	}
	if len(inodes) == 0 {
		return 0, "", fmt.Errorf("没有找到监听 %s 的 TCP socket", listen)
	}

	pids, err := os.ReadDir("/proc")
	if err != nil {
		return 0, "", fmt.Errorf("读取 /proc: %w", err)
	}
	type match struct {
		pid  int
		path string
	}
	matches := []match{}
	for _, entry := range pids {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		matched := false
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if _, ok := inodes[inode]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil {
			continue
		}
		matches = append(matches, match{pid: pid, path: filepath.Clean(exe)})
	}
	if len(matches) == 0 {
		return 0, "", fmt.Errorf("已找到监听 %s 的 socket，但无法定位拥有该 socket 的进程", listen)
	}
	if len(matches) > 1 {
		return 0, "", fmt.Errorf("监听 %s 对应多个进程，拒绝自动选择 PID", listen)
	}
	return matches[0].pid, matches[0].path, nil
}

func linuxListeningSocketInodes(path string, port uint16) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := map[string]struct{}{}
	scanner := bufio.NewScanner(file)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[3] != "0A" {
			continue
		}
		parts := strings.Split(fields[1], ":")
		if len(parts) != 2 {
			continue
		}
		value, err := strconv.ParseUint(parts[1], 16, 16)
		if err != nil || uint16(value) != port {
			continue
		}
		result[fields[9]] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, err)
	}
	return result, nil
}
