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
	sandboxHostPrepReadyMarkerName = "sandbox-host-prep.ready"
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
		status.Message = "强沙盒当前已经生效，但还需要完成一次管理员确认，才能保证重启电脑后继续可用"
	} else if report.NullDevicePrepared && taskInstalled {
		status.Message = "强沙盒已启用：当前可用，重启电脑后也会自动恢复"
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
	if err := writeSandboxHostPrepReadyMarker(helper); err != nil {
		_ = removeSandboxHostPrepBootTask()
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
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("resolve protected Program Files directory: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) {
		return "", fmt.Errorf("Windows Program Files known folder is unavailable")
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
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return err
	}
	action := fmt.Sprintf(`"%s" --sandbox-host-prep-boot`, helper)
	command := hiddenWindowsCommand(
		schtasks,
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
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return err
	}
	command := hiddenWindowsCommand(schtasks, "/Delete", "/TN", sandboxHostPrepTaskName, "/F")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove sandbox host-prep boot task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_ = removeSandboxHostPrepReadyMarker()
	return nil
}

func sandboxHostPrepBootTaskInstalled() (bool, error) {
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return false, err
	}
	command := hiddenWindowsCommand(schtasks, "/Query", "/TN", sandboxHostPrepTaskName)
	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			// SYSTEM tasks may intentionally deny ordinary users permission to
			// query their Task Scheduler metadata. Do not interpret that as
			// "not installed". The elevated installer writes a readiness marker
			// next to the protected Program Files helper only after task
			// registration succeeds; ordinary users can read but not forge it.
			return sandboxHostPrepReadyMarkerInstalled(), nil
		}
		return false, fmt.Errorf("query sandbox host-prep boot task: %w", err)
	}
	return true, nil
}

func sandboxHostPrepReadyMarkerPath() (string, error) {
	helper, err := sandboxHostPrepMachineExecutable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(helper), sandboxHostPrepReadyMarkerName), nil
}

func writeSandboxHostPrepReadyMarker(helper string) error {
	marker, err := sandboxHostPrepReadyMarkerPath()
	if err != nil {
		return err
	}
	if filepath.Clean(filepath.Dir(marker)) != filepath.Clean(filepath.Dir(helper)) {
		return fmt.Errorf("sandbox host-prep readiness marker escaped protected helper directory")
	}
	payload := []byte("task=" + sandboxHostPrepTaskName + "\nhelper=" + filepath.Base(helper) + "\n")
	temp, err := os.CreateTemp(filepath.Dir(marker), "sandbox-host-prep-*.ready")
	if err != nil {
		return fmt.Errorf("create sandbox host-prep readiness marker: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		_ = temp.Close()
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(payload); err != nil {
		return fmt.Errorf("write sandbox host-prep readiness marker: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync sandbox host-prep readiness marker: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close sandbox host-prep readiness marker: %w", err)
	}
	_ = os.Remove(marker)
	if err := os.Rename(tempName, marker); err != nil {
		return fmt.Errorf("install sandbox host-prep readiness marker: %w", err)
	}
	cleanup = false
	return nil
}

func removeSandboxHostPrepReadyMarker() error {
	marker, err := sandboxHostPrepReadyMarkerPath()
	if err != nil {
		return err
	}
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func sandboxHostPrepReadyMarkerInstalled() bool {
	helper, err := sandboxHostPrepMachineExecutable()
	if err != nil {
		return false
	}
	helperInfo, err := os.Stat(helper)
	if err != nil || helperInfo.IsDir() {
		return false
	}
	marker, err := sandboxHostPrepReadyMarkerPath()
	if err != nil {
		return false
	}
	markerInfo, err := os.Stat(marker)
	return err == nil && !markerInfo.IsDir()
}

func sandboxHostPrepSystemExecutable(name string) (string, error) {
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid Windows system executable name %q", name)
	}
	systemRoot, err := windows.KnownFolderPath(windows.FOLDERID_System, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("resolve Windows system directory: %w", err)
	}
	path := filepath.Join(systemRoot, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("resolve Windows system executable %s: %w", name, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("Windows system executable is a directory: %s", path)
	}
	return path, nil
}

func hiddenWindowsCommand(name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command
}
