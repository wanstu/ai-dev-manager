package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/gateway"
)

func (a *Adapter) GetGatewayStatus() (gateway.HTTPStatus, error) {
	if err := a.ready(); err != nil {
		return gateway.HTTPStatus{}, err
	}
	return gateway.InspectHTTP(gateway.DefaultHTTPListen)
}

func (a *Adapter) InspectADMConnection(input ADMConnectionInput) (ADMConnectionStatus, error) {
	if a == nil {
		return ADMConnectionStatus{}, errors.New("desktop adapter is not initialized")
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		baseURL = defaultADMBaseURL()
	}
	status, err := gateway.InspectHTTPBaseURL(baseURL)
	if err != nil {
		return ADMConnectionStatus{}, err
	}
	return desktopConnectionStatus(status), nil
}

func (a *Adapter) ConnectADM(input ADMConnectionInput) (ADMConnectionStatus, error) {
	status, err := a.InspectADMConnection(input)
	if err != nil {
		if a != nil {
			a.management = nil
			a.runtime = nil
		}
		return ADMConnectionStatus{}, err
	}
	if status.State == gateway.HTTPStateRunning {
		client := adminmcp.NewWithAPIKey(status.AdminMCPURL, a.connectionAPIKey(status.BaseURL, input.APIKey))
		a.management = client
		a.runtime = client
	} else {
		a.management = nil
		a.runtime = nil
	}
	return status, nil
}

func (a *Adapter) StartLocalADM(input ADMConnectionInput) (ADMConnectionStatus, error) {
	if a == nil {
		return ADMConnectionStatus{}, errors.New("desktop adapter is not initialized")
	}
	status, err := a.InspectADMConnection(input)
	if err != nil {
		return ADMConnectionStatus{}, err
	}
	listen, err := localBootstrapListen(status.BaseURL)
	if err != nil {
		return status, err
	}
	switch status.State {
	case gateway.HTTPStateRunning:
		client := adminmcp.NewWithAPIKey(status.AdminMCPURL, a.connectionAPIKey(status.BaseURL, input.APIKey))
		a.management = client
		a.runtime = client
		return status, nil
	case gateway.HTTPStateIncompatible:
		return status, fmt.Errorf("refusing to start local ADM because %s is incompatible: %s", status.BaseURL, status.Detail)
	}
	if err := gateway.CheckHTTPListenAvailable(listen); err != nil {
		return status, err
	}
	process, err := startDetachedGatewayProcess(listen)
	if err != nil {
		return status, fmt.Errorf("start detached Gateway: %w", err)
	}
	ready, err := gateway.WaitHTTPReady(listen, 5*time.Second)
	if err != nil {
		_ = process.Kill()
		_ = process.Release()
		return status, err
	}
	_ = process.Release()
	connected := desktopConnectionStatus(ready)
	client := adminmcp.NewWithAPIKey(connected.AdminMCPURL, a.connectionAPIKey(connected.BaseURL, input.APIKey))
	a.management = client
	a.runtime = client
	return connected, nil
}

func (a *Adapter) StopLocalADM(input ADMConnectionInput) (ADMConnectionStatus, error) {
	if a == nil {
		return ADMConnectionStatus{}, errors.New("desktop adapter is not initialized")
	}
	status, err := a.InspectADMConnection(input)
	if err != nil {
		return ADMConnectionStatus{}, err
	}
	stopped, err := gateway.ForceStopHTTPBaseURLWithAPIKey(status.BaseURL, a.connectionAPIKey(status.BaseURL, input.APIKey))
	if err != nil {
		return status, err
	}
	a.management = nil
	a.runtime = nil
	return desktopConnectionStatus(stopped), nil
}

func (a *Adapter) StartGateway() (gateway.HTTPStatus, error) {
	if err := a.ready(); err != nil {
		return gateway.HTTPStatus{}, err
	}
	status, err := gateway.InspectHTTP(gateway.DefaultHTTPListen)
	if err != nil {
		return gateway.HTTPStatus{}, err
	}
	switch status.State {
	case gateway.HTTPStateRunning:
		return status, nil
	case gateway.HTTPStateIncompatible:
		return status, fmt.Errorf("refusing to start Gateway because %s is incompatible: %s", gateway.DefaultHTTPListen, status.Detail)
	}
	if err := gateway.CheckHTTPListenAvailable(gateway.DefaultHTTPListen); err != nil {
		return gateway.HTTPStatus{}, err
	}

	process, err := startDetachedGatewayProcess(gateway.DefaultHTTPListen)
	if err != nil {
		return gateway.HTTPStatus{}, fmt.Errorf("start detached Gateway: %w", err)
	}
	ready, err := gateway.WaitHTTPReady(gateway.DefaultHTTPListen, 5*time.Second)
	if err != nil {
		_ = process.Kill()
		_ = process.Release()
		return gateway.HTTPStatus{}, err
	}
	_ = process.Release()
	return ready, nil
}

func (a *Adapter) StopGateway() (gateway.HTTPStatus, error) {
	if err := a.ready(); err != nil {
		return gateway.HTTPStatus{}, err
	}
	return gateway.StopHTTPWithAPIKey(gateway.DefaultHTTPListen, a.connectionAPIKey(defaultADMBaseURL(), ""))
}

func defaultADMBaseURL() string {
	baseURL, _ := gateway.HTTPBaseURL(gateway.DefaultHTTPListen)
	return baseURL
}

func desktopConnectionStatus(status gateway.HTTPStatus) ADMConnectionStatus {
	target, _ := gateway.ResolveHTTPTarget(status.BaseURL)
	return ADMConnectionStatus{
		State:                  status.State,
		BaseURL:                status.BaseURL,
		Listen:                 status.Listen,
		HealthURL:              target.HealthURL,
		AgentMCPURL:            status.MCPURL,
		AdminMCPURL:            status.AdminMCPURL,
		PID:                    status.PID,
		Version:                status.Version,
		ManagementAPIVersion:   status.ManagementAPIVersion,
		RecognizedADMGateway:   status.RecognizedADMGateway,
		OwnerID:                status.OwnerID,
		Detail:                 status.Detail,
		LocalBootstrapEligible: gateway.LocalHTTPLifecycleEligible(status.BaseURL),
	}
}

func isLocalBootstrapBaseURL(raw string) bool {
	return gateway.LocalHTTPLifecycleEligible(raw)
}

func localBootstrapListen(raw string) (string, error) {
	return gateway.LocalHTTPListenFromBaseURL(raw)
}
