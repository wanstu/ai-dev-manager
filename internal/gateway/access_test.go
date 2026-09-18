package gateway

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

const (
	testAdminKey = "admin-0123456789abcdef0123456789"
	testAgentKey = "agent-0123456789abcdef0123456789"
)

func TestGatewayRequestAccessLocalAndRemote(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))

	local := httptest.NewRequest("POST", "http://127.0.0.1:43137/mcp", nil)
	local.Host = "127.0.0.1:43137"
	local.RemoteAddr = "127.0.0.1:54321"
	if !gatewayRequestAllowed(service, local, serverSurfaceAgent) {
		t.Fatal("loopback Agent MCP request should remain allowed without remote access configuration")
	}

	proxiedLocal := httptest.NewRequest("POST", "http://127.0.0.1:43137/mcp", nil)
	proxiedLocal.Host = "127.0.0.1:43137"
	proxiedLocal.RemoteAddr = "127.0.0.1:54322"
	proxiedLocal.Header.Set("X-Forwarded-For", "203.0.113.10")
	if gatewayRequestAllowed(service, proxiedLocal, serverSurfaceAgent) {
		t.Fatal("forwarded request must not use local no-key compatibility path")
	}

	remoteAgent := httptest.NewRequest("POST", "http://adm.example.com/mcp", nil)
	remoteAgent.Host = "adm.example.com"
	if gatewayRequestAllowed(service, remoteAgent, serverSurfaceAgent) {
		t.Fatal("remote Agent MCP request must be denied before access configuration")
	}

	if _, err := service.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAllowedHosts([]string{"adm.example.com"}); err != nil {
		t.Fatal(err)
	}

	if gatewayRequestAllowed(service, remoteAgent, serverSurfaceAgent) {
		t.Fatal("remote Agent MCP request without API key must be denied")
	}
	remoteAgent.Header.Set("X-ADM-API-Key", testAdminKey)
	if gatewayRequestAllowed(service, remoteAgent, serverSurfaceAgent) {
		t.Fatal("Admin API key must not authenticate Agent MCP")
	}
	remoteAgent.Header.Set("X-ADM-API-Key", testAgentKey)
	if !gatewayRequestAllowed(service, remoteAgent, serverSurfaceAgent) {
		t.Fatal("Agent MCP should accept the configured Agent API key")
	}

	remoteAdmin := httptest.NewRequest("POST", "http://adm.example.com/admin/mcp", nil)
	remoteAdmin.Host = "adm.example.com"
	remoteAdmin.Header.Set("X-ADM-API-Key", testAgentKey)
	if gatewayRequestAllowed(service, remoteAdmin, serverSurfaceAdmin) {
		t.Fatal("Agent API key must not authenticate Admin MCP")
	}
	remoteAdmin.Header.Set("X-ADM-API-Key", testAdminKey)
	if !gatewayRequestAllowed(service, remoteAdmin, serverSurfaceAdmin) {
		t.Fatal("Admin MCP should accept the configured Admin API key")
	}

	if gatewayRequestAllowed(service, local, serverSurfaceAgent) {
		t.Fatal("once Host access is configured, loopback MCP/Admin requests must also carry the surface-specific API key")
	}
	local.Header.Set("X-ADM-API-Key", testAgentKey)
	if !gatewayRequestAllowed(service, local, serverSurfaceAgent) {
		t.Fatal("loopback Agent MCP request should accept Agent API key while remote access is configured")
	}
}

func TestRemoteListenRequiresHostsAndBothKeys(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if remoteGatewayListenAllowed(service) {
		t.Fatal("remote listen must be disabled by default")
	}
	if _, err := service.SetGatewayAllowedHosts([]string{"101.37.171.174"}); err != nil {
		t.Fatal(err)
	}
	if remoteGatewayListenAllowed(service) {
		t.Fatal("allowlist without keys must not enable remote listen")
	}
	if _, err := service.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if remoteGatewayListenAllowed(service) {
		t.Fatal("allowlist plus Admin key only must not enable remote listen")
	}
	if _, err := service.SetGatewayAgentAPIKey(testAdminKey); err == nil {
		t.Fatal("Admin and Agent API keys must not be allowed to share the same secret")
	}
	if _, err := service.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}
	if !remoteGatewayListenAllowed(service) {
		t.Fatal("allowlist plus separate Admin and Agent keys should enable remote listen")
	}
}

func TestGatewayWildcardHostAllowsAnyHostButStillRequiresSurfaceKey(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))

	status, err := service.SetGatewayAllowedHosts([]string{"adm.example.com", "*", "101.37.171.174"})
	if err != nil {
		t.Fatal(err)
	}
	if len(status.AllowedHosts) != 1 || status.AllowedHosts[0] != "*" {
		t.Fatalf("wildcard should normalize to the only allowed host entry, got %#v", status.AllowedHosts)
	}
	if _, err := service.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}

	remote := httptest.NewRequest("POST", "http://other.example.net/mcp", nil)
	remote.Host = "other.example.net:43137"
	if gatewayRequestAllowed(service, remote, serverSurfaceAgent) {
		t.Fatal("wildcard host must not bypass Agent API key authentication")
	}
	remote.Header.Set("X-ADM-API-Key", testAgentKey)
	if !gatewayRequestAllowed(service, remote, serverSurfaceAgent) {
		t.Fatal("wildcard host with correct Agent API key should be allowed")
	}

	if _, err := service.SetGatewayAllowedHosts([]string{"0.0.0.0"}); err != nil {
		t.Fatal(err)
	}
	if gatewayRequestAllowed(service, remote, serverSurfaceAgent) {
		t.Fatal("0.0.0.0 must remain an exact host entry, not act as a wildcard")
	}
}
