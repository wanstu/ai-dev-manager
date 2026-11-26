//go:build linux

package runtime

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type linuxManagedProcess struct {
	pid   int
	start uint64
	stop  func()
}
type linuxProcessStat struct {
	group                     int
	state                     string
	start, cpuTicks, rssPages uint64
}

func readLinuxProcessStat(pid int) (linuxProcessStat, error) {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return linuxProcessStat{}, err
	}
	return parseLinuxProcessStat(string(raw))
}
func parseLinuxProcessStat(raw string) (linuxProcessStat, error) {
	// comm may contain spaces and parentheses; the final ')' ends comm.
	end := strings.LastIndexByte(raw, ')')
	if end < 0 {
		return linuxProcessStat{}, fmt.Errorf("invalid process stat")
	}
	f := strings.Fields(raw[end+1:])
	if len(f) < 22 {
		return linuxProcessStat{}, fmt.Errorf("truncated process stat")
	}
	group, e1 := strconv.Atoi(f[2])
	user, e2 := strconv.ParseUint(f[11], 10, 64)
	kernel, e3 := strconv.ParseUint(f[12], 10, 64)
	start, e4 := strconv.ParseUint(f[19], 10, 64)
	rss, e5 := strconv.ParseInt(f[21], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
		return linuxProcessStat{}, fmt.Errorf("invalid process stat counters")
	}
	if rss < 0 {
		rss = 0
	}
	return linuxProcessStat{group: group, state: f[0], start: start, cpuTicks: user + kernel, rssPages: uint64(rss)}, nil
}
func startManagedProcess(cmd *exec.Cmd) (ManagedProcess, error) {
	return startSupervisedLinuxProcess(cmd)
}
func (m *linuxManagedProcess) Close() {
	if m.stop != nil {
		m.stop()
	}
}
func (m *linuxManagedProcess) WorkloadPID() int { return m.pid }
func (m *linuxManagedProcess) Sample() (ProcessResources, error) {
	root, err := readLinuxProcessStat(m.pid)
	if err != nil || m.start == 0 || root.start != m.start || root.group != m.pid {
		return ProcessResources{}, fmt.Errorf("process group identity unavailable")
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ProcessResources{}, err
	}
	out := ProcessResources{SampledAt: time.Now().UTC(), Scope: "process_group", Complete: true}
	var ticks uint64
	var basis strings.Builder
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		stat, err := readLinuxProcessStat(pid)
		if err != nil {
			continue
		}
		if stat.group != m.pid || stat.state == "Z" || stat.state == "X" {
			continue
		}
		fmt.Fprintf(&basis, "%d:%d;", pid, stat.start)
		out.ProcessCount++
		out.MemoryBytes += stat.rssPages * uint64(os.Getpagesize())
		ticks += stat.cpuTicks
	}
	// Linux exposes /proc stat CPU counters in USER_HZ=100 on supported
	// Linux amd64/arm64 builds. Unlike Windows job totals this is live members only.
	out.CPUTimeMS = ticks * 10
	out.CPUBasis = fmt.Sprintf("%x", sha256.Sum256([]byte(basis.String())))
	root, err = readLinuxProcessStat(m.pid)
	if err != nil || root.start != m.start || root.group != m.pid {
		return ProcessResources{}, fmt.Errorf("process group changed while sampling")
	}
	return out, nil
}
