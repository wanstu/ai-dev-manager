package sandbox

// HostPreparationReport describes machine-wide preparation required by the
// strong sandbox backend. Inspection is read-only; mutation is exposed only by
// the separate privileged host-prep helper.
type HostPreparationReport struct {
	Platform           string `json:"platform"`
	NullDevicePrepared bool   `json:"null_device_prepared"`
	RequiresAdminSetup bool   `json:"requires_admin_setup"`
	Message            string `json:"message,omitempty"`
}
