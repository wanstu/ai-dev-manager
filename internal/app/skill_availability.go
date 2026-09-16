package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
	skillruntime "ai-dev-manager-v2/internal/skill"
)

const (
	SkillAvailabilityAvailable          = "available"
	SkillAvailabilityDisabled           = "disabled"
	SkillAvailabilityUnresolved         = "unresolved"
	SkillAvailabilitySourceMissing      = "source_missing"
	SkillAvailabilityArtifactMissing    = "artifact_missing"
	SkillAvailabilityArtifactUnreadable = "artifact_unreadable"
	SkillAvailabilitySupportRootMissing = "support_root_missing"
	SkillAvailabilityUnconfigured       = "unconfigured"
)

const defaultSkillInventoryLimit = 256

type SkillAvailability struct {
	EnvironmentID        string   `json:"environment_id"`
	SkillID              string   `json:"skill_id"`
	SourceID             string   `json:"source_id,omitempty"`
	Name                 string   `json:"name,omitempty"`
	Enabled              bool     `json:"enabled"`
	State                string   `json:"state"`
	Reason               string   `json:"reason,omitempty"`
	SourceRoot           string   `json:"source_root,omitempty"`
	ArtifactPath         string   `json:"artifact_path,omitempty"`
	RelativeArtifactPath string   `json:"relative_artifact_path,omitempty"`
	SupportRoots         []string `json:"support_roots,omitempty"`
	MissingSupportRoots  []string `json:"missing_support_roots,omitempty"`
}

type SkillAvailabilityList struct {
	EnvironmentID string              `json:"environment_id,omitempty"`
	Scope         string              `json:"scope,omitempty"`
	Skills        []SkillAvailability `json:"skills"`
}

type SkillFileInventoryItem struct {
	Path      string `json:"path"`
	RootKind  string `json:"root_kind"`
	Root      string `json:"root"`
	SizeBytes int64  `json:"size_bytes"`
	Readable  bool   `json:"readable"`
	ErrorKind string `json:"error_kind,omitempty"`
	Error     string `json:"error,omitempty"`
}

type SkillFileInventory struct {
	EnvironmentID string                   `json:"environment_id"`
	SkillID       string                   `json:"skill_id"`
	RootKind      string                   `json:"root_kind"`
	Root          string                   `json:"root"`
	MaxEntries    int                      `json:"max_entries"`
	Truncated     bool                     `json:"truncated"`
	Files         []SkillFileInventoryItem `json:"files"`
}

type SkillError struct {
	SkillID   string `json:"skill_id"`
	ErrorKind string `json:"error_kind"`
	Message   string `json:"message"`
}

func (e *SkillError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("skill_id=%s error_kind=%s message=%s", e.SkillID, e.ErrorKind, e.Message)
}

func (s *Service) SkillAvailabilities() (SkillAvailabilityList, error) {
	entries, err := s.Skills.List()
	if err != nil {
		return SkillAvailabilityList{}, err
	}
	items := make([]SkillAvailability, 0, len(entries))
	for _, entry := range entries {
		items = append(items, s.skillCatalogAvailabilityForEntry(entry))
	}
	sortSkillAvailabilities(items)
	return SkillAvailabilityList{Scope: "catalog", Skills: items}, nil
}
func (s *Service) EnvironmentSkillAvailabilities(environmentID string) (SkillAvailabilityList, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return SkillAvailabilityList{}, err
	}
	entries, err := s.Skills.List()
	if err != nil {
		return SkillAvailabilityList{}, err
	}
	byID := make(map[string]model.CatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	seen := map[string]struct{}{}
	items := make([]SkillAvailability, 0, len(env.EnabledSkillIDs)+len(entries))
	for _, id := range env.EnabledSkillIDs {
		seen[id] = struct{}{}
		entry, ok := byID[id]
		if !ok {
			items = append(items, SkillAvailability{EnvironmentID: environmentID, SkillID: id, Enabled: true, State: SkillAvailabilityUnresolved, Reason: "environment selects a Skill ID that is not present in the current catalog"})
			continue
		}
		items = append(items, s.skillAvailabilityForEntry(environmentID, entry, true))
	}
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		items = append(items, s.skillAvailabilityForEntry(environmentID, entry, false))
	}
	sortSkillAvailabilities(items)
	return SkillAvailabilityList{EnvironmentID: environmentID, Skills: items}, nil
}

func (s *Service) InspectEnvironmentSkill(environmentID, skillID string) (SkillAvailability, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return SkillAvailability{}, err
	}
	enabled := containsString(env.EnabledSkillIDs, skillID)
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		if enabled {
			return SkillAvailability{EnvironmentID: environmentID, SkillID: skillID, Enabled: true, State: SkillAvailabilityUnresolved, Reason: "environment selects a Skill ID that is not present in the current catalog"}, nil
		}
		return SkillAvailability{}, err
	}
	return s.skillAvailabilityForEntry(environmentID, entry, enabled), nil
}

func (s *Service) skillCatalogAvailabilityForEntry(entry model.CatalogEntry) SkillAvailability {
	return s.skillStructuralAvailability("", entry, true)
}

func (s *Service) skillAvailabilityForEntry(environmentID string, entry model.CatalogEntry, enabled bool) SkillAvailability {
	status := s.skillStructuralAvailability(environmentID, entry, enabled)
	if status.State != SkillAvailabilityAvailable {
		return status
	}
	if !enabled {
		status.State = SkillAvailabilityDisabled
		status.Reason = "skill is not enabled for this Environment"
	}
	return status
}

func (s *Service) skillStructuralAvailability(environmentID string, entry model.CatalogEntry, enabled bool) SkillAvailability {
	status := SkillAvailability{
		EnvironmentID:        environmentID,
		SkillID:              entry.ID,
		SourceID:             entry.SourceID,
		Name:                 entry.Name,
		Enabled:              enabled,
		State:                SkillAvailabilityAvailable,
		SourceRoot:           entry.SourceRoot,
		ArtifactPath:         entry.ArtifactPath,
		RelativeArtifactPath: entry.RelativeArtifactPath,
		SupportRoots:         append([]string(nil), entry.SupportRoots...),
	}
	if !skillruntime.Configured(entry) {
		status.State = SkillAvailabilityUnconfigured
		status.Reason = "skill catalog entry is not backed by a configured artifact"
		return status
	}
	if ok, reason := directoryUsable(entry.SourceRoot); !ok {
		status.State = SkillAvailabilitySourceMissing
		status.Reason = reason
		return status
	}
	artifactState, artifactReason := artifactAvailability(entry.ArtifactPath)
	if artifactState != SkillAvailabilityAvailable {
		status.State = artifactState
		status.Reason = artifactReason
		return status
	}
	missingSupport := make([]string, 0)
	for _, root := range entry.SupportRoots {
		if ok, _ := directoryUsable(root); !ok {
			missingSupport = append(missingSupport, root)
		}
	}
	if len(missingSupport) != 0 {
		status.State = SkillAvailabilitySupportRootMissing
		status.Reason = "one or more configured support roots are unavailable"
		status.MissingSupportRoots = missingSupport
		return status
	}
	return status
}

func sortSkillAvailabilities(items []SkillAvailability) {
	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].State + "/" + items[i].SourceID + "/" + items[i].RelativeArtifactPath + "/" + items[i].SkillID)
		right := strings.ToLower(items[j].State + "/" + items[j].SourceID + "/" + items[j].RelativeArtifactPath + "/" + items[j].SkillID)
		return left < right
	})
}
func (s *Service) EnvironmentSkillFiles(environmentID, skillID, rootKind string, maxEntries int) (SkillFileInventory, error) {
	availability, err := s.InspectEnvironmentSkill(environmentID, skillID)
	if err != nil {
		return SkillFileInventory{}, err
	}
	if !availability.Enabled {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: "not_enabled", Message: "skill is not enabled for this Environment"}
	}
	if availability.State != SkillAvailabilityAvailable {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: availability.State, Message: availability.Reason}
	}
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: "unresolved", Message: err.Error()}
	}
	kind := strings.TrimSpace(rootKind)
	if kind == "" {
		kind = "artifact"
	}
	root, err := skillInventoryRoot(entry, kind)
	if err != nil {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: "invalid_scope", Message: err.Error()}
	}
	if maxEntries <= 0 || maxEntries > defaultSkillInventoryLimit {
		maxEntries = defaultSkillInventoryLimit
	}
	inventory := SkillFileInventory{EnvironmentID: environmentID, SkillID: skillID, RootKind: kind, Root: root, MaxEntries: maxEntries}
	err = filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if dirEntry.Type()&os.ModeSymlink != 0 {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if dirEntry.IsDir() {
			return nil
		}
		if len(inventory.Files) >= maxEntries {
			inventory.Truncated = true
			return filepath.SkipAll
		}
		info, err := dirEntry.Info()
		item := SkillFileInventoryItem{Path: normalizeInventoryPath(root, path), RootKind: kind, Root: root}
		if err != nil {
			item.Readable = false
			item.ErrorKind = "stat_failed"
			item.Error = err.Error()
		} else {
			item.SizeBytes = info.Size()
			item.Readable = info.Mode().IsRegular()
			if !item.Readable {
				item.ErrorKind = "not_regular"
				item.Error = "skill file is not a regular file"
			}
		}
		inventory.Files = append(inventory.Files, item)
		return nil
	})
	if err != nil {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: classifySkillReadError(err), Message: err.Error()}
	}
	sort.Slice(inventory.Files, func(i, j int) bool { return inventory.Files[i].Path < inventory.Files[j].Path })
	return inventory, nil
}

func (s *Service) ReadEnvironmentSkill(environmentID, skillID, path string, maxBytes int) (skillruntime.Content, error) {
	env, err := s.Environments.Get(environmentID)
	if err != nil {
		return skillruntime.Content{}, err
	}
	if !containsString(env.EnabledSkillIDs, skillID) {
		return skillruntime.Content{}, &SkillError{SkillID: skillID, ErrorKind: "not_enabled", Message: fmt.Sprintf("skill %q is not enabled for environment %q", skillID, environmentID)}
	}
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		return skillruntime.Content{}, &SkillError{SkillID: skillID, ErrorKind: "unresolved", Message: err.Error()}
	}
	content, err := skillruntime.Read(entry, path, maxBytes)
	if err != nil {
		return skillruntime.Content{}, &SkillError{SkillID: skillID, ErrorKind: classifySkillReadError(err), Message: err.Error()}
	}
	return content, nil
}

func artifactAvailability(path string) (string, string) {
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SkillAvailabilityArtifactMissing, "skill artifact is missing"
		}
		return SkillAvailabilityArtifactUnreadable, err.Error()
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SkillAvailabilityArtifactMissing, "skill artifact is missing"
		}
		return SkillAvailabilityArtifactUnreadable, err.Error()
	}
	if !info.Mode().IsRegular() {
		return SkillAvailabilityArtifactUnreadable, "skill artifact is not a regular file"
	}
	if info.Size() > int64(defaultSkillReadMaxBytes()) {
		return SkillAvailabilityArtifactUnreadable, "skill artifact exceeds readable size limit"
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return SkillAvailabilityArtifactUnreadable, err.Error()
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return SkillAvailabilityArtifactUnreadable, "skill artifact is binary or contains NUL bytes"
	}
	return SkillAvailabilityAvailable, ""
}

func directoryUsable(path string) (bool, string) {
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "directory is missing"
		}
		return false, err.Error()
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, "path is not a real directory"
	}
	if _, err := filepath.EvalSymlinks(filepath.Clean(path)); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func skillInventoryRoot(entry model.CatalogEntry, kind string) (string, error) {
	switch {
	case kind == "artifact":
		return filepath.Dir(filepath.Clean(entry.ArtifactPath)), nil
	case strings.HasPrefix(kind, "support"):
		index := 0
		if kind != "support" && strings.HasPrefix(kind, "support:") {
			if _, err := fmt.Sscanf(strings.TrimPrefix(kind, "support:"), "%d", &index); err != nil || index < 0 {
				return "", fmt.Errorf("support root index is invalid")
			}
		}
		if index >= len(entry.SupportRoots) {
			return "", fmt.Errorf("support root index %d is not configured", index)
		}
		return filepath.Clean(entry.SupportRoots[index]), nil
	default:
		return "", fmt.Errorf("skill file root must be artifact, support, or support:<index>")
	}
}

func classifySkillReadError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not enabled"):
		return "not_enabled"
	case strings.Contains(message, "outside configured artifact/support roots"):
		return "outside_allowed_roots"
	case strings.Contains(message, "support root"):
		return "support_root_missing"
	case strings.Contains(message, "artifact"):
		if strings.Contains(message, "not found") || strings.Contains(message, "cannot find") || strings.Contains(message, "no such file") {
			return "artifact_missing"
		}
		return "artifact_unreadable"
	case strings.Contains(message, "binary") || strings.Contains(message, "nul"):
		return "binary_file"
	case strings.Contains(message, "exceeds max_bytes"):
		return "file_oversize"
	case strings.Contains(message, "not a regular file"):
		return "not_regular"
	case strings.Contains(message, "not found") || strings.Contains(message, "cannot find") || strings.Contains(message, "no such file"):
		return "file_missing"
	default:
		return "read_failed"
	}
}

func normalizeInventoryPath(root, path string) string {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == "." {
		return filepath.ToSlash(filepath.Base(path))
	}
	return filepath.ToSlash(rel)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func defaultSkillReadMaxBytes() int { return 1 << 20 }
