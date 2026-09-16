package model

import "time"

type DiscoveryRequest struct {
	Path             string `json:"path,omitempty"`
	Query            string `json:"query,omitempty"`
	MaxDepth         int    `json:"max_depth,omitempty"`
	MaxEntries       int    `json:"max_entries,omitempty"`
	MaxCandidates    int    `json:"max_candidates,omitempty"`
	MaxDigestEntries int    `json:"max_digest_entries,omitempty"`
	MaxOutputBytes   int    `json:"max_output_bytes,omitempty"`
}
type DiscoveryScope struct {
	WorkspaceID   string `json:"workspace_id,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
}
type DiscoveryLimits struct {
	MaxDepth         int `json:"max_depth"`
	MaxEntries       int `json:"max_entries"`
	MaxCandidates    int `json:"max_candidates"`
	MaxDigestEntries int `json:"max_digest_entries"`
	MaxOutputBytes   int `json:"max_output_bytes"`
}
type ProjectMarker struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type ProjectCandidate struct {
	Root                     string          `json:"root"`
	Name                     string          `json:"name"`
	SuggestedEnvironmentRoot string          `json:"suggested_environment_root"`
	Evidence                 string          `json:"evidence"`
	QueryMatch               string          `json:"query_match"`
	Markers                  []ProjectMarker `json:"markers"`
}
type DirectoryDigestEntry struct {
	Path             string `json:"path"`
	Kind             string `json:"kind"`
	ObservedChildren int    `json:"observed_children"`
	ObservedMarkers  int    `json:"observed_markers"`
	ChildrenComplete bool   `json:"children_complete"`
}
type DiscoveryDiagnostic struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type DiscoveryReport struct {
	Scope                DiscoveryScope         `json:"scope"`
	ScanPath             string                 `json:"scan_path"`
	Query                string                 `json:"query"`
	ObservedAt           time.Time              `json:"observed_at"`
	Limits               DiscoveryLimits        `json:"limits"`
	VisitedEntries       int                    `json:"visited_entries"`
	ReadBatches          int                    `json:"read_batches"`
	Candidates           []ProjectCandidate     `json:"candidates"`
	Digest               []DirectoryDigestEntry `json:"digest"`
	Truncated            bool                   `json:"truncated"`
	StopReasons          []string               `json:"stop_reasons"`
	Diagnostics          []DiscoveryDiagnostic  `json:"diagnostics"`
	ExcludedDirectories  []string               `json:"excluded_directories"`
	SkippedDirectories   int                    `json:"skipped_directories"`
	SkippedLinks         int                    `json:"skipped_links"`
	OmittedCandidates    int                    `json:"omitted_candidates"`
	OmittedDigestEntries int                    `json:"omitted_digest_entries"`
	OmittedMarkers       int                    `json:"omitted_markers"`
	OmittedDiagnostics   int                    `json:"omitted_diagnostics"`
	Coverage             string                 `json:"coverage"`
}
