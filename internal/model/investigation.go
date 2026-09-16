package model

// EndpointInvestigationRequest describes one bounded, side-effect-free endpoint
// evidence lookup inside an Environment root.
type EndpointInvestigationRequest struct {
	Target          string `json:"target"`
	Method          string `json:"method,omitempty"`
	Path            string `json:"path,omitempty"`
	MaxFiles        int    `json:"max_files,omitempty"`
	MaxMatches      int    `json:"max_matches,omitempty"`
	MaxBytesPerFile int    `json:"max_bytes_per_file,omitempty"`
}

// EndpointInvestigationReport returns concrete file/line evidence plus an
// explicit confidence label and uncertainties. It does not encode a task plan.
type EndpointInvestigationReport struct {
	EnvironmentID  string                          `json:"environment_id"`
	Target         string                          `json:"target"`
	NormalizedPath string                          `json:"normalized_path"`
	Method         string                          `json:"method,omitempty"`
	SearchPath     string                          `json:"search_path,omitempty"`
	Queries        []string                        `json:"queries"`
	Confidence     string                          `json:"confidence"`
	Evidence       []EndpointInvestigationEvidence `json:"evidence"`
	Uncertainties  []string                        `json:"uncertainties,omitempty"`
	Truncated      bool                            `json:"truncated,omitempty"`
}

// EndpointInvestigationEvidence is one concrete route-like match.
type EndpointInvestigationEvidence struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Line    int      `json:"line"`
	Text    string   `json:"text"`
	Matched string   `json:"matched"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}
