package hostenv

import (
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status struct {
	Source            string     `json:"source"`
	LoadedAt          time.Time  `json:"loaded_at"`
	LastRefreshAt     *time.Time `json:"last_refresh_at,omitempty"`
	LastRefreshStatus string     `json:"last_refresh_status"`
	LastRefreshError  string     `json:"last_refresh_error,omitempty"`
	VariableCount     int        `json:"variable_count"`
}

type Reader func() (map[string]string, string, error)

type Manager struct {
	mu                    sync.RWMutex
	values                map[string]string
	initialValues         map[string]string
	managedKeys           map[string]struct{}
	reader                Reader
	useProcessEnvironment bool
	refreshed             bool
	status                Status
}

func New() *Manager {
	initial := canonicalize(environmentMap(os.Environ()))
	manager := &Manager{
		initialValues:         cloneMap(initial),
		managedKeys:           map[string]struct{}{},
		reader:                readHostEnvironment,
		useProcessEnvironment: true,
		status: Status{
			Source:            "process_environment",
			LoadedAt:          time.Now().UTC(),
			LastRefreshStatus: "not_refreshed",
			VariableCount:     len(initial),
		},
	}
	// Identify keys owned by the refreshable host source without changing the
	// active runtime environment. Until Refresh succeeds, lookups stay live.
	if baseline, _, err := manager.reader(); err == nil {
		for key := range canonicalize(baseline) {
			manager.managedKeys[key] = struct{}{}
		}
	}
	return manager
}

// NewWithReader is primarily useful for deterministic refresh tests. Before
// the first successful refresh it reads from the supplied initial values.
func NewWithReader(initial map[string]string, reader Reader) *Manager {
	values := canonicalize(initial)
	if reader == nil {
		reader = readHostEnvironment
	}
	return &Manager{
		initialValues: cloneMap(values),
		managedKeys:   map[string]struct{}{},
		reader:        reader,
		status: Status{
			Source:            "process_environment",
			LoadedAt:          time.Now().UTC(),
			LastRefreshStatus: "not_refreshed",
			VariableCount:     len(values),
		},
	}
}

func (m *Manager) Lookup(name string) (string, bool) {
	if m == nil {
		return os.LookupEnv(name)
	}
	m.mu.RLock()
	if m.refreshed {
		value, ok := m.values[canonicalName(name)]
		m.mu.RUnlock()
		return value, ok
	}
	useProcessEnvironment := m.useProcessEnvironment
	initial := cloneMap(m.initialValues)
	m.mu.RUnlock()
	if useProcessEnvironment {
		return os.LookupEnv(name)
	}
	value, ok := initial[canonicalName(name)]
	return value, ok
}

func (m *Manager) Environ() []string {
	if m == nil {
		return os.Environ()
	}
	m.mu.RLock()
	if m.refreshed {
		values := cloneMap(m.values)
		m.mu.RUnlock()
		return sortedEnvironment(values)
	}
	useProcessEnvironment := m.useProcessEnvironment
	initial := cloneMap(m.initialValues)
	m.mu.RUnlock()
	if useProcessEnvironment {
		return append([]string(nil), os.Environ()...)
	}
	return sortedEnvironment(initial)
}

func (m *Manager) Status() Status {
	if m == nil {
		return Status{Source: "process_environment", LoadedAt: time.Now().UTC(), LastRefreshStatus: "unmanaged", VariableCount: len(environmentMap(os.Environ()))}
	}
	m.mu.RLock()
	status := cloneStatus(m.status)
	refreshed := m.refreshed
	useProcessEnvironment := m.useProcessEnvironment
	initialCount := len(m.initialValues)
	m.mu.RUnlock()
	if !refreshed {
		if useProcessEnvironment {
			status.VariableCount = len(environmentMap(os.Environ()))
		} else {
			status.VariableCount = initialCount
		}
	}
	return status
}

func (m *Manager) Refresh() Status {
	if m == nil {
		return Status{Source: "process_environment", LoadedAt: time.Now().UTC(), LastRefreshStatus: "error", LastRefreshError: "host environment manager is unavailable", VariableCount: len(environmentMap(os.Environ()))}
	}
	refreshedAt := time.Now().UTC()
	values, source, err := m.reader()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.LastRefreshAt = &refreshedAt
	if err != nil {
		m.status.LastRefreshStatus = "error"
		m.status.LastRefreshError = err.Error()
		return cloneStatus(m.status)
	}

	var next map[string]string
	if m.refreshed {
		next = cloneMap(m.values)
	} else if m.useProcessEnvironment {
		next = canonicalize(environmentMap(os.Environ()))
	} else {
		next = cloneMap(m.initialValues)
	}
	for key := range m.managedKeys {
		delete(next, key)
	}
	incoming := canonicalize(values)
	m.managedKeys = make(map[string]struct{}, len(incoming))
	for key, value := range incoming {
		next[key] = value
		m.managedKeys[key] = struct{}{}
	}
	if runtime.GOOS == "windows" {
		next = expandPercentReferences(next)
	}
	m.values = next
	m.refreshed = true
	m.status.Source = strings.TrimSpace(source)
	if m.status.Source == "" {
		m.status.Source = "host_environment"
	}
	m.status.LastRefreshStatus = "success"
	m.status.LastRefreshError = ""
	m.status.VariableCount = len(next)
	return cloneStatus(m.status)
}

func environmentMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, item := range values {
		index := strings.IndexByte(item, '=')
		if index <= 0 {
			continue
		}
		result[item[:index]] = item[index+1:]
	}
	return result
}

func canonicalize(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsRune(key, '=') || strings.ContainsRune(key, '\x00') || strings.ContainsRune(value, '\x00') {
			continue
		}
		result[canonicalName(key)] = value
	}
	return result
}

func canonicalName(name string) string {
	name = strings.TrimSpace(name)
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}

func cloneMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneStatus(status Status) Status {
	if status.LastRefreshAt != nil {
		value := status.LastRefreshAt.UTC()
		status.LastRefreshAt = &value
	}
	return status
}

func sortedEnvironment(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

func expandPercentReferences(values map[string]string) map[string]string {
	result := cloneMap(values)
	for pass := 0; pass < 4; pass++ {
		changed := false
		for key, value := range result {
			expanded := expandPercentValue(value, result)
			if expanded != value {
				result[key] = expanded
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return result
}

func expandPercentValue(value string, values map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '%' {
			out.WriteByte(value[i])
			i++
			continue
		}
		end := strings.IndexByte(value[i+1:], '%')
		if end < 0 {
			out.WriteString(value[i:])
			break
		}
		end += i + 1
		name := value[i+1 : end]
		if replacement, ok := values[canonicalName(name)]; ok {
			out.WriteString(replacement)
		} else {
			out.WriteString(value[i : end+1])
		}
		i = end + 1
	}
	return out.String()
}
