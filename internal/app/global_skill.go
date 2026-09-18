package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillruntime "ai-dev-manager-v2/internal/skill"
)

func (s *Service) GlobalSkillAvailabilities() (SkillAvailabilityList, error) {
	return s.SkillAvailabilities()
}

func (s *Service) InspectGlobalSkill(skillID string) (SkillAvailability, error) {
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		return SkillAvailability{}, err
	}
	return s.skillCatalogAvailabilityForEntry(entry), nil
}

func (s *Service) ReadGlobalSkill(skillID, path string, maxBytes int) (skillruntime.Content, error) {
	availability, err := s.InspectGlobalSkill(skillID)
	if err != nil {
		return skillruntime.Content{}, err
	}
	if availability.State != SkillAvailabilityAvailable {
		return skillruntime.Content{}, &SkillError{SkillID: skillID, ErrorKind: availability.State, Message: availability.Reason}
	}
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		return skillruntime.Content{}, err
	}
	content, err := skillruntime.Read(entry, path, maxBytes)
	if err != nil {
		return skillruntime.Content{}, &SkillError{SkillID: skillID, ErrorKind: classifySkillReadError(err), Message: err.Error()}
	}
	return content, nil
}

func (s *Service) GlobalSkillFiles(skillID, rootKind string, maxEntries int) (SkillFileInventory, error) {
	availability, err := s.InspectGlobalSkill(skillID)
	if err != nil {
		return SkillFileInventory{}, err
	}
	if availability.State != SkillAvailabilityAvailable {
		return SkillFileInventory{}, &SkillError{SkillID: skillID, ErrorKind: availability.State, Message: availability.Reason}
	}
	entry, err := s.Skills.Get(skillID)
	if err != nil {
		return SkillFileInventory{}, err
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
	inventory := SkillFileInventory{SkillID: skillID, RootKind: kind, Root: root, MaxEntries: maxEntries}
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
