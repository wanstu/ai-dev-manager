//go:build windows

package desktop

import (
	"errors"
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
	sandboxHostPrepTaskName        = "AI Dev Manager Sandbox Host Prep"
	sandboxHostPrepMachineDirName  = "AI Dev Manager"
	sandboxHostPrepMachineExeName  = "adm-sandbox-host-prep.exe"
	sandboxHostPrepReadyMarkerName = "sandbox-host-prep.ready"
)

func sandboxHostPreparationStatus() SandboxHostPreparationStatus {
	report := sandbox.InspectHostPreparation()
	taskInstalled, taskVerified, taskErr := sandboxHostPrepBootTaskStatus()
	brokerReady := sandboxACLBrokerReady()
	status := SandboxHostPreparationStatus{
		Platform:           "windows",
		Supported:          true,
		NullDevicePrepared: report.NullDevicePrepared,
		BootTaskInstalled:  taskInstalled,
		BootTaskVerified:   taskVerified,
		ACLBrokerReady:     brokerReady,
		RequiresAdminSetup: !report.NullDevicePrepared || !taskInstalled || !taskVerified || !brokerReady,
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
	} else if report.NullDevicePrepared && taskInstalled && !taskVerified {
		status.Message = "强沙盒当前可用；开机恢复标记存在，但任务查询无法验证，可能被安全软件或系统权限拦截"
	} else if report.NullDevicePrepared && taskInstalled && !brokerReady {
		status.Message = "强沙盒基础能力已准备，但 Sandbox ACL broker 未运行；需要修复受保护的开机任务"
	} else if report.NullDevicePrepared && taskInstalled {
		status.Message = "强沙盒已启用：当前可用，Sandbox ACL broker 已运行，重启电脑后也会自动恢复"
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
		if status.NullDevicePrepared && status.BootTaskInstalled && status.ACLBrokerReady {
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
	userSID, err := currentWindowsUserSID()
	if err != nil {
		return fmt.Errorf("resolve Desktop install user SID: %w", err)
	}
	helper, err := sandboxHostPrepMachineExecutable()
	if err != nil {
		return err
	}
	_ = endSandboxHostPrepBootTask()
	if err := waitSandboxACLBrokerStopped(10 * time.Second); err != nil {
		return err
	}
	copyDeadline := time.Now().Add(10 * time.Second)
	for {
		copyErr := copyProtectedHostPrepExecutable(source, helper)
		if copyErr == nil {
			break
		}
		if !errors.Is(copyErr, windows.ERROR_ACCESS_DENIED) && !errors.Is(copyErr, windows.ERROR_SHARING_VIOLATION) {
			return copyErr
		}
		if time.Now().After(copyDeadline) {
			return fmt.Errorf("replace protected sandbox host-prep executable after stopping previous broker: %w", copyErr)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := registerSandboxHostPrepBootTask(helper); err != nil {
		return err
	}
	if err := writeSandboxHostPrepReadyMarker(helper, userSID); err != nil {
		_ = removeSandboxHostPrepBootTask()
		return err
	}
	if err := sandbox.PrepareNullDeviceHost(); err != nil {
		_ = removeSandboxHostPrepBootTask()
		return err
	}
	if err := startSandboxHostPrepBootTask(); err != nil {
		_ = removeSandboxHostPrepBootTask()
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if sandboxACLBrokerReady() {
			status := sandboxHostPreparationStatus()
			if status.NullDevicePrepared && status.BootTaskInstalled {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("sandbox host preparation completed but ACL broker did not become ready")
}

func RunSandboxHostPreparationBoot() error {
	if err := sandbox.PrepareNullDeviceHost(); err != nil {
		return err
	}
	return RunSandboxACLBroker()
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

func waitSandboxACLBrokerStopped(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !sandboxACLBrokerReady() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("previous sandbox ACL broker did not stop before protected helper replacement")
}

func startSandboxHostPrepBootTask() error {
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return err
	}
	command := hiddenWindowsCommand(schtasks, "/Run", "/TN", sandboxHostPrepTaskName)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start sandbox host-prep task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func endSandboxHostPrepBootTask() error {
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return err
	}
	command := hiddenWindowsCommand(schtasks, "/End", "/TN", sandboxHostPrepTaskName)
	_ = command.Run()
	return nil
}

func removeSandboxHostPrepBootTask() error {
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return err
	}
	_ = endSandboxHostPrepBootTask()
	command := hiddenWindowsCommand(schtasks, "/Delete", "/TN", sandboxHostPrepTaskName, "/F")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove sandbox host-prep boot task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	_ = removeSandboxHostPrepReadyMarker()
	return nil
}

func sandboxHostPrepBootTaskStatus() (installed bool, verified bool, err error) {
	schtasks, err := sandboxHostPrepSystemExecutable("schtasks.exe")
	if err != nil {
		return false, false, err
	}
	command := hiddenWindowsCommand(schtasks, "/Query", "/TN", sandboxHostPrepTaskName)
	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			// Security software and SYSTEM-task ACLs can both block Task Scheduler
			// queries. A protected marker still proves that an elevated installer
			// completed task registration at some point, but it does not prove the
			// task remains queryable/present now. Preserve that distinction.
			return sandboxHostPrepReadyMarkerInstalled(), false, nil
		}
		return false, false, fmt.Errorf("query sandbox host-prep boot task: %w", err)
	}
	return true, true, nil
}

func sandboxHostPrepReadyMarkerPath() (string, error) {
	helper, err := sandboxHostPrepMachineExecutable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(helper), sandboxHostPrepReadyMarkerName), nil
}

func currentWindowsUserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	if user == nil || user.User.Sid == nil {
		return "", fmt.Errorf("current Windows token has no user SID")
	}
	return user.User.Sid.String(), nil
}

func writeSandboxHostPrepReadyMarker(helper, userSID string) error {
	marker, err := sandboxHostPrepReadyMarkerPath()
	if err != nil {
		return err
	}
	if filepath.Clean(filepath.Dir(marker)) != filepath.Clean(filepath.Dir(helper)) {
		return fmt.Errorf("sandbox host-prep readiness marker escaped protected helper directory")
	}
	if _, err := windows.StringToSid(strings.TrimSpace(userSID)); err != nil {
		return fmt.Errorf("invalid sandbox host-prep user SID: %w", err)
	}
	payload := []byte("task=" + sandboxHostPrepTaskName + "\nhelper=" + filepath.Base(helper) + "\nuser_sid=" + strings.TrimSpace(userSID) + "\n")
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

func sandboxHostPrepInstalledUserSID() (string, error) {
	marker, err := sandboxHostPrepReadyMarkerPath()
	if err != nil {
		return "", err
	}
	payload, err := os.ReadFile(marker)
	if err != nil {
		return "", fmt.Errorf("read sandbox host-prep readiness marker: %w", err)
	}
	for _, line := range strings.Split(string(payload), "\n") {
		if !strings.HasPrefix(line, "user_sid=") {
			continue
		}
		sid := strings.TrimSpace(strings.TrimPrefix(line, "user_sid="))
		if _, err := windows.StringToSid(sid); err != nil {
			return "", fmt.Errorf("invalid installed sandbox host-prep user SID: %w", err)
		}
		return sid, nil
	}
	return "", fmt.Errorf("sandbox host-prep readiness marker has no installed user SID")
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
