package desktop

import (
	"errors"

	"ai-dev-manager-v2/internal/projectanalysis"
)

type indexManagementBackend interface {
	ProjectIndexStatus(string, int) (projectanalysis.IndexStatusResult, error)
	ProjectAnalyze(string, string, int, int) (projectanalysis.Result, error)
}

// ProjectIndexStatus reads a pre-existing index and never takes a writer lease.
func (a *Adapter) ProjectIndexStatus(environmentID string, maxChanges int) (projectanalysis.IndexStatusResult, error) {
	if err := a.ready(); err != nil {
		return projectanalysis.IndexStatusResult{}, err
	}
	backend, ok := a.management.(indexManagementBackend)
	if !ok {
		return projectanalysis.IndexStatusResult{}, errors.New("ADM Gateway does not support project index management; update the connected Gateway")
	}
	return backend.ProjectIndexStatus(environmentID, maxChanges)
}

// AnalyzeProject is an explicit, user-triggered, read-only source scan.
// The ADM Gateway owns an independent index lock, never a source writer lease.
func (a *Adapter) AnalyzeProject(environmentID string, maxFiles, maxSymbols int) (projectanalysis.Result, error) {
	if err := a.ready(); err != nil {
		return projectanalysis.Result{}, err
	}
	backend, ok := a.management.(indexManagementBackend)
	if !ok {
		return projectanalysis.Result{}, errors.New("ADM Gateway does not support project index management; update the connected Gateway")
	}
	return backend.ProjectAnalyze(environmentID, "", maxFiles, maxSymbols)
}
