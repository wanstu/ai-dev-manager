//go:build !windows && !linux && !darwin

package runtime

import "fmt"

// ListeningTCPPorts returns no facts on platforms without a bounded
// process-scoped observation implementation.
func ListeningTCPPorts(pid int) ([]int, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("process pid must be positive")
	}
	return nil, nil
}
