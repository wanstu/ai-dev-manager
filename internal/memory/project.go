package memory

import (
	"fmt"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

// ProjectScope identifies a durable source using ADM's managed-worktree
// relationship, never by a guessed name, path, or common Git remote.
type ProjectScope struct {
	EnvironmentID        string   `json:"environment_id"`
	ProjectEnvironmentID string   `json:"project_environment_id"`
	SourceChain          []string `json:"source_chain"`
}

func projectScope(state *model.State, id string) (ProjectScope, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ProjectScope{}, fmt.Errorf("environment_id required")
	}
	out := ProjectScope{EnvironmentID: id, SourceChain: []string{}}
	seen := map[string]bool{}
	for current := id; current != ""; {
		if seen[current] {
			return ProjectScope{}, fmt.Errorf("cyclic worktree source")
		}
		seen[current] = true
		out.SourceChain = append(out.SourceChain, current)
		var env *model.Environment
		for i := range state.Environments {
			if state.Environments[i].ID == current {
				env = &state.Environments[i]
				break
			}
		}
		if env == nil {
			return ProjectScope{}, fmt.Errorf("source environment %q unavailable", current)
		}
		source := ""
		managed := false
		for _, w := range state.ManagedWorktrees {
			if w.EnvironmentID == current {
				if managed {
					return ProjectScope{}, fmt.Errorf("ambiguous worktree source")
				}
				managed = true
				source = w.SourceEnvironmentID
			}
		}
		if managed {
			if strings.TrimSpace(source) == "" {
				return ProjectScope{}, fmt.Errorf("worktree has no explicit source environment")
			}
			current = source
			continue
		}
		if env.Retention.Persistence == model.PersistenceTemporary {
			return ProjectScope{}, fmt.Errorf("no durable project source")
		}
		out.ProjectEnvironmentID = current
		return out, nil
	}
	return ProjectScope{}, fmt.Errorf("no durable project source")
}
func (s *Service) ProjectScope(id string) (ProjectScope, error) {
	state, err := s.store.Load()
	if err != nil {
		return ProjectScope{}, err
	}
	return projectScope(&state, id)
}

// Handoff copies selected current entries into a verified durable ancestor.
// It never overwrites or revives an existing destination key. Both sides are
// checked in the same Store transaction, and the source is left unchanged.
type HandoffItem struct {
	Key              string `json:"key"`
	TargetKey        string `json:"target_key,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	ExpectedValue    string `json:"expected_value"`
}
type HandoffResult struct {
	ProjectScope ProjectScope `json:"scope"`
	Copied       []string     `json:"copied"`
	DryRun       bool         `json:"dry_run"`
}

func (s *Service) Handoff(id, target string, items []HandoffItem, dryRun bool) (HandoffResult, error) {
	if strings.TrimSpace(target) == "" || len(items) == 0 || len(items) > maxChanges {
		return HandoffResult{}, fmt.Errorf("explicit target and 1 to %d items required", maxChanges)
	}
	total := 0
	for _, item := range items {
		total += len(item.Key) + len(item.TargetKey) + len(item.ExpectedValue)
		if total > maxChangeBytes {
			return HandoffResult{}, fmt.Errorf("handoff size limit exceeded")
		}
	}
	var out HandoffResult
	apply := func(state *model.State) error {
		project, err := projectScope(state, id)
		if err != nil {
			return err
		}
		if project.ProjectEnvironmentID != target || project.EnvironmentID == target {
			return fmt.Errorf("handoff requires distinct verified durable project destination")
		}
		source, sr, err := scope(state, project.EnvironmentID, false, false)
		if err != nil {
			return err
		}
		dest, dr, err := scope(state, target, false, true)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		type copied struct {
			key, value string
			metadata   model.MemoryMetadata
		}
		pending := []copied{}
		for _, item := range items {
			k, dk := strings.TrimSpace(item.Key), strings.TrimSpace(item.TargetKey)
			if dk == "" {
				dk = k
			}
			if k == "" || seen[dk] {
				return fmt.Errorf("invalid or duplicate destination key")
			}
			seen[dk] = true
			value, ok := (*source)[k]
			if !ok || value != item.ExpectedValue || sr[k].Revision != item.ExpectedRevision {
				return fmt.Errorf("source memory %q changed; reread", k)
			}
			if _, ok := (*dest)[dk]; ok {
				return fmt.Errorf("destination memory %q exists; use organize explicitly", dk)
			}
			if _, ok := dr[dk]; ok {
				return fmt.Errorf("destination memory %q has retained history; choose another key or restore explicitly", dk)
			}
			pending = append(pending, copied{dk, value, sr[k].Metadata})
		}
		out = HandoffResult{ProjectScope: project, Copied: []string{}, DryRun: dryRun}
		for _, item := range pending {
			applyVersion(dest, dr, item.key, &item.value, &item.metadata)
			out.Copied = append(out.Copied, item.key)
		}
		sort.Strings(out.Copied)
		return nil
	}
	if dryRun {
		state, err := s.store.Load()
		if err != nil {
			return HandoffResult{}, err
		}
		err = apply(&state)
		if err != nil {
			return HandoffResult{}, err
		}
		return out, nil
	}
	if err := s.store.Update(apply); err != nil {
		return HandoffResult{}, err
	}
	return out, nil
}
