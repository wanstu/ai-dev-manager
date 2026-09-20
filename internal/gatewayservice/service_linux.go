//go:build linux

package gatewayservice

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func platformSupported() bool { return true }

func checkManagementPrivilegesPlatform() error { return requireRoot() }

func resolveTargetUser(name string) (TargetUser, error) {
	account, err := user.Lookup(strings.TrimSpace(name))
	if err != nil {
		return TargetUser{}, fmt.Errorf("lookup gateway service user %q: %w", name, err)
	}
	uid, gid, err := ParseUIDGID(account.Uid, account.Gid)
	if err != nil {
		return TargetUser{}, err
	}
	home := strings.TrimSpace(account.HomeDir)
	if home == "" || !filepath.IsAbs(home) {
		return TargetUser{}, fmt.Errorf("gateway service user %q has no absolute home directory", account.Username)
	}
	return TargetUser{
		Name:      account.Username,
		Home:      home,
		UID:       uid,
		GID:       gid,
		StatePath: filepath.Join(home, ".config", "ai-dev-manager-v2", "state.json"),
	}, nil
}

func fixStateOwnership(target TargetUser) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("fix Gateway state ownership requires root")
	}
	root := filepath.Dir(target.StatePath)
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := os.Chown(path, target.UID, target.GID); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
		if entry.IsDir() {
			if err := os.Chmod(path, 0o700); err != nil {
				return fmt.Errorf("chmod %s: %w", path, err)
			}
		}
		return nil
	})
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("gateway service management requires root; run with sudo")
	}
	return nil
}

func validateExecutableFile(executable string) error {
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("stat ADM executable %s: %w", executable, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("ADM executable %s is not an executable regular file", executable)
	}
	return nil
}

func writeUnitAtomically(data []byte) error {
	tmp := UnitPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}
	if err := os.Rename(tmp, UnitPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install systemd unit: %w", err)
	}
	return nil
}

func installPlatform(options InstallOptions) (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	if err := validateExecutableFile(options.Executable); err != nil {
		return Status{}, err
	}
	if data, err := os.ReadFile(UnitPath); err == nil {
		if !strings.Contains(string(data), "# Managed by AI Dev Manager.") {
			return Status{}, fmt.Errorf("%s exists but is not managed by ADM; refusing to overwrite it", UnitPath)
		}
		return Status{}, fmt.Errorf("ADM Gateway service is already installed; uninstall it first or use gateway service restart")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Status{}, err
	}
	unit, err := RenderSystemdUnit(options)
	if err != nil {
		return Status{}, err
	}
	if err := writeUnitAtomically([]byte(unit)); err != nil {
		return Status{}, err
	}
	rollback := true
	defer func() {
		if rollback {
			_, _ = runSystemctl("disable", "--now", UnitName)
			_ = os.Remove(UnitPath)
			_, _ = runSystemctl("daemon-reload")
		}
	}()
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return Status{}, err
	}
	if _, err := runSystemctl("enable", UnitName); err != nil {
		return Status{}, err
	}
	if options.Start {
		if _, err := runSystemctl("start", UnitName); err != nil {
			return Status{}, err
		}
	}
	rollback = false
	return inspectPlatform()
}

func reconfigurePlatform(options InstallOptions) (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	if err := validateExecutableFile(options.Executable); err != nil {
		return Status{}, err
	}
	oldUnit, err := os.ReadFile(UnitPath)
	if errors.Is(err, os.ErrNotExist) {
		return Status{}, fmt.Errorf("ADM Gateway service is not installed")
	}
	if err != nil {
		return Status{}, err
	}
	if !strings.Contains(string(oldUnit), "# Managed by AI Dev Manager.") {
		return Status{}, fmt.Errorf("%s exists but is not managed by ADM; refusing to overwrite it", UnitPath)
	}
	previous, err := inspectPlatform()
	if err != nil {
		return Status{}, err
	}
	newUnit, err := RenderSystemdUnit(options)
	if err != nil {
		return Status{}, err
	}

	restore := func(cause error) error {
		restoreErr := writeUnitAtomically(oldUnit)
		if restoreErr == nil {
			_, restoreErr = runSystemctl("daemon-reload")
		}
		if restoreErr == nil {
			if previous.Enabled {
				_, restoreErr = runSystemctl("enable", UnitName)
			} else {
				_, restoreErr = runSystemctl("disable", UnitName)
			}
		}
		if restoreErr == nil {
			if previous.Active {
				_, restoreErr = runSystemctl("restart", UnitName)
			} else {
				_, restoreErr = runSystemctl("stop", UnitName)
			}
		}
		if restoreErr != nil {
			return fmt.Errorf("%w; additionally failed to restore previous Gateway service: %v", cause, restoreErr)
		}
		return cause
	}

	if err := writeUnitAtomically([]byte(newUnit)); err != nil {
		return Status{}, err
	}
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return Status{}, restore(err)
	}
	if options.Start {
		action := "start"
		if previous.Active {
			action = "restart"
		}
		if _, err := runSystemctl(action, UnitName); err != nil {
			return Status{}, restore(err)
		}
	} else if previous.Active {
		if _, err := runSystemctl("stop", UnitName); err != nil {
			return Status{}, restore(err)
		}
	}
	return inspectPlatform()
}

func startPlatform() (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	status, err := inspectPlatform()
	if err != nil {
		return Status{}, err
	}
	if err := validateManagedServiceStatus(status); err != nil {
		return status, err
	}
	if _, err := runSystemctl("start", UnitName); err != nil {
		return status, err
	}
	return inspectPlatform()
}

func restartPlatform() (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	status, err := inspectPlatform()
	if err != nil {
		return Status{}, err
	}
	if err := validateManagedServiceStatus(status); err != nil {
		return status, err
	}
	if _, err := runSystemctl("restart", UnitName); err != nil {
		return status, err
	}
	return inspectPlatform()
}

func stopPlatform() (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	status, err := inspectPlatform()
	if err != nil {
		return Status{}, err
	}
	if err := validateManagedServiceStatus(status); err != nil {
		return status, err
	}
	if _, err := runSystemctl("stop", UnitName); err != nil {
		return status, err
	}
	return inspectPlatform()
}

func setEnabledPlatform(enabled bool) (Status, error) {
	if err := requireRoot(); err != nil {
		return Status{}, err
	}
	status, err := inspectPlatform()
	if err != nil {
		return Status{}, err
	}
	if err := validateManagedServiceStatus(status); err != nil {
		return status, err
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	if _, err := runSystemctl(action, UnitName); err != nil {
		return status, err
	}
	return inspectPlatform()
}

func uninstallPlatform() error {
	if err := requireRoot(); err != nil {
		return err
	}
	data, err := os.ReadFile(UnitPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), "# Managed by AI Dev Manager.") {
		return fmt.Errorf("%s is not managed by ADM; refusing to remove it", UnitPath)
	}
	_, _ = runSystemctl("disable", "--now", UnitName)
	if err := os.Remove(UnitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove systemd unit: %w", err)
	}
	_, err = runSystemctl("daemon-reload")
	return err
}

func inspectPlatform() (Status, error) {
	data, err := os.ReadFile(UnitPath)
	if errors.Is(err, os.ErrNotExist) {
		return Status{Supported: true, UnitPath: UnitPath}, nil
	}
	if err != nil {
		return Status{}, err
	}
	if !strings.Contains(string(data), "# Managed by AI Dev Manager.") {
		return Status{Supported: true, Installed: true, UnitPath: UnitPath, Detail: "unit exists but is not managed by ADM"}, nil
	}
	status := parseManagedUnit(string(data))
	enabledOutput, enabledErr := runSystemctl("is-enabled", UnitName)
	if enabledErr == nil && strings.TrimSpace(enabledOutput) == "enabled" {
		status.Enabled = true
	}
	output, activeErr := runSystemctl("is-active", UnitName)
	if activeErr == nil && strings.TrimSpace(output) == "active" {
		status.Active = true
		if pidOutput, pidErr := runSystemctl("show", "--property", "MainPID", "--value", UnitName); pidErr == nil {
			if pid, parseErr := strconv.Atoi(strings.TrimSpace(pidOutput)); parseErr == nil && pid > 0 {
				status.PID = pid
			}
		}
	} else {
		status.Detail = strings.TrimSpace(output)
	}
	return status, nil
}

func runSystemctl(args ...string) (string, error) {
	command := exec.Command("systemctl", args...)
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), text)
	}
	return text, nil
}
