//go:build darwin

package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// ListeningTCPPorts reports listening TCP ports only for the supplied process.
// macOS does not expose Linux-style /proc socket tables, so this uses the
// system lsof binary with an explicit PID filter rather than a host-wide scan.
func ListeningTCPPorts(pid int) ([]int, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("process pid must be positive")
	}
	lsofPath := "/usr/sbin/lsof"
	if _, err := os.Stat(lsofPath); err != nil {
		resolved, lookErr := exec.LookPath("lsof")
		if lookErr != nil {
			return nil, fmt.Errorf("observe owned process ports: lsof unavailable: %w", lookErr)
		}
		lsofPath = resolved
	}
	cmd := exec.Command(lsofPath,
		"-nP", "-a", "-p", strconv.Itoa(pid), "-iTCP", "-sTCP:LISTEN", "-F", "n")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("observe owned process ports: %w", err)
		}
		// lsof exits 1 when the scoped process has no matching LISTEN sockets.
		if len(out) == 0 {
			return nil, nil
		}
	}
	seen := map[int]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "n") {
			continue
		}
		endpoint := strings.TrimPrefix(line, "n")
		separator := strings.LastIndexByte(endpoint, ':')
		if separator < 0 || separator == len(endpoint)-1 {
			continue
		}
		port, err := strconv.Atoi(endpoint[separator+1:])
		if err == nil && port > 0 && port <= 65535 {
			seen[port] = struct{}{}
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}
