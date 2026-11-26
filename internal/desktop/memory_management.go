package desktop

import (
	"ai-dev-manager-v2/internal/memory"
	"fmt"
)

type memoryManagementBackend interface {
	MemorySimilar(string, float64, int) (memory.SimilarResult, error)
	MemoryProjectScope(string) (memory.ProjectScope, error)
	MemoryHandoff(string, string, []memory.HandoffItem, bool) (memory.HandoffResult, error)
	MemoryHistory(string, string, *int64) (memory.HistoryResult, error)
	MemoryRestore(string, string, int64, int64) (memory.Entry, error)
	MemoryTrash(string) ([]memory.TrashEntry, error)
	MemoryPurge(string, string, int64) error
	MemoryDuplicates(string) (memory.DuplicateResult, error)
	MemoryOrganize(string, []memory.Change, bool) (memory.OrganizeResult, error)
}

func (a *Adapter) memoryBackend() (memoryManagementBackend, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	b, ok := a.management.(memoryManagementBackend)
	if !ok {
		return nil, fmt.Errorf("memory management backend unavailable")
	}
	return b, nil
}
func (a *Adapter) GetMemoryHistory(id, key string, revision *int64) (memory.HistoryResult, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.HistoryResult{}, err
	}
	return b.MemoryHistory(id, key, revision)
}
func (a *Adapter) RestoreMemory(id, key string, revision, expected int64) (memory.Entry, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.Entry{}, err
	}
	return b.MemoryRestore(id, key, revision, expected)
}
func (a *Adapter) ListMemoryTrash(id string) ([]memory.TrashEntry, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return nil, err
	}
	return b.MemoryTrash(id)
}
func (a *Adapter) PurgeMemory(id, key string, expected int64) error {
	b, err := a.memoryBackend()
	if err != nil {
		return err
	}
	return b.MemoryPurge(id, key, expected)
}
func (a *Adapter) FindMemoryDuplicates(id string) (memory.DuplicateResult, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.DuplicateResult{}, err
	}
	return b.MemoryDuplicates(id)
}
func (a *Adapter) OrganizeMemory(id string, changes []memory.Change, dryRun bool) (memory.OrganizeResult, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.OrganizeResult{}, err
	}
	return b.MemoryOrganize(id, changes, dryRun)
}

func (a *Adapter) FindSimilarMemory(id string, threshold float64, limit int) (memory.SimilarResult, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.SimilarResult{}, err
	}
	return b.MemorySimilar(id, threshold, limit)
}
func (a *Adapter) GetMemoryProjectScope(id string) (memory.ProjectScope, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.ProjectScope{}, err
	}
	return b.MemoryProjectScope(id)
}
func (a *Adapter) HandoffMemory(id, target string, items []memory.HandoffItem, dryRun bool) (memory.HandoffResult, error) {
	b, err := a.memoryBackend()
	if err != nil {
		return memory.HandoffResult{}, err
	}
	return b.MemoryHandoff(id, target, items, dryRun)
}
