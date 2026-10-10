//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const desktopInstallerExeName = "adm-desktop.exe"

type desktopProcessEntry struct {
	pid  uint32
	name string
}

// otherDesktopProcessIDs applies the same executable-name policy as Kit's
// Windows Setup, while excluding only this Desktop process.
func otherDesktopProcessIDs(entries []desktopProcessEntry, ownPID, allowedGatewayPID uint32) []uint32 {
	var blockers []uint32
	for _, entry := range entries {
		if entry.pid != ownPID && entry.pid != allowedGatewayPID && strings.EqualFold(entry.name, desktopInstallerExeName) {
			blockers = append(blockers, entry.pid)
		}
	}
	sort.Slice(blockers, func(i, j int) bool { return blockers[i] < blockers[j] })
	return blockers
}

// ensureNoOtherDesktopProcesses never ends processes. It reports every
// remaining same-name instance before Kit starts a Setup that would refuse
// to replace the running executable.
func ensureNoOtherDesktopProcesses() error {
	return ensureNoOtherDesktopProcessesExcept(0)
}

func ensureNoOtherDesktopProcessesExcept(allowedGatewayPID uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return fmt.Errorf("检查同名 Desktop 进程失败: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return fmt.Errorf("读取 Desktop 进程列表失败: %w", err)
	}
	var entries []desktopProcessEntry
	for {
		entries = append(entries, desktopProcessEntry{
			pid:  entry.ProcessID,
			name: windows.UTF16ToString(entry.ExeFile[:]),
		})
		err = windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 Desktop 进程列表失败: %w", err)
		}
	}
	if blockers := otherDesktopProcessIDs(entries, uint32(os.Getpid()), allowedGatewayPID); len(blockers) > 0 {
		return fmt.Errorf("仍有其他 adm-desktop.exe 进程正在运行（PID: %v）；请先从对应实例正常退出，再重试安装", blockers)
	}
	return nil
}

// waitForGatewayProcessExit waits only for the recognized Gateway PID reported
// before its controlled shutdown. It never terminates or opens unrelated PIDs.
func waitForGatewayProcessExit(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("Gateway 未提供可验证的进程 PID")
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // Process already exited.
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	wait, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil {
		return err
	}
	if wait != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("本地 Gateway 进程 %d 在 5 秒内未退出", pid)
	}
	return nil
}
