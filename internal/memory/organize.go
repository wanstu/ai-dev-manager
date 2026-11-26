package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

// SearchResult is bounded in serialized bytes. Large entries can be read by key.
type SearchResult struct {
	Entries []Entry `json:"entries"`
	Total   int     `json:"total_matches"`
	Omitted int     `json:"omitted"`
}

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	defaultSearchBytes = 16384
	maxSearchBytes     = 65536
	maxChanges         = 50
	maxChangeBytes     = 262144
)

func (s *Service) GlobalSearch(query string, limit, maxBytes int) (SearchResult, error) {
	items, err := s.GlobalList()
	if err != nil {
		return SearchResult{}, err
	}
	return search(items, query, limit, maxBytes)
}

func (s *Service) EnvironmentSearch(id, query string, limit, maxBytes int) (SearchResult, error) {
	items, err := s.EnvironmentList(id)
	if err != nil {
		return SearchResult{}, err
	}
	return search(items, query, limit, maxBytes)
}

func search(items []Entry, query string, limit, maxBytes int) (SearchResult, error) {
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if maxBytes == 0 {
		maxBytes = defaultSearchBytes
	}
	if limit < 1 || limit > maxSearchLimit || maxBytes < 256 || maxBytes > maxSearchBytes || len(query) > 4096 {
		return SearchResult{}, fmt.Errorf("memory search bounds exceeded")
	}
	terms := strings.Fields(strings.ToLower(query))
	type ranked struct {
		entry Entry
		score int
	}
	matches := make([]ranked, 0)
	for _, item := range items {
		key, value := strings.ToLower(item.Key), strings.ToLower(item.Value)
		if item.Metadata != nil {
			key += " " + item.Metadata.Category + " " + strings.Join(item.Metadata.Tags, " ")
		}
		score, matched := 0, true
		for _, term := range terms {
			if strings.Contains(key, term) {
				score += 2
			} else if strings.Contains(value, term) {
				score++
			} else {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, ranked{item, score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].entry.Key < matches[j].entry.Key
	})
	out := SearchResult{Entries: []Entry{}, Total: len(matches), Omitted: len(matches)}
	for _, item := range matches {
		if len(out.Entries) >= limit {
			break
		}
		out.Entries = append(out.Entries, item.entry)
		out.Omitted--
		b, err := json.Marshal(out)
		if err != nil {
			return SearchResult{}, err
		}
		if len(b) > maxBytes {
			out.Entries = out.Entries[:len(out.Entries)-1]
			out.Omitted++
		}
	}
	return out, nil
}

// Change is a compare-and-set operation. Null/absent expected_value means the
// key must not exist. Null/absent value means delete; an empty string is valid.
type Change struct {
	Key              string                `json:"key"`
	ExpectedValue    *string               `json:"expected_value"`
	Value            *string               `json:"value"`
	Metadata         *model.MemoryMetadata `json:"metadata,omitempty"`
	ExpectedRevision *int64                `json:"expected_revision,omitempty"`
}
type OrganizeResult struct {
	Created int  `json:"created"`
	Updated int  `json:"updated"`
	Deleted int  `json:"deleted"`
	DryRun  bool `json:"dry_run"`
}

func (s *Service) GlobalOrganize(changes []Change, dryRun bool) (OrganizeResult, error) {
	return s.organize("", true, changes, dryRun)
}
func (s *Service) EnvironmentOrganize(id string, changes []Change, dryRun bool) (OrganizeResult, error) {
	if strings.TrimSpace(id) == "" {
		return OrganizeResult{}, fmt.Errorf("environment_id is required")
	}
	return s.organize(id, false, changes, dryRun)
}

func (s *Service) organize(id string, global bool, changes []Change, dryRun bool) (OrganizeResult, error) {
	if len(changes) == 0 || len(changes) > maxChanges {
		return OrganizeResult{}, fmt.Errorf("memory organize requires 1 to %d changes", maxChanges)
	}
	encoded, err := json.Marshal(changes)
	if err != nil || len(encoded) > maxChangeBytes {
		return OrganizeResult{}, fmt.Errorf("memory organize input exceeds byte limit")
	}
	seen := map[string]bool{}
	normalized := append([]Change(nil), changes...)
	for i, c := range normalized {
		key := strings.TrimSpace(c.Key)
		if key == "" || seen[key] || (c.ExpectedValue == nil && c.Value == nil) {
			return OrganizeResult{}, fmt.Errorf("invalid or duplicate memory change at index %d", i)
		}
		seen[key] = true
		normalized[i].Key = key
		if c.Metadata != nil {
			if c.Value == nil || c.ExpectedRevision == nil {
				return OrganizeResult{}, fmt.Errorf("metadata changes require value and expected_revision")
			}
			m, err := normalizeMetadata(*c.Metadata)
			if err != nil {
				return OrganizeResult{}, err
			}
			normalized[i].Metadata = &m
		}
	}
	var out OrganizeResult
	apply := func(state *model.State) error {
		target, records, err := scope(state, id, global, true)
		if err != nil {
			return err
		}
		// Validate every precondition before touching any entry.
		for _, c := range normalized {
			old, exists := (*target)[c.Key]
			if c.ExpectedRevision != nil && records[c.Key].Revision != *c.ExpectedRevision {
				return fmt.Errorf("memory revision conflict for key %q", c.Key)
			}
			if (c.ExpectedValue == nil && exists) || (c.ExpectedValue != nil && (!exists || old != *c.ExpectedValue)) {
				return fmt.Errorf("memory conflict for key %q; reread before organizing", c.Key)
			}
		}
		out = OrganizeResult{DryRun: dryRun}
		if *target == nil {
			*target = map[string]string{}
		}
		for _, c := range normalized {
			if c.Value == nil {
				applyVersion(target, records, c.Key, nil, nil)
				out.Deleted++
			} else {
				if c.ExpectedValue == nil {
					out.Created++
				} else {
					out.Updated++
				}
				applyVersion(target, records, c.Key, c.Value, c.Metadata)
			}
		}
		return nil
	}
	if dryRun {
		state, err := s.store.Load()
		if err != nil {
			return OrganizeResult{}, err
		}
		err = apply(&state)
		if err != nil {
			return OrganizeResult{}, err
		}
		return out, nil
	}
	if err := s.store.Update(apply); err != nil {
		return OrganizeResult{}, err
	}
	return out, nil
}
