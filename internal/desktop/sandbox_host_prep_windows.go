//go:build windows

package desktop

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ai-dev-manager-v2/internal/sandbox"

	"golang.org/x/sys/windows"
)

const (
	sandboxHostPrepTaskName       = "AI Dev Manager Sandbox Host Prep"
	sandboxHostPrepMachineDirName = "AI Dev Manager"
	sandboxHostPrepMachineExeName = "adm-sandbox-host-prep.exe"
)

func sandboxHostPreparationStatus() SandboxHostPreparationStatus {
	report := sandbox.InspectHostPreparation()
	taskInstalled, taskErr := sandboxHostPrepBootTaskInstalled()
	status := SandboxHostPreparationStatus{
		Platform:           "windows",
		Supported:          true,
		NullDevicePrepared: report.NullDevicePrepared,
		BootTaskInstalled:  taskInstalled,
		RequiresAdminSetup: !report.NullDevicePrepared || !taskInstalled,
		Message:            report.Message,
	}
	if taskErr != nil {
		status.RequiresAdminSetup = true
		if status.Message == "" {
			status.Message = taskErr.Error()
		} else {
			status.Message += "; " + taskErr.Error()
		}
	} else if report.NullDevicePrepared && !taskInstalled {
		status.Message = "当前会话的强沙盒已准备，但尚未配置开机自动恢复"
	} else if report.NullDevicePrepared && taskInstalled {
		status.Message = "Windows 强沙盒已准备，并会在开机时自动恢复"
	}
	return status
}

func enableSandboxHostPreparation() (SandboxHostPreparationStatus, error) {
	if windows.GetCurrentProcessToken().IsElevated() {
		if err := RunSandboxHostPreparationInstall(); err != nil {
			return sandboxHostPreparationStatus(), err
		}
		return sandboxHostPreparationStatus(), nil
	}

	executable, err := os.Executable()
	if err != nil {
		return sandboxHostPreparationStatus(), fmt.Errorf("resolve Desktop executable: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return sandboxHostPreparationStatus(), err
	}
	file, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return sandboxHostPreparationStatus(), err
	}
	args, err := windows.UTF16PtrFromString("--sandbox-host-prep-install")
	if err != nil {
		return sandboxHostPreparationStatus(), err
	}
	if err := windows.ShellExecute(0, verb, file, args, nil, 0); err != nil {
		return sandboxHostPreparationStatus(), fmt.Errorf("request administrator approval: %w", err)
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		status := sandboxHostPreparationStatus()
		if status.NullDevicePrepared && status.BootTaskInstalled {
			return status, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	status := sandboxHostPreparationStatus()
	return status, fmt.Errorf("administrator setup did not complete before timeout")
}

// RunSandboxHostPreparationInstall is invoked only by the elevated hidden
// Desktop mode. It copies the executable into Program Files before creating a
// SYSTEM boot task; the task never points at a user-writable binary.
func RunSandboxHostPreparationInstall() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("sandbox host preparation install requires administrator elevation")
	}
	source, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Desktop executable: %w", err)
	}
	helper, err := sandboxHostPrepMachineExecutable()
	if err != nil {
		return err
	}
	if err := copyProtectedHostPrepExecutable(source, helper); err != nil {
		return err
	}
	if err := registerSandboxHostPrepBootTask(helper); err != nil {
		return err
	}
	if err := sandbox.PrepareNullDeviceHost(); err != nil {
		_ = removeSandboxHostPrepBootTask()
		return err
	}
	status := sandboxHostPreparationStatus()
	if !status.NullDevicePrepared || !status.BootTaskInstalled {
		return fmt.Errorf("sandbox host preparation completed but readiness verification failed")
	}
	return nil
}

func RunSandboxHostPreparationBoot() error {
	return sandbox.PrepareNullDeviceHost()
}

func sandboxHostPrepMachineExecutable() (string, error) {
	root := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if root == "" {
		return "", fmt.Errorf("ProgramFiles is unavailable")
	}
	return filepath.Join(root, sandboxHostPrepMachineDirName, sandboxHostPrepMachineExeName), nil
}

func copyProtectedHostPrepExecutable(source, destination string) error {
	if strings.Contains(destination, `"`) || strings.Contains(source, `"`) {
		return fmt.Errorf("unexpected quote in host-prep executable path")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create protected host-prep directory: %w", err)
	}
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open Desktop executable for host-prep copy: %w", err)
	}
	defer src.Close()

	temp, err := os.CreateTemp(filepath.Dir(destination), "adm-host-prep-*.exe")
	if err != nil {
		return fmt.Errorf("create protected host-prep temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		_ = temp.Close()
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := io.Copy(temp, src); err != nil {
		return fmt.Errorf("copy protected host-prep executable: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync protected host-prep executable: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close protected host-prep executable: %w", err)
	}
	_ = os.Remove(destination)
	if err := os.Rename(tempName, destination); err != nil {
		return fmt.Errorf("install protected host-prep executable: %w", err)
	}
	cleanup = false
	return nil
}

func registerSandboxHostPrepBootTask(helper string) error {
	if strings.Contains(helper, `"`) {
		return fmt.Errorf("unexpected quote in host-prep helper path")
	}
	action := fmt.Sprintf(`"%s" --sandbox-host-prep-boot`, helper)
	command := hiddenWindowsCommand(
		"schtasks.exe",
		"/Create",
		"/TN", sandboxHostPrepTaskName,
		"/TR", action,
		"/SC", "ONSTART",
		"/RU", "SYSTEM",
		"/RL", "HIGHEST",
		"/F",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("register sandbox host-prep boot task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func removeSandboxHostPrepBootTask() error {
	command := hiddenWindowsCommand("schtasks.exe", "/Delete", "/TN", sandboxHostPrepTaskName, "/F")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove sandbox host-prep boot task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sandboxHostPrepBootTaskInstalled() (bool, error) {
	command := hiddenWindowsCommand("schtasks.exe", "/Query", "/TN", sandboxHostPrepTaskName)
	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return false, nil
		}
		return false, fmt.Errorf("query sandbox host-prep boot task: %w", err)
	}
	return true, nil
}

func hiddenWindowsCommand(name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command
}
