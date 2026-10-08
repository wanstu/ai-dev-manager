package gateway

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayAccessPolicyHostAndTransportIP(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := service.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}
	mk := func(host, addr string) *http.Request {
		r := httptest.NewRequest("POST", "http://"+host+"/mcp", nil)
		r.Host = host
		r.RemoteAddr = addr
		r.Header.Set("X-ADM-API-Key", testAgentKey)
		return r
	}
	allowed := mk("adm.example.com:8001", "10.0.0.8:55888")
	other := mk("other.example.com:8001", "198.51.100.6:55001")
	if !gatewayRequestAllowed(service, allowed, serverSurfaceAgent) || !gatewayRequestAllowed(service, other, serverSurfaceAgent) {
		t.Fatal("new install should not restrict Host or client IP once authenticated")
	}
	if _, err := service.SetGatewayAccessPolicy(true, []string{"ADM.EXAMPLE.COM"}, []string{"10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	if !gatewayRequestAllowed(service, allowed, serverSurfaceAgent) {
		t.Fatal("allowed host and peer must succeed")
	}
	if gatewayRequestAllowed(service, other, serverSurfaceAgent) {
		t.Fatal("unlisted host/peer must fail")
	}
	forged := mk("ADM.EXAMPLE.COM:8001", "198.51.100.6:55001")
	forged.Header.Set("X-Forwarded-For", "10.0.0.8")
	forged.Header.Set("X-Real-IP", "10.0.0.8")
	forged.Header.Set("Forwarded", "for=10.0.0.8")
	if gatewayRequestAllowed(service, forged, serverSurfaceAgent) {
		t.Fatal("forged forwarding headers must not bypass source restrictions")
	}
	wrongHost := mk("other.example.com", "10.0.0.8:55888")
	if gatewayRequestAllowed(service, wrongHost, serverSurfaceAgent) {
		t.Fatal("Host and source-IP restrictions require AND")
	}
	if _, err := service.SetGatewayAccessPolicy(false, []string{"adm.example.com"}, []string{"10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	if !gatewayRequestAllowed(service, other, serverSurfaceAgent) {
		t.Fatal("disabled whitelist should permit authenticated peers")
	}
	other.Header.Del("X-ADM-API-Key")
	if gatewayRequestAllowed(service, other, serverSurfaceAgent) {
		t.Fatal("disabled whitelist must never bypass authentication")
	}
}

func TestGatewayWhitelistRejectsInvalidOrEmptyAndKeepsLegacyPolicy(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := service.SetGatewayAccessPolicy(true, nil, nil); err == nil {
		t.Fatal("empty enabled whitelist must fail")
	}
	if _, err := service.SetGatewayAccessPolicy(true, []string{"*"}, nil); err == nil {
		t.Fatal("wildcard-only enabled whitelist must fail")
	}
	if _, err := service.SetGatewayAccessPolicy(true, nil, []string{"127.0.0.1/not-a-cidr"}); err == nil {
		t.Fatal("invalid CIDR must fail")
	}
	if _, err := service.SetGatewayAllowedHosts([]string{"adm.example.com"}); err != nil {
		t.Fatal(err)
	}
	status, err := service.GatewayAccessStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.WhitelistEnabled {
		t.Fatal("legacy setter must preserve Host restriction")
	}
}
