//go:build windows

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows"
)

type PathAccess string

const (
	PathAccessReadOnly  PathAccess = "read_only"
	PathAccessReadWrite PathAccess = "read_write"
)

func GrantWindowsAppContainerPath(path, sidText string, access PathAccess) error {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
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

	mask := windows.ACCESS_MASK(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE)
	if access == PathAccessReadWrite {
		const fileDeleteChild = 0x00000040
		mask |= windows.ACCESS_MASK(windows.FILE_GENERIC_WRITE | windows.DELETE | fileDeleteChild)
	} else if access != PathAccessReadOnly {
		return fmt.Errorf("unsupported sandbox path access %q", access)
	}

	inheritance := uint32(windows.NO_INHERITANCE)
	if info.IsDir() {
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
		return fmt.Errorf("build DACL for %s: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, updated, nil); err != nil {
		return fmt.Errorf("grant AppContainer access to %s: %w", path, err)
	}
	return nil
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
