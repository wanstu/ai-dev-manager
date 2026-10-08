package gateway

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"ai-dev-manager-v2/internal/app"
)

const gatewayAPIKeyHeader = "X-ADM-API-Key"

type GatewayAccessPolicyInput struct {
	Enabled   bool     `json:"enabled"`
	Hosts     []string `json:"hosts"`
	ClientIPs []string `json:"client_ips"`
}

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
	// Recovery uses the TCP peer, never a caller-controlled Host header.
	localRecovery := isLocalGatewayHost(host) && directLoopbackPeer(request)
	keyConfigured := settings.AdminAPIKeyHash != "" && settings.AgentAPIKeyHash != ""
	if !keyConfigured {
		return localRecovery
	} // Local-only bootstrap.
	if app.GatewayWhitelistEnabled(settings) && !localRecovery {
		if len(settings.AllowedHosts) > 0 && !gatewayHostConfigured(host, settings.AllowedHosts) {
			return false
		}
		if len(settings.AllowedClientIPs) > 0 && !gatewayClientIPConfigured(request.RemoteAddr, settings.AllowedClientIPs) {
			return false
		}
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

// Trust only the transport peer; forwarding headers are untrusted.
func gatewayClientIPConfigured(remoteAddr string, configured []string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	peer = peer.Unmap()
	for _, item := range configured {
		if addr, err := netip.ParseAddr(item); err == nil && addr.Unmap() == peer {
			return true
		}
		if prefix, err := netip.ParsePrefix(item); err == nil && prefix.Contains(peer) {
			return true
		}
	}
	return false
}

// Web management uses the same network policy; login/session auth is separate.
func gatewayWebAccessAllowed(service *app.Service, request *http.Request) bool {
	if service == nil || request == nil {
		return false
	}
	settings, err := service.GatewayAccessConfig()
	if err != nil {
		return false
	}
	if !app.GatewayWhitelistEnabled(settings) {
		return true
	}
	host := gatewayAuthorityHost(request.Host)
	if isLocalGatewayHost(host) && directLoopbackPeer(request) {
		return true
	}
	if len(settings.AllowedHosts) > 0 && !gatewayHostConfigured(host, settings.AllowedHosts) {
		return false
	}
	if len(settings.AllowedClientIPs) > 0 && !gatewayClientIPConfigured(request.RemoteAddr, settings.AllowedClientIPs) {
		return false
	}
	return true
}

func remoteGatewayListenAllowed(service *app.Service) bool {
	if service == nil {
		return false
	}
	readiness, err := service.GatewayRemoteReadiness()
	return err == nil && readiness.Ready
}
