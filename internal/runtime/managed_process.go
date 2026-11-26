package runtime

import (
	"os/exec"
	"time"
)

// ManagedProcess owns the OS-specific containment and resource observation
// for a long-running ADM Process. It is independent of Writer leases.
type ManagedProcess interface {
	Sample() (ProcessResources, error)
	Close()
}
type ProcessResources struct {
	CPUBasis     string    `json:"-"`
	SampledAt    time.Time `json:"sampled_at"`
	CPUTimeMS    uint64    `json:"cpu_time_ms"`
	CPUPercent   *float64  `json:"cpu_percent,omitempty"`
	MemoryBytes  uint64    `json:"memory_bytes"`
	ProcessCount int       `json:"process_count"`
	Scope        string    `json:"scope"`
	Complete     bool      `json:"complete"`
}

// StartManagedProcess takes a prepared command, retaining the execution
// policy, arguments, environment and output routing chosen by Runtime.
func StartManagedProcess(cmd *exec.Cmd) (ManagedProcess, error) {
	// Bound output copy waits when descendants inherit stdout/stderr handles.
	cmd.WaitDelay = 2 * time.Second
	return startManagedProcess(cmd)
}
