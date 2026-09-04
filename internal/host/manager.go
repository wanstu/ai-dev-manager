package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"ai-dev-manager/internal/adapter/mcpserver"
	"ai-dev-manager/internal/adapter/runtimeadapter"
)

type InstanceState string

const (
	InstanceRunning InstanceState = "running"
	InstanceError   InstanceState = "error"
)

type Instance struct {
	ID          string        `json:"id"`
	WorkspaceID string        `json:"workspace_id"`
	RuntimeID   string        `json:"runtime_id"`
	Transport   string        `json:"transport"`
	Address     string        `json:"address"`
	Endpoint    string        `json:"endpoint"`
	State       InstanceState `json:"state"`
}

type managedInstance struct {
	info     Instance
	server   *http.Server
	listener net.Listener
	adapter  runtimeadapter.Runtime
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*managedInstance
}

func NewManager() *Manager {
	return &Manager{instances: make(map[string]*managedInstance)}
}

func (m *Manager) StartHTTP(instanceID string, adapter runtimeadapter.Runtime, addr string) (Instance, error) {
	return m.startHTTP(instanceID, adapter, addr, false)
}

// StartHTTPExposed is an explicit opt-in path for callers that intentionally
// need an MCP listener reachable outside host loopback (for example from a
// local Docker container). StartHTTP remains loopback-only by default.
func (m *Manager) StartHTTPExposed(instanceID string, adapter runtimeadapter.Runtime, addr string) (Instance, error) {
	return m.startHTTP(instanceID, adapter, addr, true)
}

func (m *Manager) startHTTP(instanceID string, adapter runtimeadapter.Runtime, addr string, allowNonLoopback bool) (Instance, error) {
	if strings.TrimSpace(instanceID) == "" || adapter == nil {
		return Instance{}, fmt.Errorf("invalid MCP instance")
	}
	if strings.TrimSpace(addr) == "" {
		addr = "127.0.0.1:0"
	}
	if allowNonLoopback {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return Instance{}, fmt.Errorf("invalid MCP listen address %q", addr)
		}
	} else if err := validateLoopbackAddress(addr); err != nil {
		return Instance{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.instances[instanceID]; exists {
		return Instance{}, fmt.Errorf("MCP instance %q already exists", instanceID)
	}

	network := "tcp"
	if host, _, splitErr := net.SplitHostPort(addr); splitErr == nil && host == "0.0.0.0" {
		network = "tcp4"
	}
	listener, err := net.Listen(network, addr)
	if err != nil {
		return Instance{}, fmt.Errorf("listen MCP instance %q: %w", instanceID, err)
	}
	mux := http.NewServeMux()
	handler := mcpserver.NewHTTPHandler(adapter)
	if allowNonLoopback {
		bindHost, _, _ := net.SplitHostPort(addr)
		options := mcpserver.ExposedHTTPOptions{AllowedHosts: []string{bindHost}}
		if bindHost == "0.0.0.0" || bindHost == "::" || bindHost == "" {
			options.AllowedHosts = []string{"host.docker.internal"}
			options.AllowIPLiteral = true
		}
		handler = mcpserver.NewExposedHTTPHandler(adapter, options)
	}
	mux.Handle("/mcp", handler)
	httpServer := &http.Server{Handler: mux}

	actualAddress := listener.Addr().String()
	info := Instance{
		ID:          instanceID,
		WorkspaceID: adapter.WorkspaceID(),
		RuntimeID:   adapter.ID(),
		Transport:   "streamable-http",
		Address:     actualAddress,
		Endpoint:    "http://" + actualAddress + "/mcp",
		State:       InstanceRunning,
	}
	managed := &managedInstance{info: info, server: httpServer, listener: listener, adapter: adapter}
	m.instances[instanceID] = managed

	go func() {
		err := httpServer.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if current, ok := m.instances[instanceID]; ok && current == managed {
			current.info.State = InstanceError
		}
	}()

	return info, nil
}

func (m *Manager) Get(instanceID string) (Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	managed, ok := m.instances[instanceID]
	if !ok {
		return Instance{}, false
	}
	return managed.info, true
}

func (m *Manager) List() []Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.instances))
	for id := range m.instances {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Instance, 0, len(ids))
	for _, id := range ids {
		result = append(result, m.instances[id].info)
	}
	return result
}

func (m *Manager) Stop(ctx context.Context, instanceID string) error {
	m.mu.Lock()
	managed, ok := m.instances[instanceID]
	if ok {
		delete(m.instances, instanceID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if err := managed.server.Shutdown(ctx); err != nil {
		_ = managed.listener.Close()
		return fmt.Errorf("stop MCP instance %q: %w", instanceID, err)
	}
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	instances := m.List()
	var errs []error
	for _, instance := range instances {
		if err := m.Stop(ctx, instance.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validateLoopbackAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid MCP listen address %q", addr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("MCP listen address must be loopback in v0.1: %q", addr)
	}
	return nil
}
