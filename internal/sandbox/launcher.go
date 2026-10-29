package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultLaunchOutputBytes = 256 * 1024
	maxLaunchOutputBytes     = 4 * 1024 * 1024
)

const CapabilityInternetClient = "internetClient"

type LaunchRequest struct {
	Executable     string    `json:"executable"`
	Args           []string  `json:"args,omitempty"`
	Cwd            string    `json:"cwd"`
	Environment    []string  `json:"environment,omitempty"`
	ProfileSID     string    `json:"profile_sid"`
	Capabilities   []string  `json:"capabilities,omitempty"`
	MaxOutputBytes int       `json:"max_output_bytes,omitempty"`
	Stdout         io.Writer `json:"-"`
	Stderr         io.Writer `json:"-"`
}

type LaunchResult struct {
	Started         bool   `json:"started"`
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
}

func (r LaunchRequest) Validate() error {
	if strings.TrimSpace(r.Executable) == "" {
		return fmt.Errorf("sandbox executable is required")
	}
	if !filepath.IsAbs(r.Executable) {
		return fmt.Errorf("sandbox executable must be an absolute path")
	}
	if strings.TrimSpace(r.Cwd) == "" || !filepath.IsAbs(r.Cwd) {
		return fmt.Errorf("sandbox cwd must be an absolute path")
	}
	if strings.TrimSpace(r.ProfileSID) == "" {
		return fmt.Errorf("sandbox AppContainer SID is required")
	}
	for _, capability := range r.Capabilities {
		if capability != CapabilityInternetClient {
			return fmt.Errorf("unsupported sandbox capability %q", capability)
		}
	}
	if r.MaxOutputBytes < 0 {
		return fmt.Errorf("sandbox max_output_bytes must be nonnegative")
	}
	return nil
}

func Launch(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	if err := request.Validate(); err != nil {
		return LaunchResult{}, err
	}
	return launchPlatform(ctx, request)
}

func launchOutputLimit(value int) int {
	if value <= 0 {
		return defaultLaunchOutputBytes
	}
	if value > maxLaunchOutputBytes {
		return maxLaunchOutputBytes
	}
	return value
}

type launchCapture struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
	mirror    io.Writer
}

func newLaunchCapture(limit int, mirror io.Writer) *launchCapture {
	return &launchCapture{limit: launchOutputLimit(limit), mirror: mirror}
}

func (c *launchCapture) Write(p []byte) (int, error) {
	original := len(p)
	c.mu.Lock()
	remaining := c.limit - c.buffer.Len()
	if remaining > 0 {
		part := p
		if len(part) > remaining {
			part = part[:remaining]
		}
		_, _ = c.buffer.Write(part)
	}
	if original > remaining {
		c.truncated = true
	}
	mirror := c.mirror
	c.mu.Unlock()

	if mirror != nil {
		if _, err := mirror.Write(p); err != nil {
			return 0, err
		}
	}
	return original, nil
}

func (c *launchCapture) Snapshot() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buffer.String(), c.truncated
}
