package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"ai-dev-manager/internal/adapter/mcpgateway"
)

const DefaultGatewayListen = "127.0.0.1:0"

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
	Endpoint       string       `json:"endpoint,omitempty"`
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
	requestedListen = strings.TrimSpace(requestedListen)
	if requestedListen != "" {
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
	if o.status.State == GatewayRunning && o.server != nil {
		if requestedListen != "" && requestedListen != o.status.Listen {
			return cloneGatewayStatus(o.status), fmt.Errorf("gateway already configured at %s; stop it before changing listen address", o.status.Listen)
		}
		if !desired.DesiredRunning || desired.Listen != o.status.Listen {
			if err := o.store.Save(GatewayDesired{DesiredRunning: true, Listen: o.status.Listen}); err != nil {
				return cloneGatewayStatus(o.status), err
			}
			o.status.DesiredRunning = true
		}
		return cloneGatewayStatus(o.status), nil
	}

	listen := requestedListen
	if desired.Listen != "" {
		if requestedListen != "" && requestedListen != desired.Listen {
			if desired.DesiredRunning {
				return GatewayStatus{}, fmt.Errorf("gateway desired listen is already %s; use gateway down before changing it", desired.Listen)
			}
			// An explicit down is the boundary that allows the user to
			// intentionally reconfigure the otherwise stable endpoint.
			listen = requestedListen
		} else {
			listen = desired.Listen
		}
	}
	if listen == "" {
		listen = DefaultGatewayListen
	}
	if err := validateLoopbackAddress(listen); err != nil {
		return GatewayStatus{}, err
	}

	if !isDynamicListen(listen) {
		if err := o.store.Save(GatewayDesired{DesiredRunning: true, Listen: listen}); err != nil {
			return GatewayStatus{}, err
		}
	} else if err := o.store.Save(GatewayDesired{DesiredRunning: true}); err != nil {
		return GatewayStatus{}, err
	}

	status, startErr := o.startObservedLocked(listen)
	if startErr != nil {
		return status, startErr
	}
	return status, nil
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
		o.status = GatewayStatus{DesiredRunning: false, Listen: desired.Listen, State: GatewayStopped}
		return cloneGatewayStatus(o.status), nil
	}
	listen := strings.TrimSpace(desired.Listen)
	if listen == "" {
		listen = DefaultGatewayListen
	}
	if err := validateLoopbackAddress(listen); err != nil {
		o.status = GatewayStatus{DesiredRunning: true, Listen: desired.Listen, State: GatewayError, Error: err.Error()}
		return cloneGatewayStatus(o.status), nil
	}
	status, startErr := o.startObservedLocked(listen)
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
	return GatewayStatus{DesiredRunning: desired.DesiredRunning, Listen: desired.Listen, State: GatewayStopped}, nil
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
	if err := o.store.Save(GatewayDesired{DesiredRunning: false, Listen: listen}); err != nil {
		return GatewayStatus{}, err
	}
	return o.stopObservedLocked(ctx, false, listen)
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
	_, err = o.stopObservedLocked(ctx, desired.DesiredRunning, desired.Listen)
	return err
}

func (o *GatewayOwner) startObservedLocked(listen string) (GatewayStatus, error) {
	status := GatewayStatus{DesiredRunning: true, Listen: listen, State: GatewayStarting}
	o.status = status

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		status.State = GatewayError
		status.Error = fmt.Sprintf("listen gateway: %v", err)
		o.status = status
		return cloneGatewayStatus(status), err
	}
	actual := listener.Addr().String()
	if err := validateLoopbackAddress(actual); err != nil {
		_ = listener.Close()
		status.State = GatewayError
		status.Error = err.Error()
		o.status = status
		return cloneGatewayStatus(status), err
	}

	if isDynamicListen(listen) {
		if err := o.store.Save(GatewayDesired{DesiredRunning: true, Listen: actual}); err != nil {
			_ = listener.Close()
			status.State = GatewayError
			status.Error = "persist concrete gateway listen: " + err.Error()
			o.status = status
			return cloneGatewayStatus(status), err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpgateway.NewHTTPHandler(o.discovery))
	server := &http.Server{Handler: mux}
	status.Listen = actual
	status.Endpoint = "http://" + actual + "/mcp"
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
			o.status.Error = "gateway server failed: " + err.Error()
		}
	}(server)

	return cloneGatewayStatus(status), nil
}

func (o *GatewayOwner) stopObservedLocked(ctx context.Context, preserveDesired bool, listen string) (GatewayStatus, error) {
	if o.server == nil {
		o.listener = nil
		o.status = GatewayStatus{DesiredRunning: preserveDesired, Listen: listen, State: GatewayStopped}
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
		o.status.Error = shutdownErr.Error()
		return cloneGatewayStatus(o.status), shutdownErr
	}
	o.status = GatewayStatus{DesiredRunning: preserveDesired, Listen: listen, State: GatewayStopped}
	return cloneGatewayStatus(o.status), nil
}

func isDynamicListen(addr string) bool {
	_, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	return err == nil && port == "0"
}

func cloneGatewayStatus(status GatewayStatus) GatewayStatus { return status }
