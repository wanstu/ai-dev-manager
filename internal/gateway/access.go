package gateway

import (
	"net"
	"net/http"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

const gatewayAPIKeyHeader = "X-ADM-API-Key"

type GatewayAllowedHostsInput struct {
	Hosts []string `json:"hosts"`
}

type GatewayAPIKeyInput struct {
	APIKey string `json:"api_key"`
}

type ExecFullAuthorizationInput struct {
	Enabled bool `json:"enabled"`
}

func gatewayRequestAllowed(service *app.Service, request *http.Request, surface serverSurface) bool {
	if service == nil || request == nil {
		return false
	}
	settings, err := service.GatewayAccessConfig()
	if err != nil {
		return false
	}
	host := gatewayAuthorityHost(request.Host)
	local := isLocalGatewayHost(host)
	remoteConfigured := len(settings.AllowedHosts) > 0
	if !remoteConfigured {
		return local && (host == "host.docker.internal" || directLoopbackPeer(request))
	}
	if !local && !gatewayHostConfigured(host, settings.AllowedHosts) {
		return false
	}
	secret := gatewayRequestAPIKey(request)
	if secret == "" {
		return false
	}
	var ok bool
	switch surface {
	case serverSurfaceAdmin:
		ok, err = service.VerifyGatewayAdminAPIKey(secret)
	case serverSurfaceAgent:
		ok, err = service.VerifyGatewayAgentAPIKey(secret)
	default:
		return false
	}
	return err == nil && ok
}

func gatewayShutdownRequestAllowed(service *app.Service, request *http.Request) bool {
	return gatewayRequestAllowed(service, request, serverSurfaceAdmin)
}

func directLoopbackPeer(request *http.Request) bool {
	if request == nil {
		return false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	if strings.TrimSpace(request.Header.Get("Forwarded")) != "" ||
		strings.TrimSpace(request.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(request.Header.Get("X-Real-IP")) != "" {
		return false
	}
	return true
}

func gatewayRequestAPIKey(request *http.Request) string {
	if request == nil {
		return ""
	}
	if value := strings.TrimSpace(request.Header.Get(gatewayAPIKeyHeader)); value != "" {
		return value
	}
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if len(authorization) > len("Bearer ") && strings.EqualFold(authorization[:len("Bearer ")], "Bearer ") {
		return strings.TrimSpace(authorization[len("Bearer "):])
	}
	return ""
}

func gatewayAuthorityHost(authority string) string {
	host := strings.TrimSpace(strings.ToLower(authority))
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return strings.Trim(strings.TrimSuffix(host, "."), "[]")
}

func isLocalGatewayHost(host string) bool {
	switch gatewayAuthorityHost(host) {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
}

func gatewayHostConfigured(host string, configured []string) bool {
	host = gatewayAuthorityHost(host)
	for _, candidate := range configured {
		normalized, err := app.NormalizeGatewayAllowedHost(candidate)
		if err != nil {
			continue
		}
		if normalized == "*" || strings.EqualFold(normalized, host) {
			return true
		}
	}
	return false
}

func remoteGatewayListenAllowed(service *app.Service) bool {
	if service == nil {
		return false
	}
	readiness, err := service.GatewayRemoteReadiness()
	return err == nil && readiness.Ready
}
