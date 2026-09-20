package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultHTTPListen = "127.0.0.1:43137"

// ManagementAPIVersion is the compatibility contract for Desktop/CLI management.
// Increment it only when the Admin management surface is no longer backwards compatible.
const ManagementAPIVersion = 1

const (
	HTTPStateStopped      = "stopped"
	HTTPStateRunning      = "running"
	HTTPStateIncompatible = "incompatible"
)

type HTTPHealth struct {
	Name                 string `json:"name"`
	Version              string `json:"version"`
	ManagementAPIVersion int    `json:"management_api_version,omitempty"`
	Status               string `json:"status"`
	PID                  int    `json:"pid"`
	Transport            string `json:"transport"`
	OwnerID              string `json:"owner_id,omitempty"`
}

type HTTPStatus struct {
	State                string `json:"state"`
	Listen               string `json:"listen"`
	BaseURL              string `json:"base_url"`
	MCPURL               string `json:"mcp_url"`
	AdminMCPURL          string `json:"admin_mcp_url"`
	PID                  int    `json:"pid,omitempty"`
	Version              string `json:"version,omitempty"`
	ManagementAPIVersion int    `json:"management_api_version,omitempty"`
	RecognizedADMGateway bool   `json:"recognized_adm_gateway,omitempty"`
	OwnerID              string `json:"owner_id,omitempty"`
	Detail               string `json:"detail,omitempty"`
}

// HTTPTarget is the canonical ADM HTTP target shared by CLI and Desktop surfaces.
// It describes endpoint derivation only; it does not grant management authority.
type HTTPTarget struct {
	BaseURL     string
	HealthURL   string
	MCPURL      string
	AdminMCPURL string
}

func ResolveHTTPTarget(rawBaseURL string) (HTTPTarget, error) {
	baseURL, err := normalizeHTTPBaseURL(rawBaseURL)
	if err != nil {
		return HTTPTarget{}, err
	}
	return HTTPTarget{
		BaseURL:     baseURL,
		HealthURL:   baseURL + "/healthz",
		MCPURL:      baseURL + "/mcp",
		AdminMCPURL: baseURL + "/admin/mcp",
	}, nil
}

func LocalHTTPListenFromBaseURL(rawBaseURL string) (string, error) {
	target, err := ResolveHTTPTarget(rawBaseURL)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(target.BaseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" {
		return "", fmt.Errorf("local Gateway lifecycle requires an http ADM base URL; use status/inspection for remote HTTPS health checks")
	}
	if strings.Trim(parsed.Path, "/") != "" {
		return "", fmt.Errorf("local Gateway lifecycle requires an ADM base URL without a base path")
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" || port == "" {
		return "", fmt.Errorf("local Gateway lifecycle requires an explicit loopback host and port")
	}
	loopback := strings.EqualFold(host, "localhost")
	if !loopback {
		ip := net.ParseIP(host)
		loopback = ip != nil && ip.IsLoopback()
	}
	if !loopback {
		return "", fmt.Errorf("local Gateway lifecycle is only available for loopback ADM URLs")
	}
	return net.JoinHostPort(host, port), nil
}

func LocalHTTPLifecycleEligible(rawBaseURL string) bool {
	_, err := LocalHTTPListenFromBaseURL(rawBaseURL)
	return err == nil
}

func HTTPBaseURL(listen string) (string, error) {
	listen = strings.TrimSpace(listen)
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("invalid Gateway listen address %q; expected host:port", listen)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func CheckHTTPListenAvailable(listen string) error {
	listen = strings.TrimSpace(listen)
	if _, err := HTTPBaseURL(listen); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("Gateway listen address %s cannot bind: %w; the port may be occupied or reserved by the operating system", listen, err)
	}
	return listener.Close()
}

func InspectHTTP(listen string) (HTTPStatus, error) {
	listen = strings.TrimSpace(listen)
	baseURL, err := HTTPBaseURL(listen)
	if err != nil {
		return HTTPStatus{}, err
	}
	probeURL, err := HTTPProbeBaseURL(listen)
	if err != nil {
		return HTTPStatus{}, err
	}
	status, err := InspectHTTPBaseURL(probeURL)
	if err != nil {
		return HTTPStatus{}, err
	}
	status.Listen = listen
	status.BaseURL = baseURL
	status.MCPURL = baseURL + "/mcp"
	status.AdminMCPURL = baseURL + "/admin/mcp"
	return status, nil
}

func HTTPProbeBaseURL(listen string) (string, error) {
	listen = strings.TrimSpace(listen)
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("invalid Gateway listen address %q; expected host:port", listen)
	}
	switch strings.Trim(strings.ToLower(host), "[]") {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func InspectHTTPBaseURL(rawBaseURL string) (HTTPStatus, error) {
	target, err := ResolveHTTPTarget(rawBaseURL)
	if err != nil {
		return HTTPStatus{}, err
	}
	parsed, _ := url.Parse(target.BaseURL)
	status := HTTPStatus{
		State:       HTTPStateStopped,
		Listen:      parsed.Host,
		BaseURL:     target.BaseURL,
		MCPURL:      target.MCPURL,
		AdminMCPURL: target.AdminMCPURL,
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	response, err := client.Get(target.HealthURL)
	if err != nil {
		if isHTTPConnectionFailure(err) {
			return status, nil
		}
		return HTTPStatus{}, fmt.Errorf("check Gateway %s: %w", target.BaseURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		status.State = HTTPStateIncompatible
		status.Detail = fmt.Sprintf("endpoint %s responded with %s instead of an ADM V2 health response", target.BaseURL, response.Status)
		return status, nil
	}
	var health HTTPHealth
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		status.State = HTTPStateIncompatible
		status.Detail = fmt.Sprintf("endpoint %s returned invalid ADM V2 health JSON: %v", target.BaseURL, err)
		return status, nil
	}
	if health.Name != serverName || health.Status != "ok" || health.Transport != "http" {
		status.State = HTTPStateIncompatible
		status.Detail = fmt.Sprintf("endpoint %s is not the expected ADM V2 HTTP Gateway", target.BaseURL)
		return status, nil
	}
	status.RecognizedADMGateway = true
	status.PID = health.PID
	status.Version = health.Version
	status.ManagementAPIVersion = health.ManagementAPIVersion
	status.OwnerID = health.OwnerID
	if health.ManagementAPIVersion != ManagementAPIVersion {
		status.State = HTTPStateIncompatible
		status.Detail = fmt.Sprintf(
			"ADM Gateway version %s uses management API %d, but this client requires management API %d; management is disabled until the Gateway is upgraded. Forced stop remains available",
			strings.TrimSpace(health.Version), health.ManagementAPIVersion, ManagementAPIVersion,
		)
		return status, nil
	}
	status.State = HTTPStateRunning
	return status, nil
}

func normalizeHTTPBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("ADM base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid ADM base URL %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported ADM URL scheme %q; expected http or https", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("ADM base URL %q requires a host", raw)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("ADM base URL must not contain userinfo")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("ADM base URL must not contain query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed.String(), nil
}

func WaitHTTPReady(listen string, timeout time.Duration) (HTTPStatus, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := InspectHTTP(listen)
		if err != nil {
			return HTTPStatus{}, err
		}
		switch status.State {
		case HTTPStateRunning:
			return status, nil
		case HTTPStateIncompatible:
			return status, fmt.Errorf("Gateway endpoint is incompatible: %s", status.Detail)
		}
		time.Sleep(100 * time.Millisecond)
	}
	baseURL, _ := HTTPBaseURL(listen)
	return HTTPStatus{}, fmt.Errorf("Gateway did not become ready within %s: %s/healthz", timeout, baseURL)
}

func StopHTTP(listen string) (HTTPStatus, error) {
	return StopHTTPWithAPIKey(listen, "")
}

func StopHTTPWithAPIKey(listen, apiKey string) (HTTPStatus, error) {
	status, err := InspectHTTP(listen)
	if err != nil {
		return HTTPStatus{}, err
	}
	switch status.State {
	case HTTPStateStopped:
		return status, nil
	case HTTPStateIncompatible:
		return status, fmt.Errorf("refusing to stop incompatible process on %s without force: %s", listen, status.Detail)
	}
	if status.PID <= 0 {
		return status, fmt.Errorf("Gateway %s did not provide a usable PID", status.BaseURL)
	}
	if status.OwnerID != "" {
		if err := requestHTTPShutdown(status, apiKey); err != nil {
			return status, err
		}
		return InspectHTTP(listen)
	}
	if err := TerminateHTTPProcess(status.PID, listen); err != nil {
		return status, err
	}
	return InspectHTTP(listen)
}

func ForceStopHTTPBaseURLWithAPIKey(rawBaseURL, apiKey string) (HTTPStatus, error) {
	status, err := InspectHTTPBaseURL(rawBaseURL)
	if err != nil {
		return HTTPStatus{}, err
	}
	if status.State == HTTPStateStopped {
		return status, nil
	}
	if !status.RecognizedADMGateway {
		return status, fmt.Errorf("refusing to force-stop %s because it is not a recognized ADM Gateway", status.BaseURL)
	}
	if status.OwnerID != "" {
		if err := requestHTTPShutdown(status, apiKey); err != nil {
			return status, err
		}
		return InspectHTTPBaseURL(status.BaseURL)
	}
	if listen, localErr := LocalHTTPListenFromBaseURL(status.BaseURL); localErr == nil {
		if status.PID <= 0 {
			return status, fmt.Errorf("Gateway %s did not provide a usable PID", status.BaseURL)
		}
		if err := TerminateHTTPProcess(status.PID, listen); err != nil {
			return status, err
		}
		return InspectHTTPBaseURL(status.BaseURL)
	}
	return status, fmt.Errorf("remote Gateway %s cannot be force-stopped because it does not expose runtime-owner shutdown support", status.BaseURL)
}

func requestHTTPShutdown(status HTTPStatus, apiKey string) error {
	request, err := http.NewRequest(http.MethodPost, status.BaseURL+"/shutdown", nil)
	if err != nil {
		return err
	}
	request.Header.Set(runtimeOwnerHeader, status.OwnerID)
	if value := strings.TrimSpace(apiKey); value != "" {
		request.Header.Set(gatewayAPIKeyHeader, value)
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request graceful Gateway shutdown: %w", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Gateway refused graceful shutdown with %s", response.Status)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current, err := InspectHTTPBaseURL(status.BaseURL)
		if err != nil {
			return err
		}
		if current.State == HTTPStateStopped {
			return nil
		}
		if !current.RecognizedADMGateway || (current.OwnerID != "" && current.OwnerID != status.OwnerID) {
			return fmt.Errorf("Gateway runtime owner changed while waiting for graceful shutdown")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Gateway owner %s did not stop gracefully within 3s", status.OwnerID)
}

func TerminateHTTPProcess(pid int, listen string) error {
	baseURL, err := HTTPBaseURL(listen)
	if err != nil {
		return err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find Gateway process %d: %w", pid, err)
	}
	if err := process.Kill(); err != nil {
		return fmt.Errorf("stop Gateway process %d: %w", pid, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		connection, dialErr := net.DialTimeout("tcp", listen, 200*time.Millisecond)
		if dialErr != nil {
			return nil
		}
		_ = connection.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Gateway process %d was terminated but endpoint %s still responds", pid, baseURL)
}

func isHTTPConnectionFailure(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection refused") ||
		strings.Contains(message, "actively refused") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "forcibly closed by the remote host") ||
		strings.Contains(message, "use of closed network connection") ||
		(errors.As(err, &netErr) && netErr.Timeout())
}
