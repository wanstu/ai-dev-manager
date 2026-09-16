package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const maxExecDenials = 200

// ExecDenials returns blocked executable observations sorted by count desc and
// recency desc. It intentionally records only the executable and lightweight
// source metadata, never args or stdout/stderr.
func (s *Service) ExecDenials() ([]model.ExecDenial, error) {
	state, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	items := normalizeExecDenials(state.ExecDenials)
	return append([]model.ExecDenial(nil), items...), nil
}

func (s *Service) ClearExecDenial(executable string) ([]model.ExecDenial, error) {
	executable = normalizeExecutableName(executable)
	if executable == "" {
		return nil, fmt.Errorf("executable is required")
	}
	if err := s.Store.Update(func(state *model.State) error {
		state.ExecDenials = removeExecDenial(state.ExecDenials, executable)
		return nil
	}); err != nil {
		return nil, err
	}
	return s.ExecDenials()
}

func (s *Service) ClearExecDenials() error {
	return s.Store.Update(func(state *model.State) error {
		state.ExecDenials = nil
		return nil
	})
}

func (s *Service) recordExecDenial(environmentID, executable, surface, reason string) {
	if !isExecutableNotAllowedErrorText(reason) {
		return
	}
	_ = s.RecordExecDenial(environmentID, executable, surface, reason)
}

func (s *Service) RecordExecDenial(environmentID, executable, surface, reason string) error {
	executable = normalizeExecutableName(executable)
	if executable == "" {
		return nil
	}
	environmentID = strings.TrimSpace(environmentID)
	surface = strings.TrimSpace(surface)
	reason = strings.TrimSpace(reason)
	now := time.Now().UTC()
	return s.Store.Update(func(state *model.State) error {
		for i := range state.ExecDenials {
			if strings.EqualFold(state.ExecDenials[i].Executable, executable) {
				if state.ExecDenials[i].FirstBlockedAt.IsZero() {
					state.ExecDenials[i].FirstBlockedAt = now
				}
				state.ExecDenials[i].Executable = executable
				state.ExecDenials[i].Count++
				state.ExecDenials[i].LastBlockedAt = now
				state.ExecDenials[i].LastEnvironmentID = environmentID
				state.ExecDenials[i].LastSurface = surface
				state.ExecDenials[i].LastReason = reason
				state.ExecDenials = normalizeExecDenials(state.ExecDenials)
				return nil
			}
		}
		state.ExecDenials = append(state.ExecDenials, model.ExecDenial{
			Executable:        executable,
			Count:             1,
			FirstBlockedAt:    now,
			LastBlockedAt:     now,
			LastEnvironmentID: environmentID,
			LastSurface:       surface,
			LastReason:        reason,
		})
		state.ExecDenials = normalizeExecDenials(state.ExecDenials)
		return nil
	})
}

func normalizeExecutableName(executable string) string {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return ""
	}
	if filepath.IsAbs(executable) {
		return filepath.Clean(executable)
	}
	return executable
}

func isExecutableNotAllowedError(err error) bool {
	if err == nil {
		return false
	}
	return isExecutableNotAllowedErrorText(err.Error())
}

func isExecutableNotAllowedErrorText(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "executable") && strings.Contains(message, "not allowed")
}

func removeExecDenial(items []model.ExecDenial, executable string) []model.ExecDenial {
	out := items[:0]
	for _, item := range items {
		if strings.EqualFold(item.Executable, executable) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeExecDenials(items []model.ExecDenial) []model.ExecDenial {
	if len(items) == 0 {
		return nil
	}
	byExecutable := make(map[string]model.ExecDenial, len(items))
	for _, item := range items {
		name := normalizeExecutableName(item.Executable)
		if name == "" {
			continue
		}
		if item.Count <= 0 {
			item.Count = 1
		}
		item.Executable = name
		current, ok := byExecutable[strings.ToLower(name)]
		if !ok {
			byExecutable[strings.ToLower(name)] = item
			continue
		}
		current.Count += item.Count
		if current.FirstBlockedAt.IsZero() || (!item.FirstBlockedAt.IsZero() && item.FirstBlockedAt.Before(current.FirstBlockedAt)) {
			current.FirstBlockedAt = item.FirstBlockedAt
		}
		if item.LastBlockedAt.After(current.LastBlockedAt) {
			current.LastBlockedAt = item.LastBlockedAt
			current.LastEnvironmentID = item.LastEnvironmentID
			current.LastSurface = item.LastSurface
			current.LastReason = item.LastReason
		}
		byExecutable[strings.ToLower(name)] = current
	}
	out := make([]model.ExecDenial, 0, len(byExecutable))
	for _, item := range byExecutable {
		out = append(out, item)
	}
	sortExecDenials(out)
	if len(out) > maxExecDenials {
		out = out[:maxExecDenials]
	}
	return out
}

func sortExecDenials(items []model.ExecDenial) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		if !items[i].LastBlockedAt.Equal(items[j].LastBlockedAt) {
			return items[i].LastBlockedAt.After(items[j].LastBlockedAt)
		}
		return strings.ToLower(items[i].Executable) < strings.ToLower(items[j].Executable)
	})
}
