//go:build windows

package pathutil

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func normalizeExistingPath(path string) string {
	clean := filepath.Clean(path)
	input, err := windows.UTF16PtrFromString(clean)
	if err != nil {
		return clean
	}
	size, err := windows.GetLongPathName(input, nil, 0)
	if err != nil || size == 0 {
		return clean
	}
	buf := make([]uint16, size)
	n, err := windows.GetLongPathName(input, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		return clean
	}
	return filepath.Clean(windows.UTF16ToString(buf))
}
