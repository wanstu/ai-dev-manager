//go:build windows

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type PathAccess string

const (
	PathAccessTraverse  PathAccess = "traverse"
	PathAccessReadOnly  PathAccess = "read_only"
	PathAccessReadWrite PathAccess = "read_write"

	objectInheritACE    = 0x01
	containerInheritACE = 0x02
)

func windowsPathAccessMask(access PathAccess) (windows.ACCESS_MASK, error) {
	if access == PathAccessTraverse {
		// FILE_GENERIC_EXECUTE on a directory includes FILE_TRAVERSE,
		// FILE_READ_ATTRIBUTES and SYNCHRONIZE, but not FILE_LIST_DIRECTORY.
		return windows.ACCESS_MASK(windows.FILE_GENERIC_EXECUTE), nil
	}
	mask := windows.ACCESS_MASK(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE)
	if access == PathAccessReadWrite {
		const fileDeleteChild = 0x00000040
		mask |= windows.ACCESS_MASK(windows.FILE_GENERIC_WRITE | windows.DELETE | fileDeleteChild)
	} else if access != PathAccessReadOnly {
		return 0, fmt.Errorf("unsupported sandbox path access %q", access)
	}
	return mask, nil
}

// WindowsAppContainerPathAccessGranted reports whether the path already has a
// sufficient allow ACE for the AppContainer SID. This is intentionally
// read-only so ordinary Gateway execution can reuse a one-time elevated grant
// without requiring WRITE_DAC again.
func WindowsAppContainerPathAccessGranted(path, sidText string, access PathAccess) (bool, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	sid, err := windows.StringToSid(sidText)
	if err != nil {
		return false, fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var pinner runtime.Pinner
	pinner.Pin(sid)
	defer pinner.Unpin()

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, fmt.Errorf("read DACL for %s: %w", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return false, fmt.Errorf("read DACL for %s: %w", path, err)
	}
	if dacl == nil {
		return true, nil
	}
	requiredMask, err := windowsPathAccessMask(access)
	if err != nil {
		return false, err
	}
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil || ace == nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !windows.EqualSid(aceSID, sid) {
			continue
		}
		if windows.ACCESS_MASK(ace.Mask)&requiredMask != requiredMask {
			continue
		}
		if info.IsDir() && access != PathAccessTraverse {
			requiredFlags := byte(objectInheritACE | containerInheritACE)
			if ace.Header.AceFlags&requiredFlags != requiredFlags {
				continue
			}
		}
		return true, nil
	}
	return false, nil
}

// EnsureWindowsAppContainerPath grants access only when the required ACE is not
// already present. changed is false when a previous administrator or Gateway
// preparation can be safely reused.
func EnsureWindowsAppContainerPath(path, sidText string, access PathAccess) (changed bool, err error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	granted, err := WindowsAppContainerPathAccessGranted(path, sidText, access)
	if err != nil {
		return false, err
	}
	if granted {
		return false, nil
	}
	sid, err := windows.StringToSid(sidText)
	if err != nil {
		return false, fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var pinner runtime.Pinner
	pinner.Pin(sid)
	defer pinner.Unpin()

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, fmt.Errorf("read DACL for %s: %w", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return false, fmt.Errorf("read DACL for %s: %w", path, err)
	}
	mask, err := windowsPathAccessMask(access)
	if err != nil {
		return false, err
	}

	inheritance := uint32(windows.NO_INHERITANCE)
	if info.IsDir() && access != PathAccessTraverse {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entry := windows.EXPLICIT_ACCESS{
		AccessPermissions: mask,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
			TrusteeForm:              windows.TRUSTEE_IS_SID,
			TrusteeType:              windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue:             windows.TrusteeValueFromSID(sid),
		},
	}
	updated, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, dacl)
	if err != nil {
		return false, fmt.Errorf("build DACL for %s: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, updated, nil); err != nil {
		return false, fmt.Errorf("grant AppContainer access to %s: %w", path, err)
	}
	return true, nil
}

func GrantWindowsAppContainerPath(path, sidText string, access PathAccess) error {
	_, err := EnsureWindowsAppContainerPath(path, sidText, access)
	return err
}

// WindowsPathDACLWritable reports whether the current Gateway token can modify
// the path DACL without changing it. It is used only for readiness reporting;
// Access denied is a normal "requires administrator preparation" outcome.
func WindowsPathDACLWritable(path string) (bool, error) {
	const (
		readControlAccess = 0x00020000
		writeDACAccess    = 0x00040000
	)
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	flags := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if info.IsDir() {
		flags = windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	handle, err := windows.CreateFile(
		pathPtr,
		readControlAccess|writeDACAccess,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		flags,
		0,
	)
	if err != nil {
		if err == windows.ERROR_ACCESS_DENIED {
			return false, nil
		}
		return false, fmt.Errorf("open %s for WRITE_DAC probe: %w", path, err)
	}
	_ = windows.CloseHandle(handle)
	return true, nil
}

func RevokeWindowsAppContainerPath(path, sidText string) error {
	path = filepath.Clean(path)
	sid, err := windows.StringToSid(sidText)
	if err != nil {
		return fmt.Errorf("parse AppContainer SID: %w", err)
	}
	var pinner runtime.Pinner
	pinner.Pin(sid)
	defer pinner.Unpin()

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read DACL for %s: %w", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("read DACL for %s: %w", path, err)
	}
	entry := windows.EXPLICIT_ACCESS{
		AccessMode:  windows.REVOKE_ACCESS,
		Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			MultipleTrusteeOperation: windows.NO_MULTIPLE_TRUSTEE,
			TrusteeForm:              windows.TRUSTEE_IS_SID,
			TrusteeType:              windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue:             windows.TrusteeValueFromSID(sid),
		},
	}
	updated, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, dacl)
	if err != nil {
		return fmt.Errorf("build revoked DACL for %s: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, updated, nil); err != nil {
		return fmt.Errorf("revoke AppContainer access from %s: %w", path, err)
	}
	return nil
}
