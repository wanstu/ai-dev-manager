//go:build windows

package sandbox

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	errorAlreadyExists            = 183
	errorNotFound                 = 1168
	hresultWin32Base       uint32 = 0x80070000
)

var (
	userenvDLL                         = windows.NewLazySystemDLL("userenv.dll")
	procCreateAppContainerProfile      = userenvDLL.NewProc("CreateAppContainerProfile")
	procDeriveAppContainerSIDFromName  = userenvDLL.NewProc("DeriveAppContainerSidFromAppContainerName")
	procDeleteAppContainerProfile      = userenvDLL.NewProc("DeleteAppContainerProfile")
)

func WindowsAppContainerAvailable() bool {
	return procCreateAppContainerProfile.Find() == nil &&
		procDeriveAppContainerSIDFromName.Find() == nil &&
		procDeleteAppContainerProfile.Find() == nil
}

func EnsureWindowsAppContainerProfile(name string) (AppContainerProfile, bool, error) {
	if !WindowsAppContainerAvailable() {
		return AppContainerProfile{}, false, fmt.Errorf("Windows AppContainer APIs are unavailable")
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return AppContainerProfile{}, false, err
	}
	displayPtr, err := windows.UTF16PtrFromString("AI Dev Manager Sandbox")
	if err != nil {
		return AppContainerProfile{}, false, err
	}
	descriptionPtr, err := windows.UTF16PtrFromString("AI Dev Manager isolated development runtime")
	if err != nil {
		return AppContainerProfile{}, false, err
	}

	var sid *windows.SID
	result, _, _ := procCreateAppContainerProfile.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(displayPtr)),
		uintptr(unsafe.Pointer(descriptionPtr)),
		0,
		0,
		uintptr(unsafe.Pointer(&sid)),
	)
	hr := uint32(result)
	created := false
	switch {
	case hr == 0:
		created = true
	case hr == hresultFromWin32(errorAlreadyExists):
		result, _, _ = procDeriveAppContainerSIDFromName.Call(
			uintptr(unsafe.Pointer(namePtr)),
			uintptr(unsafe.Pointer(&sid)),
		)
		hr = uint32(result)
		if hresultFailed(hr) {
			return AppContainerProfile{}, false, fmt.Errorf("DeriveAppContainerSidFromAppContainerName failed: HRESULT 0x%08x", hr)
		}
	default:
		return AppContainerProfile{}, false, fmt.Errorf("CreateAppContainerProfile failed: HRESULT 0x%08x", hr)
	}
	if sid == nil {
		return AppContainerProfile{}, false, fmt.Errorf("AppContainer profile returned an empty SID")
	}
	defer windows.FreeSid(sid)

	return AppContainerProfile{Name: name, SID: sid.String()}, created, nil
}

func DeleteWindowsAppContainerProfile(name string) error {
	if !WindowsAppContainerAvailable() {
		return fmt.Errorf("Windows AppContainer APIs are unavailable")
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	result, _, callErr := procDeleteAppContainerProfile.Call(uintptr(unsafe.Pointer(namePtr)))
	hr := uint32(result)
	if !hresultFailed(hr) || hr == hresultFromWin32(errorNotFound) {
		return nil
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("DeleteAppContainerProfile failed: HRESULT 0x%08x: %w", hr, callErr)
	}
	return fmt.Errorf("DeleteAppContainerProfile failed: HRESULT 0x%08x", hr)
}

func hresultFromWin32(code uint32) uint32 {
	if code == 0 {
		return 0
	}
	return hresultWin32Base | (code & 0xffff)
}

func hresultFailed(value uint32) bool {
	return int32(value) < 0
}
