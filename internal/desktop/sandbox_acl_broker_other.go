//go:build !windows

package desktop

import (
	"context"
	"errors"

	"ai-dev-manager-v2/internal/sandbox"
)

var (
	errSandboxACLBrokerUnavailable = errors.New("sandbox ACL broker is unavailable")
	errSandboxACLBrokerTimeout     = errors.New("sandbox ACL broker request timed out")
)

type sandboxACLBrokerRequest struct {
	Operation       string
	EnvironmentID   string
	ProfileSID      string
	WorkspaceRoot   string
	EnvironmentRoot string
}

func RunSandboxACLBroker() error {
	return errSandboxACLBrokerUnavailable
}

func requestSandboxACLBroker(context.Context, sandboxACLBrokerRequest) (sandbox.EnvironmentAccessStatus, error) {
	return sandbox.EnvironmentAccessStatus{}, errSandboxACLBrokerUnavailable
}

func sandboxACLBrokerReady() bool { return false }
