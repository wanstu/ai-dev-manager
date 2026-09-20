package app

import (
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
)

const (
	maxExecUsages                = 500
	maxExecUsageHourlyBuckets    = 48
	execUsageUnattributedSurface = "unattributed"
)

// ExecUsages returns persistent per-executable launch counts. It stores only
// executable identity and lightweight source metadata, never args/output.
// HourlyCounts is a bounded recent-frequency summary, not an execution log.
func (s *Service) ExecUsages() ([]model.ExecUsage, error) {
	state, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	items := normalizeExecUsages(state.ExecUsages)
	out := make([]model.ExecUsage, 0, len(items))
	for _, item := range items {
		item.SurfaceCounts = cloneExecSurfaceCounts(item.SurfaceCounts)
		item.HourlyCounts = cloneExecUsageHours(item.HourlyCounts)
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) ExecUsageList() ([]model.ExecUsage, error) {
	return s.ExecUsages()
}

// RecordExecUsage records one command launch/launch-attempt after ADM execution
// authority has already accepted the executable. Arguments are intentionally
// not accepted so they cannot be persisted by this path.
func (s *Service) RecordExecUsage(environmentID, executable, surface string) error {
	executable = normalizeExecutableName(executable)
	if executable == "" {
		return nil
	}
	environmentID = strings.TrimSpace(environmentID)
	surface = normalizeExecSurfaceName(surface)
	now := time.Now().UTC()
	hour := now.Truncate(time.Hour)
	return s.Store.Update(func(state *model.State) error {
		state.ExecUsages = normalizeExecUsages(state.ExecUsages)
		for i := range state.ExecUsages {
			if strings.EqualFold(state.ExecUsages[i].Executable, executable) {
				if state.ExecUsages[i].FirstExecutedAt.IsZero() {
					state.ExecUsages[i].FirstExecutedAt = now
				}
				if state.ExecUsages[i].SurfaceCounts == nil {
					state.ExecUsages[i].SurfaceCounts = map[string]int{}
				}
				state.ExecUsages[i].Executable = executable
				state.ExecUsages[i].Count++
				state.ExecUsages[i].SurfaceCounts[surface]++
				recordExecUsageHour(&state.ExecUsages[i], hour, surface)
				state.ExecUsages[i].LastExecutedAt = now
				state.ExecUsages[i].LastEnvironmentID = environmentID
				state.ExecUsages[i].LastSurface = surface
				state.ExecUsages = normalizeExecUsages(state.ExecUsages)
				return nil
			}
		}
		state.ExecUsages = append(state.ExecUsages, model.ExecUsage{
			Executable:        executable,
			Count:             1,
			FirstExecutedAt:   now,
			LastExecutedAt:    now,
			LastEnvironmentID: environmentID,
			LastSurface:       surface,
			SurfaceCounts:     map[string]int{surface: 1},
			HourlyCounts: []model.ExecUsageHour{{
				Hour:          hour,
				Count:         1,
				SurfaceCounts: map[string]int{surface: 1},
			}},
		})
		state.ExecUsages = normalizeExecUsages(state.ExecUsages)
		return nil
	})
}

func normalizeExecUsages(items []model.ExecUsage) []model.ExecUsage {
	if len(items) == 0 {
		return nil
	}
	now := time.Now().UTC()
	byExecutable := make(map[string]model.ExecUsage, len(items))
	for _, item := range items {
		name := normalizeExecutableName(item.Executable)
		if name == "" {
			continue
		}
		item.Executable = name
		normalizeExecUsageSurfaceCounts(&item)
		item.HourlyCounts = normalizeExecUsageHours(item.HourlyCounts, now)
		key := strings.ToLower(name)
		current, ok := byExecutable[key]
		if !ok {
			item.SurfaceCounts = cloneExecSurfaceCounts(item.SurfaceCounts)
			item.HourlyCounts = cloneExecUsageHours(item.HourlyCounts)
			byExecutable[key] = item
			continue
		}

		normalizeExecUsageSurfaceCounts(&current)
		current.Count += item.Count
		if current.SurfaceCounts == nil {
			current.SurfaceCounts = map[string]int{}
		}
		for surface, count := range item.SurfaceCounts {
			current.SurfaceCounts[surface] += count
		}
		current.HourlyCounts = mergeExecUsageHours(current.HourlyCounts, item.HourlyCounts, now)
		if current.FirstExecutedAt.IsZero() || (!item.FirstExecutedAt.IsZero() && item.FirstExecutedAt.Before(current.FirstExecutedAt)) {
			current.FirstExecutedAt = item.FirstExecutedAt
		}
		if item.LastExecutedAt.After(current.LastExecutedAt) {
			current.LastExecutedAt = item.LastExecutedAt
			current.LastEnvironmentID = item.LastEnvironmentID
			current.LastSurface = item.LastSurface
		}
		byExecutable[key] = current
	}

	out := make([]model.ExecUsage, 0, len(byExecutable))
	for _, item := range byExecutable {
		normalizeExecUsageSurfaceCounts(&item)
		item.HourlyCounts = normalizeExecUsageHours(item.HourlyCounts, now)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if !out[i].LastExecutedAt.Equal(out[j].LastExecutedAt) {
			return out[i].LastExecutedAt.After(out[j].LastExecutedAt)
		}
		return strings.ToLower(out[i].Executable) < strings.ToLower(out[j].Executable)
	})
	if len(out) > maxExecUsages {
		out = out[:maxExecUsages]
	}
	return out
}

func normalizeExecUsageSurfaceCounts(item *model.ExecUsage) {
	if item == nil {
		return
	}
	if item.Count <= 0 {
		item.Count = 1
	}
	counts := make(map[string]int)
	total := 0
	for surface, count := range item.SurfaceCounts {
		if count <= 0 {
			continue
		}
		name := normalizeExecSurfaceName(surface)
		counts[name] += count
		total += count
	}
	if total > item.Count {
		item.Count = total
	}
	if total < item.Count {
		counts[execUsageUnattributedSurface] += item.Count - total
	}
	item.SurfaceCounts = counts
	item.LastSurface = strings.TrimSpace(item.LastSurface)
}

func recordExecUsageHour(item *model.ExecUsage, hour time.Time, surface string) {
	if item == nil {
		return
	}
	hour = hour.UTC().Truncate(time.Hour)
	surface = normalizeExecSurfaceName(surface)
	for i := range item.HourlyCounts {
		if item.HourlyCounts[i].Hour.UTC().Truncate(time.Hour).Equal(hour) {
			item.HourlyCounts[i].Hour = hour
			item.HourlyCounts[i].Count++
			if item.HourlyCounts[i].SurfaceCounts == nil {
				item.HourlyCounts[i].SurfaceCounts = map[string]int{}
			}
			item.HourlyCounts[i].SurfaceCounts[surface]++
			item.HourlyCounts = normalizeExecUsageHours(item.HourlyCounts, hour)
			return
		}
	}
	item.HourlyCounts = append(item.HourlyCounts, model.ExecUsageHour{
		Hour:          hour,
		Count:         1,
		SurfaceCounts: map[string]int{surface: 1},
	})
	item.HourlyCounts = normalizeExecUsageHours(item.HourlyCounts, hour)
}

func normalizeExecUsageHours(items []model.ExecUsageHour, now time.Time) []model.ExecUsageHour {
	if len(items) == 0 {
		return nil
	}
	now = now.UTC().Truncate(time.Hour)
	cutoff := now.Add(-time.Duration(maxExecUsageHourlyBuckets-1) * time.Hour)
	byHour := make(map[time.Time]model.ExecUsageHour, len(items))
	for _, item := range items {
		if item.Hour.IsZero() {
			continue
		}
		hour := item.Hour.UTC().Truncate(time.Hour)
		if hour.Before(cutoff) || hour.After(now) {
			continue
		}
		counts := make(map[string]int)
		surfaceTotal := 0
		for surface, count := range item.SurfaceCounts {
			if count <= 0 {
				continue
			}
			name := normalizeExecSurfaceName(surface)
			counts[name] += count
			surfaceTotal += count
		}
		if item.Count <= 0 {
			item.Count = surfaceTotal
		}
		if surfaceTotal > item.Count {
			item.Count = surfaceTotal
		}
		if item.Count <= 0 {
			continue
		}
		if surfaceTotal < item.Count {
			counts[execUsageUnattributedSurface] += item.Count - surfaceTotal
		}
		current := byHour[hour]
		current.Hour = hour
		current.Count += item.Count
		if current.SurfaceCounts == nil {
			current.SurfaceCounts = map[string]int{}
		}
		for surface, count := range counts {
			current.SurfaceCounts[surface] += count
		}
		byHour[hour] = current
	}
	out := make([]model.ExecUsageHour, 0, len(byHour))
	for _, item := range byHour {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Hour.After(out[j].Hour)
	})
	if len(out) > maxExecUsageHourlyBuckets {
		out = out[:maxExecUsageHourlyBuckets]
	}
	return out
}

func mergeExecUsageHours(a, b []model.ExecUsageHour, now time.Time) []model.ExecUsageHour {
	combined := make([]model.ExecUsageHour, 0, len(a)+len(b))
	combined = append(combined, cloneExecUsageHours(a)...)
	combined = append(combined, cloneExecUsageHours(b)...)
	return normalizeExecUsageHours(combined, now)
}

func normalizeExecSurfaceName(surface string) string {
	surface = strings.ToLower(strings.TrimSpace(surface))
	if surface == "" {
		return "unknown"
	}
	return surface
}

func cloneExecSurfaceCounts(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]int, len(input))
	for surface, count := range input {
		out[surface] = count
	}
	return out
}

func cloneExecUsageHours(input []model.ExecUsageHour) []model.ExecUsageHour {
	if len(input) == 0 {
		return nil
	}
	out := make([]model.ExecUsageHour, 0, len(input))
	for _, item := range input {
		item.SurfaceCounts = cloneExecSurfaceCounts(item.SurfaceCounts)
		out = append(out, item)
	}
	return out
}
