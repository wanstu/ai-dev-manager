package catalog

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/identity"
	"ai-dev-manager-v2/internal/model"
	skillruntime "ai-dev-manager-v2/internal/skill"
	"ai-dev-manager-v2/internal/store"
)

type Kind string

const KindSkill Kind = "skill"

type SkillSourceRefreshResult struct {
	Source  model.SkillSource    `json:"source"`
	Added   int                  `json:"added"`
	Updated int                  `json:"updated"`
	Removed int                  `json:"removed"`
	Skills  []model.CatalogEntry `json:"skills"`
}

type Service struct {
	store *store.Store
	kind  Kind
	now   func() time.Time
}

func New(s *store.Store, kind Kind) *Service { return &Service{store: s, kind: kind, now: time.Now} }

// Add creates a metadata-only Skill catalog entry for existing internal tests
// and state inspection. Product surfaces configure real Skills by discovery root.
func (s *Service) Add(name string, defaultInclude bool) (model.CatalogEntry, error) {
	if s.kind != KindSkill {
		return model.CatalogEntry{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	return s.add(model.CatalogEntry{Name: name, DefaultIncludeInEnv: defaultInclude})
}

// AddSkill is retained only for existing internal tests/dev-state inspection.
// New product surfaces configure real Skills by explicit discovery root.
func (s *Service) AddSkill(name, instructions string, defaultInclude bool) (model.CatalogEntry, error) {
	if s.kind != KindSkill {
		return model.CatalogEntry{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	instructions = strings.TrimSpace(instructions)
	if instructions == "" {
		return model.CatalogEntry{}, fmt.Errorf("skill instructions are required")
	}
	return s.add(model.CatalogEntry{Name: name, DefaultIncludeInEnv: defaultInclude, Instructions: instructions})
}

// AddSkillRoot is the compatibility path for the original CLI/Gateway skill add
// surface. It registers one explicit source and immediately refreshes it once.
func (s *Service) AddSkillRoot(root string, supportRoots []string, defaultInclude bool) ([]model.CatalogEntry, error) {
	source, err := s.AddSkillSource(root, supportRoots, defaultInclude)
	if err != nil {
		return nil, err
	}
	result, err := s.RefreshSkillSource(source.ID)
	if err != nil {
		_, _ = s.RemoveSkillSource(source.ID)
		return nil, err
	}
	if len(result.Skills) == 0 {
		_, _ = s.RemoveSkillSource(source.ID)
		return nil, fmt.Errorf("skill discovery root %s contains no SKILL.md artifacts", source.Root)
	}
	return result.Skills, nil
}

func (s *Service) AddSkillSource(root string, supportRoots []string, defaultInclude bool) (model.SkillSource, error) {
	return s.AddSkillSourceWithRetention(root, supportRoots, defaultInclude, model.ResourceRetention{})
}

func (s *Service) AddSkillSourceWithRetention(root string, supportRoots []string, defaultInclude bool, retention model.ResourceRetention) (model.SkillSource, error) {
	if s.kind != KindSkill {
		return model.SkillSource{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	configured, err := skillruntime.CanonicalSource(root, supportRoots, defaultInclude)
	if err != nil {
		return model.SkillSource{}, err
	}
	var result model.SkillSource
	err = s.store.Update(func(state *model.State) error {
		for _, existing := range state.SkillSources {
			if samePath(existing.Root, configured.Root) {
				return fmt.Errorf("skill source root %s already exists", configured.Root)
			}
		}
		id, err := identity.New("skill_source")
		if err != nil {
			return err
		}
		now := s.nowUTC()
		result = configured
		result.ID = id
		result.CreatedAt = now
		result.UpdatedAt = now
		result.Retention = normalizeCatalogRetention(retention, now)
		result.LastRefreshStatus = "pending"
		state.SkillSources = append(state.SkillSources, cloneSkillSource(result))
		sortSkillSources(state.SkillSources)
		return nil
	})
	return cloneSkillSource(result), err
}

func (s *Service) ListSkillSources() ([]model.SkillSource, error) {
	if s.kind != KindSkill {
		return nil, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.SkillSource, 0, len(state.SkillSources))
	for _, source := range state.SkillSources {
		items = append(items, cloneSkillSource(source))
	}
	return items, nil
}

func (s *Service) UpdateSkillSource(id, root string, supportRoots []string, defaultInclude bool) (model.SkillSource, error) {
	if s.kind != KindSkill {
		return model.SkillSource{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.SkillSource{}, fmt.Errorf("skill source id is required")
	}
	configured, err := skillruntime.CanonicalSource(root, supportRoots, defaultInclude)
	if err != nil {
		return model.SkillSource{}, err
	}
	var result model.SkillSource
	err = s.store.Update(func(state *model.State) error {
		idx := findSkillSource(state.SkillSources, id)
		if idx < 0 {
			return fmt.Errorf("skill source %q not found", id)
		}
		for i, existing := range state.SkillSources {
			if i != idx && samePath(existing.Root, configured.Root) {
				return fmt.Errorf("skill source root %s already exists", configured.Root)
			}
		}
		current := state.SkillSources[idx]
		pathsChanged := !samePath(current.Root, configured.Root) || !sameStringSlice(current.SupportRoots, configured.SupportRoots)
		now := s.nowUTC()
		current.Root = configured.Root
		current.SupportRoots = append([]string(nil), configured.SupportRoots...)
		current.DefaultIncludeInEnv = configured.DefaultIncludeInEnv
		current.UpdatedAt = now
		if pathsChanged {
			current.LastRefreshStatus = "pending"
			current.LastRefreshError = "source settings changed; refresh required"
		}
		state.SkillSources[idx] = current
		for i := range state.Skills {
			if state.Skills[i].SourceID != id {
				continue
			}
			state.Skills[i].DefaultIncludeInEnv = configured.DefaultIncludeInEnv
			state.Skills[i].SourceRoot = configured.Root
			state.Skills[i].SupportRoots = append([]string(nil), configured.SupportRoots...)
			if state.Skills[i].RelativeArtifactPath != "" {
				state.Skills[i].ArtifactPath = filepath.Clean(filepath.Join(configured.Root, filepath.FromSlash(state.Skills[i].RelativeArtifactPath)))
			}
		}
		sortSkillSources(state.SkillSources)
		sortCatalogEntries(state.Skills)
		result = cloneSkillSource(current)
		return nil
	})
	return cloneSkillSource(result), err
}
func (s *Service) GetSkillSource(id string) (model.SkillSource, error) {
	id = strings.TrimSpace(id)
	sources, err := s.ListSkillSources()
	if err != nil {
		return model.SkillSource{}, err
	}
	for _, source := range sources {
		if source.ID == id {
			return cloneSkillSource(source), nil
		}
	}
	return model.SkillSource{}, fmt.Errorf("skill source %q not found", id)
}

func (s *Service) RemoveSkillSource(id string) (SkillSourceRefreshResult, error) {
	if s.kind != KindSkill {
		return SkillSourceRefreshResult{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	id = strings.TrimSpace(id)
	var result SkillSourceRefreshResult
	err := s.store.Update(func(state *model.State) error {
		idx := findSkillSource(state.SkillSources, id)
		if idx < 0 {
			return fmt.Errorf("skill source %q not found", id)
		}
		result.Source = cloneSkillSource(state.SkillSources[idx])
		state.SkillSources = append(state.SkillSources[:idx], state.SkillSources[idx+1:]...)
		removedIDs := map[string]struct{}{}
		kept := state.Skills[:0]
		for _, entry := range state.Skills {
			if entry.SourceID == id {
				result.Removed++
				removedIDs[entry.ID] = struct{}{}
				continue
			}
			kept = append(kept, entry)
		}
		state.Skills = kept
		for envIndex := range state.Environments {
			state.Environments[envIndex].EnabledSkillIDs = removeCatalogSelections(state.Environments[envIndex].EnabledSkillIDs, removedIDs)
		}
		for workspaceIndex := range state.Workspaces {
			state.Workspaces[workspaceIndex].EnabledSkillIDs = removeCatalogSelections(state.Workspaces[workspaceIndex].EnabledSkillIDs, removedIDs)
		}
		return nil
	})
	return result, err
}

func (s *Service) RefreshSkillSource(id string) (SkillSourceRefreshResult, error) {
	if s.kind != KindSkill {
		return SkillSourceRefreshResult{}, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	source, err := s.GetSkillSource(id)
	if err != nil {
		return SkillSourceRefreshResult{}, err
	}
	discovered, err := skillruntime.DiscoverSource(source)
	if err != nil {
		_ = s.markSkillSourceRefreshFailure(source.ID, err)
		return SkillSourceRefreshResult{}, err
	}
	for i := range discovered {
		discovered[i].Retention = cloneRetention(source.Retention)
	}

	result := SkillSourceRefreshResult{Skills: make([]model.CatalogEntry, len(discovered))}
	copy(result.Skills, discovered)
	now := s.nowUTC()
	err = s.store.Update(func(state *model.State) error {
		idx := findSkillSource(state.SkillSources, source.ID)
		if idx < 0 {
			return fmt.Errorf("skill source %q not found", source.ID)
		}
		currentSource := state.SkillSources[idx]
		oldByID := map[string]model.CatalogEntry{}
		kept := state.Skills[:0]
		for _, entry := range state.Skills {
			if entry.SourceID == source.ID {
				oldByID[entry.ID] = entry
				continue
			}
			kept = append(kept, entry)
		}
		seenNew := map[string]struct{}{}
		for _, incoming := range discovered {
			if _, exists := seenNew[incoming.ID]; exists {
				return fmt.Errorf("duplicate skill artifact identity %q in source %s", incoming.ID, source.ID)
			}
			seenNew[incoming.ID] = struct{}{}
			if old, exists := oldByID[incoming.ID]; !exists {
				result.Added++
			} else if !reflect.DeepEqual(old, incoming) {
				result.Updated++
			}
			kept = append(kept, cloneCatalogEntry(incoming))
		}
		for id := range oldByID {
			if _, exists := seenNew[id]; !exists {
				result.Removed++
			}
		}
		currentSource.UpdatedAt = now
		currentSource.LastRefreshAt = &now
		currentSource.LastRefreshStatus = "ok"
		currentSource.LastRefreshError = ""
		currentSource.DefaultIncludeInEnv = source.DefaultIncludeInEnv
		currentSource.Root = source.Root
		currentSource.SupportRoots = append([]string(nil), source.SupportRoots...)
		state.SkillSources[idx] = currentSource
		state.Skills = kept
		sortSkillSources(state.SkillSources)
		sortCatalogEntries(state.Skills)
		result.Source = cloneSkillSource(currentSource)
		return nil
	})
	if err != nil {
		return SkillSourceRefreshResult{}, err
	}
	return result, nil
}

func (s *Service) markSkillSourceRefreshFailure(id string, refreshErr error) error {
	return s.store.Update(func(state *model.State) error {
		idx := findSkillSource(state.SkillSources, id)
		if idx < 0 {
			return fmt.Errorf("skill source %q not found", id)
		}
		now := s.nowUTC()
		state.SkillSources[idx].UpdatedAt = now
		state.SkillSources[idx].LastRefreshAt = &now
		state.SkillSources[idx].LastRefreshStatus = "error"
		state.SkillSources[idx].LastRefreshError = refreshErr.Error()
		return nil
	})
}

func (s *Service) add(entry model.CatalogEntry) (model.CatalogEntry, error) {
	entry.Name = strings.TrimSpace(entry.Name)
	if entry.Name == "" {
		return model.CatalogEntry{}, fmt.Errorf("skill name is required")
	}
	var result model.CatalogEntry
	err := s.store.Update(func(state *model.State) error {
		for _, existing := range state.Skills {
			if existing.SourceID == "" && strings.EqualFold(existing.Name, entry.Name) {
				return fmt.Errorf("skill %q already exists", entry.Name)
			}
		}
		id, err := identity.New("skill")
		if err != nil {
			return err
		}
		result = cloneCatalogEntry(entry)
		result.ID = id
		result.Retention = normalizeCatalogRetention(result.Retention, s.nowUTC())
		state.Skills = append(state.Skills, cloneCatalogEntry(result))
		sortCatalogEntries(state.Skills)
		return nil
	})
	return cloneCatalogEntry(result), err
}

func (s *Service) List() ([]model.CatalogEntry, error) {
	if s.kind != KindSkill {
		return nil, fmt.Errorf("catalog kind %q is not Skill", s.kind)
	}
	state, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	items := make([]model.CatalogEntry, 0, len(state.Skills))
	for _, entry := range state.Skills {
		items = append(items, cloneCatalogEntry(entry))
	}
	return items, nil
}

func (s *Service) Get(id string) (model.CatalogEntry, error) {
	items, err := s.List()
	if err != nil {
		return model.CatalogEntry{}, err
	}
	for _, entry := range items {
		if entry.ID == id {
			return cloneCatalogEntry(entry), nil
		}
	}
	return model.CatalogEntry{}, fmt.Errorf("skill %q not found", id)
}

func (s *Service) SetDefault(id string, value bool) (model.CatalogEntry, error) {
	var result model.CatalogEntry
	err := s.store.Update(func(state *model.State) error {
		for i := range state.Skills {
			if state.Skills[i].ID == id {
				state.Skills[i].DefaultIncludeInEnv = value
				result = cloneCatalogEntry(state.Skills[i])
				return nil
			}
		}
		return fmt.Errorf("skill %q not found", id)
	})
	return cloneCatalogEntry(result), err
}

func (s *Service) Remove(id string) error {
	return s.store.Update(func(state *model.State) error {
		for i := range state.Skills {
			if state.Skills[i].ID == id {
				state.Skills = append(state.Skills[:i], state.Skills[i+1:]...)
				removed := map[string]struct{}{id: {}}
				for envIndex := range state.Environments {
					state.Environments[envIndex].EnabledSkillIDs = removeCatalogSelections(state.Environments[envIndex].EnabledSkillIDs, removed)
				}
				for workspaceIndex := range state.Workspaces {
					state.Workspaces[workspaceIndex].EnabledSkillIDs = removeCatalogSelections(state.Workspaces[workspaceIndex].EnabledSkillIDs, removed)
				}
				return nil
			}
		}
		return fmt.Errorf("skill %q not found", id)
	})
}

func (s *Service) Exists(id string) (bool, error) {
	items, err := s.List()
	if err != nil {
		return false, err
	}
	for _, entry := range items {
		if entry.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func findSkillSource(sources []model.SkillSource, id string) int {
	for i := range sources {
		if sources[i].ID == id {
			return i
		}
	}
	return -1
}

func cloneSkillSource(source model.SkillSource) model.SkillSource {
	source.SupportRoots = append([]string(nil), source.SupportRoots...)
	if source.LastRefreshAt != nil {
		last := *source.LastRefreshAt
		source.LastRefreshAt = &last
	}
	source.Retention = cloneRetention(source.Retention)
	return source
}

func cloneCatalogEntry(entry model.CatalogEntry) model.CatalogEntry {
	entry.SupportRoots = append([]string(nil), entry.SupportRoots...)
	entry.Retention = cloneRetention(entry.Retention)
	return entry
}

func sortSkillSources(sources []model.SkillSource) {
	sort.Slice(sources, func(i, j int) bool {
		return strings.ToLower(sources[i].Root) < strings.ToLower(sources[j].Root)
	})
}

func sortCatalogEntries(entries []model.CatalogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := strings.ToLower(entries[i].SourceID + "/" + entries[i].RelativeArtifactPath + "/" + entries[i].Name + "/" + entries[i].ID)
		right := strings.ToLower(entries[j].SourceID + "/" + entries[j].RelativeArtifactPath + "/" + entries[j].Name + "/" + entries[j].ID)
		return left < right
	})
}

func samePath(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !samePath(a[i], b[i]) {
			return false
		}
	}
	return true
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
