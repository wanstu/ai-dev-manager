package desktop

import (
	"errors"

	"ai-dev-manager-v2/internal/app"
)

type autoIndexManagementBackend interface {
	ProjectIndexAutoStatus(string) (app.IndexAutoStatus, error)
	ProjectIndexAutoSet(string, bool) (app.IndexAutoStatus, error)
}

func (a *Adapter) ProjectIndexAutoStatus(id string) (app.IndexAutoStatus, error) {
	if err := a.ready(); err != nil {
		return app.IndexAutoStatus{}, err
	}
	b, ok := a.management.(autoIndexManagementBackend)
	if !ok {
		return app.IndexAutoStatus{}, errors.New("connected ADM Gateway does not support automatic project indexing")
	}
	return b.ProjectIndexAutoStatus(id)
}
func (a *Adapter) SetProjectIndexAuto(id string, enabled bool) (app.IndexAutoStatus, error) {
	if err := a.ready(); err != nil {
		return app.IndexAutoStatus{}, err
	}
	b, ok := a.management.(autoIndexManagementBackend)
	if !ok {
		return app.IndexAutoStatus{}, errors.New("connected ADM Gateway does not support automatic project indexing")
	}
	return b.ProjectIndexAutoSet(id, enabled)
}
