//go:build windows

package secretvault

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows DPAPI binds ciphertext to the current Windows identity. No
// application-owned decryption key is written to the ADM state directory.
func protect(src []byte, _ string) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(src)), Data: &src[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func unprotect(src []byte, _ string) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty encrypted value")
	}
	in := windows.DataBlob{Size: uint32(len(src)), Data: &src[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
