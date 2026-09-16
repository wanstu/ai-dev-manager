//go:build linux

package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ListeningTCPPorts reports listening TCP ports only for the supplied process
// by matching that process's /proc/<pid>/fd socket inodes against the kernel
// TCP tables. It never scans unrelated process directories.
func ListeningTCPPorts(pid int) ([]int, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("process pid must be positive")
	}
	inodes, err := linuxProcessSocketInodes(pid)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("observe owned process sockets: %w", err)
	}
	if len(inodes) == 0 {
		return nil, nil
	}
	seen := map[int]struct{}{}
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if err := collectLinuxListeningPorts(table, inodes, seen); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}

func linuxProcessSocketInodes(pid int) (map[string]struct{}, error) {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "fd"))
	if err != nil {
		return nil, err
	}
	inodes := map[string]struct{}{}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", entry.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if inode != "" {
				inodes[inode] = struct{}{}
			}
		}
	}
	return inodes, nil
}

func collectLinuxListeningPorts(path string, inodes map[string]struct{}, seen map[int]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || fields[3] != "0A" {
			continue
		}
		if _, ok := inodes[fields[9]]; !ok {
			continue
		}
		local := fields[1]
		separator := strings.LastIndexByte(local, ':')
		if separator < 0 || separator == len(local)-1 {
			continue
		}
		port64, err := strconv.ParseUint(local[separator+1:], 16, 16)
		if err != nil || port64 == 0 {
			continue
		}
		seen[int(port64)] = struct{}{}
	}
	return scanner.Err()
}
