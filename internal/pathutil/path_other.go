//go:build !windows

package pathutil

import "path/filepath"

func normalizeExistingPath(path string) string {
	return filepath.Clean(path)
}
