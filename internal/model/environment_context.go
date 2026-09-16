package model

import "time"

type EnvironmentContextRequest struct {
	Path             string `json:"path,omitempty"`
	MaxDepth         int    `json:"max_depth,omitempty"`
	MaxEntries       int    `json:"max_entries,omitempty"`
	MaxDigestEntries int    `json:"max_digest_entries,omitempty"`
	MaxOutputBytes   int    `json:"max_output_bytes,omitempty"`
}

type EnvironmentContextLimits struct {
	MaxDepth         int `json:"max_depth"`
	MaxEntries       int `json:"max_entries"`
	MaxDigestEntries int `json:"max_digest_entries"`
	MaxOutputBytes   int `json:"max_output_bytes"`
}

type EnvironmentContextIdentity struct {
	EnvironmentID      string     `json:"environment_id"`
	EnvironmentName    string     `json:"environment_name"`
	WorkspaceID        string     `json:"workspace_id"`
	WorkspaceName      string     `json:"workspace_name"`
	Root               string     `json:"root"`
	State              string     `json:"state"`
	PrivateMemoryCount int        `json:"private_memory_count"`
	WriterPresent      bool       `json:"writer_present"`
	WriterExpiresAt    *time.Time `json:"writer_expires_at,omitempty"`
}

type EnvironmentContextTree struct {
	ScanPath             string                 `json:"scan_path"`
	ObservedAt           time.Time              `json:"observed_at"`
	Limits               DiscoveryLimits        `json:"limits"`
	VisitedEntries       int                    `json:"visited_entries"`
	ReadBatches          int                    `json:"read_batches"`
	Digest               []DirectoryDigestEntry `json:"digest"`
	Truncated            bool                   `json:"truncated"`
	StopReasons          []string               `json:"stop_reasons"`
	Coverage             string                 `json:"coverage"`
	SkippedDirectories   int                    `json:"skipped_directories"`
	SkippedLinks         int                    `json:"skipped_links"`
	OmittedDigestEntries int                    `json:"omitted_digest_entries"`
	OmittedDiagnostics   int                    `json:"omitted_diagnostics"`
}

type EnvironmentContextCapability struct {
	Key            string          `json:"key"`
	Kind           string          `json:"kind"`
	State          CapabilityState `json:"state"`
	ReasonCode     string          `json:"reason_code,omitempty"`
	Message        string          `json:"message,omitempty"`
	RequiresWriter bool            `json:"requires_writer"`
	Source         string          `json:"source,omitempty"`
	ObservedAt     *time.Time      `json:"observed_at,omitempty"`
	Freshness      string          `json:"freshness,omitempty"`
}

type EnvironmentContextMCP struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name,omitempty"`
	State                 CapabilityState `json:"state"`
	ReasonCode            string          `json:"reason_code,omitempty"`
	ObservationState      string          `json:"observation_state"`
	ToolInventoryObserved bool            `json:"tool_inventory_observed"`
	ToolNames             []string        `json:"tool_names"`
	ObservedAt            *time.Time      `json:"observed_at,omitempty"`
}

type EnvironmentContextSkill struct {
	ID                      string `json:"id"`
	Name                    string `json:"name,omitempty"`
	State                   string `json:"state"`
	Reason                  string `json:"reason,omitempty"`
	RelativeArtifactPath    string `json:"relative_artifact_path,omitempty"`
	SupportRootCount        int    `json:"support_root_count"`
	MissingSupportRootCount int    `json:"missing_support_root_count"`
}

type EnvironmentContextVerifier struct {
	ID             string          `json:"id"`
	Name           string          `json:"name,omitempty"`
	Kind           string          `json:"kind"`
	Enabled        bool            `json:"enabled"`
	State          CapabilityState `json:"state"`
	ReasonCode     string          `json:"reason_code,omitempty"`
	RequiresWriter bool            `json:"requires_writer"`
}

type EnvironmentContextGuidance struct {
	Operation      string          `json:"operation"`
	State          CapabilityState `json:"state"`
	RequiresWriter bool            `json:"requires_writer"`
	Message        string          `json:"message"`
}

type EnvironmentContextOmissions struct {
	TreeDigestEntries     int `json:"tree_digest_entries"`
	AvailableCapabilities int `json:"available_capabilities"`
	CapabilityIssues      int `json:"capability_issues"`
	MCPs                  int `json:"mcps"`
	MCPToolNames          int `json:"mcp_tool_names"`
	Skills                int `json:"skills"`
	Verifiers             int `json:"verifiers"`
	Guidance              int `json:"guidance"`
}

type EnvironmentContextBundle struct {
	Environment           EnvironmentContextIdentity     `json:"environment"`
	GeneratedAt           time.Time                      `json:"generated_at"`
	Limits                EnvironmentContextLimits       `json:"limits"`
	Tree                  EnvironmentContextTree         `json:"tree"`
	AvailableCapabilities []EnvironmentContextCapability `json:"available_capabilities"`
	CapabilityIssues      []EnvironmentContextCapability `json:"capability_issues"`
	MCPs                  []EnvironmentContextMCP        `json:"mcps"`
	Skills                []EnvironmentContextSkill      `json:"skills"`
	Verifiers             []EnvironmentContextVerifier   `json:"verifiers"`
	Guidance              []EnvironmentContextGuidance   `json:"guidance"`
	Omissions             EnvironmentContextOmissions    `json:"omissions"`
	Coverage              string                         `json:"coverage"`
	CoverageReasons       []string                       `json:"coverage_reasons"`
}
