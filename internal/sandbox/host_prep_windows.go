//go:build windows

package sandbox

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	nullDeviceTargetSDDL = "O:BAG:SYD:(A;;GRGWGX;;;WD)(A;;FA;;;SY)(A;;FA;;;BA)(A;;GRGX;;;RC)(A;;GRGWGX;;;AC)(A;;GRGWGX;;;S-1-15-2-2)S:(ML;;NW;;;LW)"
	readControl           = 0x00020000
	writeDAC              = 0x00040000
)

// InspectHostPreparation performs a read-only probe. It intentionally checks
// the AppContainer package SIDs rather than Everyone: Windows can grant NUL to
// Everyone while still denying classic AppContainer tokens.
func InspectHostPreparation() HostPreparationReport {
	prepared, err := WindowsNullDevicePrepared()
	report := HostPreparationReport{
		Platform:           "windows",
		NullDevicePrepared: prepared,
		RequiresAdminSetup: !prepared,
	}
	if err != nil {
		report.Message = err.Error()
	} else if !prepared {
		report.Message = "Windows null device is not prepared for AppContainer development tools"
	}
	return report
}

// WindowsNullDevicePrepared returns true when both well-known AppContainer
// package groups have an explicit non-zero allow ACE on \\Device\\Null.
func WindowsNullDevicePrepared() (bool, error) {
	handle, err := openWindowsNullDevice(windows.GENERIC_READ | readControl)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)

	sd, err := windows.GetSecurityInfo(
		handle,
		windows.SE_KERNEL_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return false, fmt.Errorf("read Windows null device DACL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		if errors.Is(err, windows.ERROR_OBJECT_NOT_FOUND) {
			// No DACL means unrestricted access.
			return true, nil
		}
		return false, fmt.Errorf("read Windows null device ACL: %w", err)
	}
	if dacl == nil {
		// A NULL DACL grants full access.
		return true, nil
	}

	allPackages, err := windows.StringToSid("S-1-15-2-1")
	if err != nil {
		return false, fmt.Errorf("parse ALL APPLICATION PACKAGES SID: %w", err)
	}
	allRestrictedPackages, err := windows.StringToSid("S-1-15-2-2")
	if err != nil {
		return false, fmt.Errorf("parse ALL RESTRICTED APPLICATION PACKAGES SID: %w", err)
	}
	wanted := []*windows.SID{allPackages, allRestrictedPackages}
	granted := [2]bool{}

	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil || ace == nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask == 0 {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		for wantedIndex, sid := range wanted {
			if windows.EqualSid(aceSID, sid) {
				granted[wantedIndex] = true
			}
		}
	}
	return granted[0] && granted[1], nil
}

// PrepareNullDeviceHost applies the documented Windows AppContainer-compatible
// DACL to the null device. It mutates machine-wide kernel-object security and
// therefore requires an elevated process. The kernel resets this descriptor on
// reboot, so callers should arrange a privileged boot-time reapply.
func PrepareNullDeviceHost() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("Windows sandbox host preparation requires an elevated administrator process")
	}
	if prepared, err := WindowsNullDevicePrepared(); err == nil && prepared {
		return nil
	}

	target, err := windows.SecurityDescriptorFromString(nullDeviceTargetSDDL)
	if err != nil {
		return fmt.Errorf("parse Windows null device target security descriptor: %w", err)
	}
	handle, err := openWindowsNullDevice(windows.GENERIC_READ | readControl | writeDAC)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	// DACL-only mode mirrors the no-SACL CI preparation used by Microsoft's
	// AppContainer tooling. It avoids SeSecurityPrivilege while granting only
	// the device access needed by AppContainer child-process plumbing.
	if err := windows.SetKernelObjectSecurity(handle, windows.DACL_SECURITY_INFORMATION, target); err != nil {
		return fmt.Errorf("apply Windows null device DACL: %w", err)
	}
	prepared, err := WindowsNullDevicePrepared()
	if err != nil {
		return err
	}
	if !prepared {
		return fmt.Errorf("Windows null device DACL was applied but AppContainer grants are still missing")
	}
	return nil
}

func openWindowsNullDevice(access uint32) (windows.Handle, error) {
	path, err := windows.UTF16PtrFromString(`\\.\NUL`)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(
		path,
		access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("open Windows null device: %w", err)
	}
	return handle, nil
}
