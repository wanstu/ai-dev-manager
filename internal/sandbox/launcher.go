package sandbox

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type LaunchRequest struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args,omitempty"`
	Cwd        string   `json:"cwd"`
	Environment []string `json:"environment,omitempty"`
	ProfileSID string   `json:"profile_sid"`
}

type LaunchResult struct {
	ExitCode int `json:"exit_code"`
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
	return nil
}

func Launch(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	if err := request.Validate(); err != nil {
		return LaunchResult{}, err
	}
	return launchPlatform(ctx, request)
}
