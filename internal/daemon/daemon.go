package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ai-dev-manager/internal/adapter/runtimeadapter"
	"ai-dev-manager/internal/agent"
	"ai-dev-manager/internal/config"
	"ai-dev-manager/internal/controlplane"
	"ai-dev-manager/internal/environment"
	"ai-dev-manager/internal/gateway"
)

const (
	runtimeDirName       = "runtime"
	metadataFilename     = "daemon.json"
	leaseFilename        = "daemon.lock"
	defaultListenAddress = "127.0.0.1:0"
	leaseHeartbeat       = time.Second
	leaseStaleAfter      = 4 * time.Second
	startReadyTimeout    = 5 * time.Second
	controlTimeout       = time.Second
	shutdownTimeout      = 3 * time.Second
)

// ChildEnvironmentKey marks the internal daemon child process. Production does
// not depend on the value, but tests can use it to route a generated test
// binary through the real CLI entrypoint.
const ChildEnvironmentKey = "AI_DEV_MANAGER_INTERNAL_DAEMON"

type State string

const (
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateError    State = "error"
)

var (
	ErrNotRunning     = errors.New("daemon not running")
	ErrAlreadyRunning = errors.New("daemon already running or starting")
	ErrCorruptState   = errors.New("corrupt daemon state")
	ErrUnsafeEndpoint = errors.New("unsafe daemon control endpoint")
)

type Metadata struct {
	InstanceID      string    `json:"instance_id,omitempty"`
	PID             int       `json:"pid,omitempty"`
	ControlEndpoint string    `json:"control_endpoint,omitempty"`
	State           State     `json:"state"`
	StartedAt       time.Time `json:"started_at,omitempty"`
}

type lease struct {
	path       string
	instanceID string
}

func ResolveRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		resolved, err := config.DefaultRoot()
		if err != nil {
			return "", err
		}
		root = resolved
	}
	return filepath.Clean(root), nil
}

func metadataPath(root string) string {
	return filepath.Join(root, runtimeDirName, metadataFilename)
}

func leasePath(root string) string {
	return filepath.Join(root, runtimeDirName, leaseFilename)
}

func ReadMetadata(root string) (Metadata, error) {
	resolved, err := ResolveRoot(root)
	if err != nil {
		return Metadata{}, err
	}
	data, err := os.ReadFile(metadataPath(resolved))
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, ErrNotRunning
	}
	if err != nil {
		return Metadata{}, fmt.Errorf("read daemon metadata: %w", err)
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, fmt.Errorf("%w: %v", ErrCorruptState, err)
	}
	if strings.TrimSpace(meta.InstanceID) == "" || meta.PID <= 0 || strings.TrimSpace(meta.ControlEndpoint) == "" {
		return Metadata{}, ErrCorruptState
	}
	return meta, nil
}

func writeMetadata(root string, meta Metadata) error {
	dir := filepath.Join(root, runtimeDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create daemon runtime directory: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".daemon-*.tmp")
	if err != nil {
		return fmt.Errorf("create daemon metadata temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	path := metadataPath(root)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace daemon metadata: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish daemon metadata: %w", err)
	}
	return nil
}

func removeMetadataIfInstance(root, instanceID string) error {
	meta, err := ReadMetadata(root)
	if errors.Is(err, ErrNotRunning) || errors.Is(err, ErrCorruptState) {
		if errors.Is(err, ErrCorruptState) {
			return os.Remove(metadataPath(root))
		}
		return nil
	}
	if err != nil {
		return err
	}
	if meta.InstanceID != instanceID {
		return nil
	}
	if err := os.Remove(metadataPath(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func acquireLease(root, instanceID string) (*lease, error) {
	dir := filepath.Join(root, runtimeDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := leasePath(root)
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) <= leaseStaleAfter {
			return nil, ErrAlreadyRunning
		}
		_ = os.Remove(path)
		_ = os.Remove(metadataPath(root))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(instanceID + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &lease{path: path, instanceID: instanceID}, nil
}

func (l *lease) touch() {
	data, err := os.ReadFile(l.path)
	if err != nil || strings.TrimSpace(string(data)) != l.instanceID {
		return
	}
	now := time.Now()
	_ = os.Chtimes(l.path, now, now)
}

func (l *lease) release() error {
	data, err := os.ReadFile(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != l.instanceID {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func newInstanceID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "daemon_" + hex.EncodeToString(buf), nil
}

func validateLoopbackAddress(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid daemon listen address %q", addr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("daemon listen address must be loopback: %q", addr)
	}
	return nil
}

func controlURL(endpoint, path string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return "", ErrUnsafeEndpoint
	}
	host := parsed.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return "", ErrUnsafeEndpoint
		}
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func Probe(ctx context.Context, meta Metadata) (Metadata, error) {
	target, err := controlURL(meta.ControlEndpoint, "/health")
	if err != nil {
		return Metadata{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Metadata{}, err
	}
	client := &http.Client{Timeout: controlTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("daemon health returned %s", resp.Status)
	}
	var observed Metadata
	if err := json.NewDecoder(resp.Body).Decode(&observed); err != nil {
		return Metadata{}, err
	}
	if observed.InstanceID != meta.InstanceID || observed.PID != meta.PID {
		return Metadata{}, fmt.Errorf("daemon identity mismatch")
	}
	return observed, nil
}

func Status(ctx context.Context, root string) (Metadata, error) {
	meta, err := ReadMetadata(root)
	if err != nil {
		return Metadata{}, err
	}
	observed, err := Probe(ctx, meta)
	if err != nil {
		meta.State = StateError
		return meta, fmt.Errorf("daemon metadata is stale or unhealthy: %w", err)
	}
	return observed, nil
}

// Start launches the current executable in internal daemon mode and waits for
// observed readiness. It never treats process creation alone as success. If a
// crashed daemon left a fresh heartbeat lease behind, Start waits until that
// lease becomes stale and then safely reclaims it within a bounded window.
func Start(ctx context.Context, root, executable string) (Metadata, error) {
	resolved, err := ResolveRoot(root)
	if err != nil {
		return Metadata{}, err
	}
	startCtx, cancelStart := context.WithTimeout(ctx, leaseStaleAfter+startReadyTimeout+2*time.Second)
	defer cancelStart()

	for {
		probeCtx, cancel := context.WithTimeout(startCtx, 250*time.Millisecond)
		current, statusErr := Status(probeCtx, resolved)
		cancel()
		if statusErr == nil {
			return current, nil
		}

		info, leaseErr := os.Stat(leasePath(resolved))
		if leaseErr == nil && time.Since(info.ModTime()) <= leaseStaleAfter {
			select {
			case <-startCtx.Done():
				return Metadata{}, fmt.Errorf("daemon owner did not become healthy before recovery timeout: %w", startCtx.Err())
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}
		if leaseErr != nil && !errors.Is(leaseErr, os.ErrNotExist) {
			return Metadata{}, leaseErr
		}
		if err := cleanupStaleState(resolved); err != nil {
			if errors.Is(err, ErrAlreadyRunning) {
				continue
			}
			return Metadata{}, err
		}
		break
	}

	if strings.TrimSpace(executable) == "" {
		executable, err = os.Executable()
		if err != nil {
			return Metadata{}, err
		}
	}
	cmd := exec.Command(executable, "--config-root", resolved, "_daemon-run")
	cmd.Env = append(os.Environ(), ChildEnvironmentKey+"=1")
	if err := cmd.Start(); err != nil {
		return Metadata{}, fmt.Errorf("start daemon process: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	return waitForHealthy(startCtx, resolved, exited)
}

func waitForHealthy(ctx context.Context, root string, exited <-chan error) (Metadata, error) {
	deadline := time.NewTimer(startReadyTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		meta, err := Status(probeCtx, root)
		cancel()
		if err == nil {
			return meta, nil
		}
		select {
		case <-ctx.Done():
			return Metadata{}, ctx.Err()
		case err := <-exited:
			probeCtx, cancel := context.WithTimeout(context.Background(), controlTimeout)
			meta, probeErr := Status(probeCtx, root)
			cancel()
			if probeErr == nil {
				return meta, nil
			}
			if err == nil {
				err = errors.New("daemon child exited before readiness")
			}
			return Metadata{}, err
		case <-deadline.C:
			return Metadata{}, errors.New("daemon did not become ready")
		case <-ticker.C:
		}
	}
}

func cleanupStaleState(root string) error {
	if info, err := os.Stat(leasePath(root)); err == nil {
		if time.Since(info.ModTime()) <= leaseStaleAfter {
			return ErrAlreadyRunning
		}
		if err := os.Remove(leasePath(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(metadataPath(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func Stop(ctx context.Context, root string) (Metadata, error) {
	resolved, err := ResolveRoot(root)
	if err != nil {
		return Metadata{}, err
	}
	meta, err := Status(ctx, resolved)
	if err != nil {
		if errors.Is(err, ErrNotRunning) {
			return Metadata{State: StateStopped}, nil
		}
		return Metadata{}, err
	}
	target, err := controlURL(meta.ControlEndpoint, "/stop")
	if err != nil {
		return Metadata{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, nil)
	if err != nil {
		return Metadata{}, err
	}
	client := &http.Client{Timeout: controlTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("daemon stop returned %s", resp.Status)
	}

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, readErr := ReadMetadata(resolved)
		if errors.Is(readErr, ErrNotRunning) {
			meta.State = StateStopped
			return meta, nil
		}
		select {
		case <-ctx.Done():
			return Metadata{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Run owns exactly one long-lived Control Plane for this daemon process.
func Run(ctx context.Context, root, listen string) error {
	// This marker exists only to bootstrap a generated Go test binary into the
	// daemon CLI path. Once the daemon entrypoint is running it must not leak to
	// Runtime/verifier children, or their own test entrypoints can mis-detect
	// themselves as ADM daemon children.
	if err := os.Unsetenv(ChildEnvironmentKey); err != nil {
		return fmt.Errorf("clear internal daemon child marker: %w", err)
	}
	resolved, err := ResolveRoot(root)
	if err != nil {
		return err
	}
	if strings.TrimSpace(listen) == "" {
		listen = defaultListenAddress
	}
	if err := validateLoopbackAddress(listen); err != nil {
		return err
	}
	instanceID, err := newInstanceID()
	if err != nil {
		return err
	}
	ownership, err := acquireLease(resolved, instanceID)
	if err != nil {
		return err
	}
	defer ownership.release()

	service, err := controlplane.New(resolved)
	if err != nil {
		return err
	}
	runtimeOwner := NewRuntimeOwner(service)
	agentManager, err := agent.NewManager(service.Registry().Get, agent.LifecycleExecutor{})
	if err != nil {
		return err
	}
	verifyExecutor, err := agent.NewVerifyWorkflowExecutor(func(workspaceID string) (runtimeadapter.Runtime, error) {
		return service.BuildRuntime(workspaceID, nil)
	})
	if err != nil {
		return err
	}
	if err := agentManager.RegisterExecutor(verifyExecutor); err != nil {
		return err
	}
	gsdExecutor, err := agent.NewGSDWorkflowExecutor(func(workspaceID string) (runtimeadapter.Runtime, error) {
		return service.BuildRuntime(workspaceID, nil)
	})
	if err != nil {
		return err
	}
	if err := agentManager.RegisterExecutor(gsdExecutor); err != nil {
		return err
	}
	parallelExecutor, err := agent.NewParallelVerifyExecutor(
		func(workspaceID string) (runtimeadapter.Runtime, error) {
			return service.BuildRuntime(workspaceID, nil)
		},
		func(workspaceID, derivedID, path string) (runtimeadapter.Runtime, error) {
			return service.BuildDerivedRuntime(workspaceID, derivedID, path)
		},
	)
	if err != nil {
		return err
	}
	if err := agentManager.RegisterExecutor(parallelExecutor); err != nil {
		return err
	}
	environmentManager, err := environment.NewManager(
		environment.NewStore(resolved),
		service.Registry().Get,
		func(workspaceID string) (runtimeadapter.Runtime, error) {
			return service.BuildRuntime(workspaceID, nil)
		},
		service.BuildDerivedRuntime,
	)
	if err != nil {
		return err
	}
	gatewayService, err := gateway.NewService(service.Registry(), environmentManager)
	if err != nil {
		return err
	}
	gatewayOwner, err := NewGatewayOwner(resolved, gatewayService)
	if err != nil {
		return err
	}
	if _, err := runtimeOwner.Reconcile(ctx); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = service.StopAll(shutdownCtx)
		return fmt.Errorf("reconcile desired runtimes: %w", err)
	}
	if _, err := gatewayOwner.Reconcile(ctx); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = runtimeOwner.StopAll(shutdownCtx)
		_ = service.StopAll(shutdownCtx)
		return fmt.Errorf("reconcile gateway: %w", err)
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = gatewayOwner.CloseObserved(shutdownCtx)
		_ = runtimeOwner.StopAll(shutdownCtx)
		_ = service.StopAll(shutdownCtx)
		return fmt.Errorf("listen daemon control endpoint: %w", err)
	}

	meta := Metadata{
		InstanceID:      instanceID,
		PID:             os.Getpid(),
		ControlEndpoint: "http://" + listener.Addr().String(),
		State:           StateRunning,
		StartedAt:       time.Now().UTC(),
	}
	stopCh := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(meta)
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		select {
		case stopCh <- struct{}{}:
		default:
		}
		stopping := meta
		stopping.State = StateStopping
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(stopping)
	})
	registerRuntimeHandlers(mux, runtimeOwner)
	registerAgentHandlers(mux, agentManager)
	registerEnvironmentHandlers(mux, environmentManager)
	registerGatewayHandlers(mux, gatewayOwner)

	httpServer := &http.Server{Handler: mux}
	serveErr := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()
	if err := writeMetadata(resolved, meta); err != nil {
		_ = listener.Close()
		return err
	}
	defer removeMetadataIfInstance(resolved, instanceID)

	heartbeatDone := make(chan struct{})
	var heartbeatOnce sync.Once
	stopHeartbeat := func() { heartbeatOnce.Do(func() { close(heartbeatDone) }) }
	defer stopHeartbeat()
	go func() {
		ticker := time.NewTicker(leaseHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ticker.C:
				ownership.touch()
			}
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case <-stopCh:
	case runErr = <-serveErr:
	}
	stopHeartbeat()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	shutdownErr := httpServer.Shutdown(shutdownCtx)
	agentManager.StopAll()
	gatewayErr := gatewayOwner.CloseObserved(shutdownCtx)
	runtimeErr := runtimeOwner.StopAll(shutdownCtx)
	controlErr := service.StopAll(shutdownCtx)
	return errors.Join(runErr, shutdownErr, gatewayErr, runtimeErr, controlErr)
}
