package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ai-dev-manager/internal/adapter/mcpgateway"
)

const (
	DefaultGatewayListen = "127.0.0.1:0"
	DockerGatewayHost    = "host.docker.internal"
	gatewayTraceEnv      = "ADM_GATEWAY_TRACE"
	gatewayTraceFileEnv  = "ADM_GATEWAY_TRACE_FILE"
	gatewayTraceFilename = "gateway-trace.log"
)

type GatewayState string

const (
	GatewayStarting GatewayState = "starting"
	GatewayRunning  GatewayState = "running"
	GatewayStopping GatewayState = "stopping"
	GatewayStopped  GatewayState = "stopped"
	GatewayError    GatewayState = "error"
)

type GatewayStatus struct {
	DesiredRunning bool         `json:"desired_running"`
	Listen         string       `json:"listen,omitempty"`
	Exposed        bool         `json:"exposed,omitempty"`
	Endpoint       string       `json:"endpoint,omitempty"`
	LocalEndpoint  string       `json:"local_endpoint,omitempty"`
	DockerEndpoint string       `json:"docker_endpoint,omitempty"`
	State          GatewayState `json:"state"`
	Error          string       `json:"error,omitempty"`
}

type GatewayOwner struct {
	mu        sync.Mutex
	store     *GatewayStore
	discovery mcpgateway.Discovery
	server    *http.Server
	listener  net.Listener
	status    GatewayStatus
}

func NewGatewayOwner(root string, discovery mcpgateway.Discovery) (*GatewayOwner, error) {
	if discovery == nil {
		return nil, errors.New("gateway discovery service is required")
	}
	return &GatewayOwner{
		store:     NewGatewayStore(root),
		discovery: discovery,
	}, nil
}

func (o *GatewayOwner) Up(ctx context.Context, requestedListen string) (GatewayStatus, error) {
	return o.upWithMode(ctx, requestedListen, false)
}

func (o *GatewayOwner) UpDocker(ctx context.Context) (GatewayStatus, error) {
	return o.upWithMode(ctx, "", true)
}

func (o *GatewayOwner) upWithMode(ctx context.Context, requestedListen string, exposed bool) (GatewayStatus, error) {
	requestedListen = strings.TrimSpace(requestedListen)
	if exposed && requestedListen != "" {
		return GatewayStatus{}, errors.New("docker Gateway mode does not accept a custom listen address")
	}
	if !exposed && requestedListen != "" {
		if err := validateLoopbackAddress(requestedListen); err != nil {
			return GatewayStatus{}, err
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	desired, err := o.store.Load()
	if err != nil {
		return GatewayStatus{}, err
	}
	listen, err := gatewayListenForMode(desired, o.status, requestedListen, exposed)
	if err != nil {
		return GatewayStatus{}, err
	}

	if o.status.State == GatewayRunning && o.server != nil {
		if o.status.Listen == listen && o.status.Exposed == exposed {
			if !desired.DesiredRunning || desired.Listen != listen || desired.Exposed != exposed {
				if err := o.store.Save(GatewayDesired{DesiredRunning: true, Listen: listen, Exposed: exposed}); err != nil {
					return cloneGatewayStatus(o.status), err
				}
				o.status.DesiredRunning = true
			}
			return cloneGatewayStatus(o.status), nil
		}
		if requestedListen != "" && desired.DesiredRunning && desired.Exposed == exposed {
			return cloneGatewayStatus(o.status), fmt.Errorf("gateway already configured at %s; use gateway down before changing listen address", o.status.Listen)
		}
		if _, err := o.stopObservedLocked(ctx, true, listen, exposed); err != nil {
			return cloneGatewayStatus(o.status), err
		}
	}

	persisted := GatewayDesired{DesiredRunning: true, Exposed: exposed}
	if !isDynamicListen(listen) {
		persisted.Listen = listen
	}
	if err := o.store.Save(persisted); err != nil {
		return GatewayStatus{}, err
	}
	status, startErr := o.startObservedLocked(listen, exposed)
	if startErr != nil {
		return status, startErr
	}
	return status, nil
}

func gatewayListenForMode(desired GatewayDesired, observed GatewayStatus, requestedListen string, exposed bool) (string, error) {
	if requestedListen != "" {
		return requestedListen, nil
	}

	port := "0"
	for _, candidate := range []string{observed.Listen, desired.Listen} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		_, candidatePort, err := net.SplitHostPort(candidate)
		if err == nil && candidatePort != "" {
			port = candidatePort
			break
		}
	}
	if exposed {
		return net.JoinHostPort("0.0.0.0", port), nil
	}
	if !desired.Exposed && strings.TrimSpace(desired.Listen) != "" {
		if err := validateLoopbackAddress(desired.Listen); err == nil {
			return desired.Listen, nil
		}
	}
	return net.JoinHostPort("127.0.0.1", port), nil
}

// Reconcile restores the persisted Gateway intent. A bind failure is recorded
// as observed GatewayError but does not prevent the control daemon from running;
// silently selecting a new port would invalidate the Agent's configured URL.
func (o *GatewayOwner) Reconcile(ctx context.Context) (GatewayStatus, error) {
	_ = ctx
	o.mu.Lock()
	defer o.mu.Unlock()

	desired, err := o.store.Load()
	if err != nil {
		return GatewayStatus{}, err
	}
	if !desired.DesiredRunning {
		o.status = GatewayStatus{DesiredRunning: false, Listen: desired.Listen, Exposed: desired.Exposed, State: GatewayStopped}
		return cloneGatewayStatus(o.status), nil
	}
	listen := strings.TrimSpace(desired.Listen)
	if listen == "" {
		if desired.Exposed {
			listen = "0.0.0.0:0"
		} else {
			listen = DefaultGatewayListen
		}
	}
	if err := validateGatewayListen(listen, desired.Exposed); err != nil {
		o.status = GatewayStatus{DesiredRunning: true, Listen: desired.Listen, Exposed: desired.Exposed, State: GatewayError, Error: err.Error()}
		return cloneGatewayStatus(o.status), nil
	}
	status, startErr := o.startObservedLocked(listen, desired.Exposed)
	if startErr != nil {
		return status, nil
	}
	return status, nil
}

func (o *GatewayOwner) Status() (GatewayStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.status.State != "" {
		return cloneGatewayStatus(o.status), nil
	}
	desired, err := o.store.Load()
	if err != nil {
		return GatewayStatus{}, err
	}
	return GatewayStatus{DesiredRunning: desired.DesiredRunning, Listen: desired.Listen, Exposed: desired.Exposed, State: GatewayStopped}, nil
}

func (o *GatewayOwner) Down(ctx context.Context) (GatewayStatus, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	desired, err := o.store.Load()
	if err != nil {
		return GatewayStatus{}, err
	}
	listen := desired.Listen
	if o.status.Listen != "" {
		listen = o.status.Listen
	}
	exposed := desired.Exposed
	if o.status.State != "" {
		exposed = o.status.Exposed
	}
	if err := o.store.Save(GatewayDesired{DesiredRunning: false, Listen: listen, Exposed: exposed}); err != nil {
		return GatewayStatus{}, err
	}
	return o.stopObservedLocked(ctx, false, listen, exposed)
}

// CloseObserved is used during daemon shutdown. It preserves desired=true so
// the exact concrete Gateway address is reconciled on the next daemon start.
func (o *GatewayOwner) CloseObserved(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	desired, err := o.store.Load()
	if err != nil {
		return err
	}
	_, err = o.stopObservedLocked(ctx, desired.DesiredRunning, desired.Listen, desired.Exposed)
	return err
}

func (o *GatewayOwner) startObservedLocked(listen string, exposed bool) (GatewayStatus, error) {
	status := GatewayStatus{DesiredRunning: true, Listen: listen, Exposed: exposed, State: GatewayStarting}
	o.status = status
	if err := validateGatewayListen(listen, exposed); err != nil {
		status.State = GatewayError
		status.Error = err.Error()
		o.status = status
		return cloneGatewayStatus(status), err
	}

	network := "tcp"
	if exposed {
		network = "tcp4"
	}
	listener, err := net.Listen(network, listen)
	if err != nil {
		status.State = GatewayError
		status.Error = fmt.Sprintf("listen gateway: %v", err)
		o.status = status
		return cloneGatewayStatus(status), err
	}
	actual := listener.Addr().String()
	if err := validateGatewayListen(actual, exposed); err != nil {
		_ = listener.Close()
		status.State = GatewayError
		status.Error = err.Error()
		o.status = status
		return cloneGatewayStatus(status), err
	}

	if isDynamicListen(listen) {
		if err := o.store.Save(GatewayDesired{DesiredRunning: true, Listen: actual, Exposed: exposed}); err != nil {
			_ = listener.Close()
			status.State = GatewayError
			status.Error = "persist concrete gateway listen: " + err.Error()
			o.status = status
			return cloneGatewayStatus(status), err
		}
	}

	handler := mcpgateway.NewHTTPHandler(o.discovery)
	if exposed {
		handler = mcpgateway.NewExposedHTTPHandler(o.discovery, mcpgateway.ExposedHTTPOptions{
			AllowedHosts: []string{DockerGatewayHost},
		})
	}
	handler = traceGatewayHandler(o.store.root, handler)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/mcp/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/" {
			http.NotFound(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: mux}
	status.Listen = actual
	_, port, _ := net.SplitHostPort(actual)
	status.LocalEndpoint = "http://127.0.0.1:" + port + "/mcp"
	status.Endpoint = status.LocalEndpoint
	if exposed {
		status.DockerEndpoint = "http://" + DockerGatewayHost + ":" + port + "/mcp"
	}
	status.State = GatewayRunning
	status.Error = ""
	o.listener = listener
	o.server = server
	o.status = status

	go func(expected *http.Server) {
		err := expected.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.server == expected {
			o.status.State = GatewayError
			o.status.Endpoint = ""
			o.status.LocalEndpoint = ""
			o.status.DockerEndpoint = ""
			o.status.Error = "gateway server failed: " + err.Error()
		}
	}(server)

	return cloneGatewayStatus(status), nil
}

func (o *GatewayOwner) stopObservedLocked(ctx context.Context, preserveDesired bool, listen string, exposed bool) (GatewayStatus, error) {
	if o.server == nil {
		o.listener = nil
		o.status = GatewayStatus{DesiredRunning: preserveDesired, Listen: listen, Exposed: exposed, State: GatewayStopped}
		return cloneGatewayStatus(o.status), nil
	}
	server := o.server
	o.status.State = GatewayStopping
	o.status.DesiredRunning = preserveDesired
	shutdownErr := server.Shutdown(ctx)
	if o.listener != nil {
		_ = o.listener.Close()
	}
	o.server = nil
	o.listener = nil
	if shutdownErr != nil {
		o.status.State = GatewayError
		o.status.Endpoint = ""
		o.status.LocalEndpoint = ""
		o.status.DockerEndpoint = ""
		o.status.Error = shutdownErr.Error()
		return cloneGatewayStatus(o.status), shutdownErr
	}
	o.status = GatewayStatus{DesiredRunning: preserveDesired, Listen: listen, Exposed: exposed, State: GatewayStopped}
	return cloneGatewayStatus(o.status), nil
}

func traceGatewayHandler(root string, next http.Handler) http.Handler {
	if strings.TrimSpace(os.Getenv(gatewayTraceEnv)) != "1" {
		return next
	}
	tracePath := strings.TrimSpace(os.Getenv(gatewayTraceFileEnv))
	if tracePath == "" {
		tracePath = filepath.Join(root, runtimeDirName, gatewayTraceFilename)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = os.MkdirAll(filepath.Dir(tracePath), 0o700)
		if file, err := os.OpenFile(tracePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			_, _ = fmt.Fprintf(file, "%s method=%s path=%q host=%q origin=%q content_type=%q accept=%q user_agent=%q\n",
				time.Now().Format(time.RFC3339Nano), r.Method, r.URL.Path, r.Host, r.Header.Get("Origin"), r.Header.Get("Content-Type"), r.Header.Get("Accept"), r.Header.Get("User-Agent"))
			_ = file.Close()
		}
		next.ServeHTTP(w, r)
	})
}

func validateGatewayListen(addr string, exposed bool) error {
	if !exposed {
		return validateLoopbackAddress(addr)
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return fmt.Errorf("invalid Gateway listen address %q", addr)
	}
	if host != "0.0.0.0" {
		return fmt.Errorf("Docker Gateway listen must bind 0.0.0.0 explicitly: %q", addr)
	}
	return nil
}

func isDynamicListen(addr string) bool {
	_, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	return err == nil && port == "0"
}

func cloneGatewayStatus(status GatewayStatus) GatewayStatus { return status }
