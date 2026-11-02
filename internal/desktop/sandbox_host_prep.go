package desktop

import (
	"errors"
	"fmt"
)

type SandboxHostPreparationStatus struct {
	Platform           string `json:"platform"`
	Supported          bool   `json:"supported"`
	NullDevicePrepared bool   `json:"null_device_prepared"`
	BootTaskInstalled  bool   `json:"boot_task_installed"`
	BootTaskVerified   bool   `json:"boot_task_verified"`
	ACLBrokerReady     bool   `json:"acl_broker_ready"`
	RequiresAdminSetup bool   `json:"requires_admin_setup"`
	Message            string `json:"message,omitempty"`
}

func (a *Adapter) GetSandboxHostPreparationStatus() SandboxHostPreparationStatus {
	return sandboxHostPreparationStatus()
}

func (a *Adapter) EnableSandboxHostPreparation() (SandboxHostPreparationStatus, error) {
	status, err := enableSandboxHostPreparation()
	if err != nil {
		return status, fmt.Errorf("enable strong sandbox: %w", err)
	}
	return status, nil
}

func actionableSandboxACLBrokerError(err error) error {
	if !errors.Is(err, errSandboxACLBrokerUnavailable) && !errors.Is(err, errSandboxACLBrokerTimeout) {
		return err
	}
	return fmt.Errorf("%w; open ADM Desktop Settings and repair Windows strong sandbox Host Prep, then accept the one-time administrator confirmation", err)
}
