package desktop

import (
	"errors"

	"ai-dev-manager-v2/internal/management"
)

type Adapter struct {
	profilesPath    string
	preferencesPath string
	management      managementBackend
	runtime         runtimeBackend
}

// NewAdapter retains an explicit local backend for tests and offline/recovery callers.
// Production Desktop uses NewClientAdapter and connects through Admin MCP.
func NewAdapter(service *management.Service) *Adapter {
	return &Adapter{management: service}
}

func NewClientAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ready() error {
	if a == nil {
		return errors.New("desktop management adapter is not initialized")
	}
	if a.management == nil {
		return errors.New("ADM Admin MCP is not connected")
	}
	return nil
}

func (a *Adapter) readyRuntime() error {
	if a == nil {
		return errors.New("desktop runtime adapter is not initialized")
	}
	if a.runtime == nil {
		return errors.New("ADM Admin MCP runtime is not connected")
	}
	return nil
}
