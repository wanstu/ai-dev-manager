package model

import "time"

type CapabilityState string

const (
	CapabilityStateAvailable    CapabilityState = "available"
	CapabilityStateDisabled     CapabilityState = "disabled"
	CapabilityStateUnconfigured CapabilityState = "unconfigured"
	CapabilityStateUnavailable  CapabilityState = "unavailable"
	CapabilityStateDegraded     CapabilityState = "degraded"
)

type CapabilityReport struct {
	EnvironmentID string           `json:"environment_id"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Facts         []CapabilityFact `json:"facts"`
}

type CapabilityFact struct {
	Key            string               `json:"key"`
	Kind           string               `json:"kind"`
	State          CapabilityState      `json:"state"`
	ReasonCode     string               `json:"reason_code,omitempty"`
	Message        string               `json:"message,omitempty"`
	RequiresWriter bool                 `json:"requires_writer"`
	Evidence       []CapabilityEvidence `json:"evidence,omitempty"`
	Source         string               `json:"source,omitempty"`
	ObservedAt     *time.Time           `json:"observed_at,omitempty"`
	Freshness      string               `json:"freshness,omitempty"`
	Confidence     string               `json:"confidence,omitempty"`
	Uncertainties  []string             `json:"uncertainties,omitempty"`
}

type InvestigationProviderReport struct {
	EnvironmentID string           `json:"environment_id"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Providers     []CapabilityFact `json:"providers"`
}

type CapabilityEvidence struct {
	Kind    string            `json:"kind"`
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name,omitempty"`
	Path    string            `json:"path,omitempty"`
	State   string            `json:"state,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}
