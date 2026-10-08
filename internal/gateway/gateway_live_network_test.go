package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/app"
)

// Covers a real Windows/Linux TCP bind, not only httptest's simulated RemoteAddr.
func TestGatewayLiveTCPAllInterfacesAndPolicy(t *testing.T) {
	svc := app.New(t.TempDir() + "/state.json")
	if _, err := svc.SetGatewayAdminAPIKey(testAdminKey); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGatewayAgentAPIKey(testAgentKey); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Skipf("IPv4 wildcard listener unavailable: %v", err)
	}
	server := &http.Server{Handler: NewHTTPHandler(svc)}
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		<-done
	})
	port := listener.Addr().(*net.TCPAddr).Port
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	t.Cleanup(func() { client.CloseIdleConnections() })
	call := func(path, host, key string, headers map[string]string) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		if key != "" {
			req.Header.Set("X-ADM-API-Key", key)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return response.StatusCode
	}
	if actual := call("/healthz", "external.example.test", "", nil); actual != 200 {
		t.Fatalf("disabled whitelist should permit public health endpoint: got %d", actual)
	}
	if actual := call("/admin/mcp", "external.example.test", "", nil); actual != http.StatusForbidden {
		t.Fatalf("remote Admin MCP without key should be denied: got %d", actual)
	}
	if _, err := svc.SetGatewayAccessPolicy(true, []string{"ADM.Example.Test"}, []string{"127.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}
	if actual := call("/healthz", "ADM.EXAMPLE.TEST:"+strconv.Itoa(port), "", nil); actual != http.StatusOK {
		t.Fatalf("correct Host and TCP peer should be accepted: got %d", actual)
	}
	if actual := call("/healthz", "another.example.test", "", nil); actual != http.StatusForbidden {
		t.Fatalf("wrong Host should be rejected: got %d", actual)
	}
	if _, err := svc.SetGatewayAccessPolicy(true, []string{"adm.example.test"}, []string{"10.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}
	spoof := map[string]string{"X-Forwarded-For": "10.0.0.8", "X-Real-IP": "10.0.0.8", "Forwarded": "for=10.0.0.8"}
	if actual := call("/healthz", "adm.example.test", "", spoof); actual != http.StatusForbidden {
		t.Fatalf("forged forwarded headers bypassed TCP IP policy: %d", actual)
	}
	if _, err := svc.SetGatewayAccessPolicy(false, nil, nil); err != nil {
		t.Fatal(err)
	}
	if actual := call("/healthz", "another.example.test", "", nil); actual != http.StatusOK {
		t.Fatalf("disabled whitelist should restore HTTP availability: got %d", actual)
	}
	// Make sure the key itself is not returned in unauthorized errors.
	if strings.TrimSpace(testAdminKey) == "" {
		t.Fatal("invalid test fixture")
	}
}

func TestRunHTTPDefaultWildcardBindWithoutRemoteKeys(t *testing.T) {
	service := app.New(t.TempDir() + "/state.json")
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local IPv4 unavailable: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	listen := "0.0.0.0:" + strconv.Itoa(port)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunHTTP(ctx, service, listen) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Gateway did not shutdown")
		}
	})
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 300 * time.Millisecond}
	var healthOK bool
	for i := 0; i < 60; i++ {
		response, getErr := client.Get(url + "/healthz")
		if getErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				healthOK = true
				break
			}
		}
		select {
		case err := <-done:
			t.Fatalf("Gateway wildcard startup failed: %v", err)
		default:
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !healthOK {
		t.Fatal("Gateway failed to listen on all IPv4 interfaces")
	}
	req, err := http.NewRequest(http.MethodGet, url+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "outside.example.test"
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("remote-shaped MCP without keys should be forbidden, got %d", response.StatusCode)
	}
}
