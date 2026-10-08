package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

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

// AnalyzeProject is an explicit, user-triggered operation. It takes a short
// owner-scoped lease and does not seize or release another session's writer.
func (a *Adapter) AnalyzeProject(environmentID string, maxFiles, maxSymbols int) (projectanalysis.Result, error) {
	if err := a.ready(); err != nil {
		return projectanalysis.Result{}, err
	}
	backend, ok := a.management.(indexManagementBackend)
	if !ok {
		return projectanalysis.Result{}, errors.New("ADM Gateway does not support project index management; update the connected Gateway")
	}
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return projectanalysis.Result{}, fmt.Errorf("create index writer owner: %w", err)
	}
	owner := "desktop-index-" + hex.EncodeToString(entropy[:])
	if _, err := a.runtime.WriterAcquire(environmentID, owner); err != nil {
		return projectanalysis.Result{}, fmt.Errorf("project index requires a free writer lease: %w", err)
	}
	defer a.runtime.WriterRelease(environmentID, owner, false)
	return backend.ProjectAnalyze(environmentID, owner, maxFiles, maxSymbols)
}
