//go:build !windows

package hostenv

import "os"

func readHostEnvironment() (map[string]string, string, error) {
	return environmentMap(os.Environ()), "process_environment", nil
}
