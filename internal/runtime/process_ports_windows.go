//go:build windows

package runtime

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// ListeningTCPPorts reports listening TCP ports only for the supplied process
// ID. Callers must already own/authorize that PID; this is an observation
// primitive, not a generic process-management API.
func ListeningTCPPorts(pid int) ([]int, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("process pid must be positive")
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows system directory: %w", err)
	}
	cmd := exec.Command(filepath.Join(systemDir, "netstat.exe"), "-ano", "-p", "tcp")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("observe owned process ports: %w", err)
	}
	seen := map[int]struct{}{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		rowPID, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil || rowPID != pid {
			continue
		}
		_, portText, err := net.SplitHostPort(fields[1])
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err == nil && port > 0 && port <= 65535 {
			seen[port] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}
