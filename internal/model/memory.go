package model

import "time"

// MemoryMetadata is descriptive data, not an authorization mechanism.
type MemoryMetadata struct {
	Status      string     `json:"status,omitempty"`
	ValidUntil  *time.Time `json:"valid_until,omitempty"`
	ReviewAfter *time.Time `json:"review_after,omitempty"`
	ReviewNote  string     `json:"review_note,omitempty"`
	Category    string     `json:"category,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Importance  int        `json:"importance,omitempty"`
}
type MemoryVersion struct {
	Revision  int64          `json:"revision"`
	Value     *string        `json:"value"`
	Metadata  MemoryMetadata `json:"metadata"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
}
type MemoryRecord struct {
	Revision  int64           `json:"revision"`
	Metadata  MemoryMetadata  `json:"metadata"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
	History   []MemoryVersion `json:"history,omitempty"`
}
