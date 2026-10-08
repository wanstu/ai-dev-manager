package desktop

import (
	"ai-dev-manager-v2/internal/app"
	"errors"
)

type indexJobManagementBackend interface {
	StartProjectIndexJob(string) (app.ProjectIndexJob, error)
	ProjectIndexJobStatus(string) (app.ProjectIndexJob, error)
}

func (a *Adapter) StartProjectIndexJob(environmentID string) (app.ProjectIndexJob, error) {
	if err := a.ready(); err != nil {
		return app.ProjectIndexJob{}, err
	}
	b, ok := a.management.(indexJobManagementBackend)
	if !ok {
		return app.ProjectIndexJob{}, errors.New("connected ADM Gateway does not support background index jobs")
	}
	return b.StartProjectIndexJob(environmentID)
}
func (a *Adapter) ProjectIndexJobStatus(environmentID string) (app.ProjectIndexJob, error) {
	if err := a.ready(); err != nil {
		return app.ProjectIndexJob{}, err
	}
	b, ok := a.management.(indexJobManagementBackend)
	if !ok {
		return app.ProjectIndexJob{}, errors.New("connected ADM Gateway does not support background index jobs")
	}
	return b.ProjectIndexJobStatus(environmentID)
}
