package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
)

// Test against the actual host network adapter rather than 127.0.0.1.
// Some CI hosts prevent even self-connects through a nonloopback interface;
// in those environments the test records an explicit skip.
func TestGatewayLiveHostInterfaceAllowlist(t *testing.T) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("network interfaces unavailable: %v", err)
	}
	var candidates []net.IP
	for _, addr := range addresses {
		network, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		candidate := network.IP.To4()
		if candidate != nil && !candidate.IsLoopback() && !candidate.IsUnspecified() && !candidate.IsLinkLocalUnicast() {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		t.Skip("no nonloopback IPv4 adapter on test host")
	}
	service := app.New(t.TempDir() + "/state.json")
	if _, err := service.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Skipf("IPv4 interface binding unavailable: %v", err)
	}
	server := &http.Server{Handler: NewHTTPHandler(service)}
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		_ = server.Shutdown(ctx)
		<-done
	})
	port := listener.Addr().(*net.TCPAddr).Port
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Timeout: 600 * time.Millisecond, Transport: transport}
	url := ""
	call := func(path, host string, headers map[string]string) (int, error) {
		req, err := http.NewRequest(http.MethodGet, url+path, nil)
		if err != nil {
			return 0, err
		}
		req.Host = host
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	var ip net.IP
	var status int
	for _, candidate := range candidates {
		url = "http://" + net.JoinHostPort(candidate.String(), strconv.Itoa(port))
		status, err = call("/healthz", "adm.example.test", nil)
		if err != nil {
			t.Logf("adapter %s self-connect unavailable: %v", candidate, err)
			continue
		}
		if status != http.StatusOK {
			t.Fatalf("default network access via %s status=%d", candidate, status)
		}
		ip = candidate
		break
	}
	if ip == nil {
		t.Skip("none of the nonloopback interfaces accepts host self-connections (network/firewall)")
	}
	t.Logf("nonloopback host interface accepted TCP traffic: %s:%d", ip, port)
	if status, err = call("/admin/mcp", "adm.example.test", nil); err != nil || status != http.StatusForbidden {
		t.Fatalf("remote MCP authentication missing: status=%d err=%v", status, err)
	}
	if _, err = service.SetGatewayAccessPolicy(true, []string{"adm.example.test"}, []string{ip.String()}); err != nil {
		t.Fatal(err)
	}
	if status, err = call("/healthz", "adm.example.test", nil); err != nil || status != http.StatusOK {
		t.Fatalf("allowlisted source and host: %d %v", status, err)
	}
	if status, err = call("/healthz", "other.example.test", nil); err != nil || status != http.StatusForbidden {
		t.Fatalf("unlisted Host: %d %v", status, err)
	}
	wrong := netip.MustParseAddr("203.0.113.22").String()
	if _, err = service.SetGatewayAccessPolicy(true, []string{"adm.example.test"}, []string{wrong}); err != nil {
		t.Fatal(err)
	}
	fake := map[string]string{"X-Forwarded-For": wrong, "X-Real-IP": wrong, "Forwarded": "for=" + wrong}
	if status, err = call("/healthz", "adm.example.test", fake); err != nil || status != http.StatusForbidden {
		t.Fatalf("spoofed forwarded headers bypassed allowlist: %d %v", status, err)
	}
}
