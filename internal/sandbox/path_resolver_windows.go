//go:build windows

package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func resolveExistingPath(path string) (string, error) {
	pathPtr, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		pathPtr,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", fmt.Errorf("open path for final-name resolution: %w", err)
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, 512)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", fmt.Errorf("resolve final path name: %w", err)
		}
		if n < uint32(len(buffer)) {
			resolved := windows.UTF16ToString(buffer[:n])
			if strings.HasPrefix(resolved, `\\?\UNC\`) {
				resolved = `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`)
			} else {
				resolved = strings.TrimPrefix(resolved, `\\?\`)
			}
			return filepath.Clean(resolved), nil
		}
		buffer = make([]uint16, n+1)
	}
}
