//go:build !windows

package sandbox

import (
	"context"
	"fmt"
)

func launchPlatform(ctx context.Context, request LaunchRequest) (LaunchResult, error) {
	return LaunchResult{}, fmt.Errorf("strong sandbox process launch is unavailable on this platform")
}
