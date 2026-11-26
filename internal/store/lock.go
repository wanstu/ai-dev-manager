package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const stateLockTimeout = 20 * time.Second
const stateLockRetry = 25 * time.Millisecond

// The sidecar's identity is stable across atomic replacement of state.json.
// Never remove the sidecar during unlock: another process might still have an
// open descriptor to it. The OS releases the lock when the descriptor closes
// or the process dies. It is independent of the ADM Writer lease.
func acquireStateLock(path string) (*os.File, error) {
	return acquireStateLockWithin(path, stateLockTimeout)
}
func acquireStateLockWithin(path string, timeout time.Duration) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("prepare state directory: %w", err)
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open state lock: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		acquired, err := tryStateLock(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("acquire state lock: %w", err)
		}
		if acquired {
			return f, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			f.Close()
			return nil, fmt.Errorf("state lock busy: timed out after %s", timeout)
		}
		time.Sleep(min(remaining, stateLockRetry))
	}
}
