//go:build windows

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
)

// requiresExplicitToolGrant returns false for Windows/application roots that
// normally already expose read/execute rights to AppContainer identities.
// Avoiding DACL mutation here is important because Program Files is commonly
// not WRITE_DAC-accessible to a non-elevated Gateway.
func requiresExplicitToolGrant(path string) bool {
	clean := filepath.Clean(path)
	roots := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("SystemRoot"),
		os.Getenv("WINDIR"),
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			continue
		}
		if pathWithin(absolute, clean) {
			return false
		}
	}
	return true
}
