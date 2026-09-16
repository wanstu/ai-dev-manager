package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestInspectHTTPReportsRunningCompatibleGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":%q,"version":"test","status":"ok","pid":%d,"transport":"http"}`, serverName, os.Getpid())
	}))
	defer server.Close()

	listen := strings.TrimPrefix(server.URL, "http://")
	status, err := InspectHTTP(listen)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != HTTPStateRunning || status.PID != os.Getpid() || status.Version != "test" {
		t.Fatalf("status = %+v", status)
	}
	if status.MCPURL != server.URL+"/mcp" {
		t.Fatalf("mcp_url=%q want %q", status.MCPURL, server.URL+"/mcp")
	}
}

func TestHTTPHealthExposesStableRuntimeOwner(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	owner := newRuntimeOwner(service)
	defer owner.Close()
	server := httptest.NewServer(newHTTPHandler(service, owner))
	defer server.Close()

	for i := 0; i < 2; i++ {
		response, err := http.Get(server.URL + "/healthz")
		if err != nil {
			t.Fatal(err)
		}
		var health HTTPHealth
		decodeErr := json.NewDecoder(response.Body).Decode(&health)
		_ = response.Body.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if health.OwnerID == "" || health.OwnerID != owner.Info().ID {
			t.Fatalf("health owner_id=%q want %q", health.OwnerID, owner.Info().ID)
		}
	}
	listen := strings.TrimPrefix(server.URL, "http://")
	status, err := InspectHTTP(listen)
	if err != nil {
		t.Fatal(err)
	}
	if status.OwnerID != owner.Info().ID {
		t.Fatalf("InspectHTTP owner_id=%q want %q", status.OwnerID, owner.Info().ID)
	}
}

func TestInspectHTTPDistinguishesStoppedAndIncompatibleEndpoints(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listen := listener.Addr().String()
	_ = listener.Close()
	stopped, err := InspectHTTP(listen)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != HTTPStateStopped {
		t.Fatalf("stopped status = %+v", stopped)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()
	incompatibleListen := strings.TrimPrefix(server.URL, "http://")
	incompatible, err := InspectHTTP(incompatibleListen)
	if err != nil {
		t.Fatal(err)
	}
	if incompatible.State != HTTPStateIncompatible || incompatible.Detail == "" {
		t.Fatalf("incompatible status = %+v", incompatible)
	}
	if _, err := StopHTTP(incompatibleListen); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("StopHTTP must refuse incompatible endpoint, got %v", err)
	}
}

func TestStopHTTPUsesOwnerBoundGracefulShutdown(t *testing.T) {
	service, environmentID, mcpID := runtimeOwnerTestService(t)
	owner := newRuntimeOwner(service)
	fake := &fakeOwnedMCPSession{}
	owner.connect = func(context.Context, string, string, map[string]string) (ownedMCPSession, error) {
		return fake, nil
	}
	if status, err := owner.Status(context.Background(), environmentID, mcpID); err != nil || status.State != app.MCPHealthHealthy {
		t.Fatalf("owner activation status=%+v err=%v", status, err)
	}

	var server *httptest.Server
	server = httptest.NewServer(newHTTPHandlerWithShutdown(service, owner, func() {
		_ = owner.Close()
		server.Close()
	}))
	listen := strings.TrimPrefix(server.URL, "http://")

	wrong, err := http.NewRequest(http.MethodPost, server.URL+"/shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}
	wrong.Header.Set(runtimeOwnerHeader, "owner_wrong")
	response, err := http.DefaultClient.Do(wrong)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("wrong owner shutdown status=%s", response.Status)
	}
	if status, err := InspectHTTP(listen); err != nil || status.State != HTTPStateRunning {
		t.Fatalf("wrong owner stopped Gateway: status=%+v err=%v", status, err)
	}

	stopped, err := StopHTTP(listen)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != HTTPStateStopped {
		t.Fatalf("StopHTTP result=%+v", stopped)
	}
	_, _, closes := fake.counts()
	if closes != 1 {
		t.Fatalf("graceful shutdown closed owned session %d times, want 1", closes)
	}
}

func TestInspectHTTPBaseURLSupportsBasePathAndAdminEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":%q,"version":"test","status":"ok","pid":%d,"transport":"http"}`, serverName, os.Getpid())
	}))
	defer server.Close()

	status, err := InspectHTTPBaseURL(server.URL + "/control/")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != HTTPStateRunning || status.BaseURL != server.URL+"/control" {
		t.Fatalf("status=%+v", status)
	}
	if status.MCPURL != server.URL+"/control/mcp" || status.AdminMCPURL != server.URL+"/control/admin/mcp" {
		t.Fatalf("derived MCP URLs=%+v", status)
	}
}

func TestInspectHTTPBaseURLRejectsInvalidProfiles(t *testing.T) {
	for _, raw := range []string{
		"",
		"ftp://example.test:21",
		"http://",
		"http://user:pass@example.test:41137",
		"http://example.test:41137?x=1",
		"http://example.test:41137/#fragment",
	} {
		if _, err := InspectHTTPBaseURL(raw); err == nil {
			t.Fatalf("InspectHTTPBaseURL(%q) unexpectedly succeeded", raw)
		}
	}
}
func TestHTTPBaseURLRejectsInvalidListen(t *testing.T) {
	if _, err := HTTPBaseURL("not-an-endpoint"); err == nil {
		t.Fatal("invalid listen must fail")
	}
	got, err := HTTPBaseURL("127.0.0.1:41137")
	if err != nil || got != "http://127.0.0.1:41137" {
		t.Fatalf("baseURL=%q err=%v", got, err)
	}
}

func TestHTTPConnectionFailuresIncludeShutdownRaces(t *testing.T) {
	for _, message := range []string{
		"connectex: No connection could be made because the target machine actively refused it.",
		"read tcp 127.0.0.1:55193->127.0.0.1:55189: wsarecv: An existing connection was forcibly closed by the remote host.",
		"read: connection reset by peer",
		"use of closed network connection",
	} {
		if !isHTTPConnectionFailure(errors.New(message)) {
			t.Fatalf("shutdown race connection error was not classified as stopped: %s", message)
		}
	}
	if isHTTPConnectionFailure(errors.New("invalid ADM V2 health JSON")) {
		t.Fatal("protocol errors must not be classified as stopped endpoints")
	}
}

func TestCheckHTTPListenAvailableRejectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if err := CheckHTTPListenAvailable(listener.Addr().String()); err == nil || !strings.Contains(err.Error(), "cannot bind") {
		t.Fatalf("occupied listen address should fail clearly, got %v", err)
	}
}

func TestResolveHTTPTargetDerivesCanonicalEndpointsAndLocalLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		base   string
		health string
		agent  string
		admin  string
		listen string
		local  bool
	}{
		{name: "ipv4 local", raw: " http://127.0.0.1:43137/ ", base: "http://127.0.0.1:43137", health: "http://127.0.0.1:43137/healthz", agent: "http://127.0.0.1:43137/mcp", admin: "http://127.0.0.1:43137/admin/mcp", listen: "127.0.0.1:43137", local: true},
		{name: "localhost local", raw: "http://localhost:48001", base: "http://localhost:48001", health: "http://localhost:48001/healthz", agent: "http://localhost:48001/mcp", admin: "http://localhost:48001/admin/mcp", listen: "localhost:48001", local: true},
		{name: "ipv6 local", raw: "http://[::1]:48002/", base: "http://[::1]:48002", health: "http://[::1]:48002/healthz", agent: "http://[::1]:48002/mcp", admin: "http://[::1]:48002/admin/mcp", listen: "[::1]:48002", local: true},
		{name: "https loopback inspection only", raw: "https://127.0.0.1:443/", base: "https://127.0.0.1:443", health: "https://127.0.0.1:443/healthz", agent: "https://127.0.0.1:443/mcp", admin: "https://127.0.0.1:443/admin/mcp"},
		{name: "remote base path inspection only", raw: "https://adm.example.test:8443/control/", base: "https://adm.example.test:8443/control", health: "https://adm.example.test:8443/control/healthz", agent: "https://adm.example.test:8443/control/mcp", admin: "https://adm.example.test:8443/control/admin/mcp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ResolveHTTPTarget(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if target.BaseURL != tt.base || target.HealthURL != tt.health || target.MCPURL != tt.agent || target.AdminMCPURL != tt.admin {
				t.Fatalf("target=%+v", target)
			}
			if got := LocalHTTPLifecycleEligible(tt.raw); got != tt.local {
				t.Fatalf("LocalHTTPLifecycleEligible(%q)=%v want %v", tt.raw, got, tt.local)
			}
			listen, listenErr := LocalHTTPListenFromBaseURL(tt.raw)
			if tt.local {
				if listenErr != nil || listen != tt.listen {
					t.Fatalf("LocalHTTPListenFromBaseURL(%q)=%q err=%v want %q", tt.raw, listen, listenErr, tt.listen)
				}
			} else if listenErr == nil {
				t.Fatalf("LocalHTTPListenFromBaseURL(%q)=%q, want error", tt.raw, listen)
			}
		})
	}
}

func TestResolveHTTPTargetRejectsAuthorityAndSuffixAmbiguity(t *testing.T) {
	for _, raw := range []string{
		"",
		"ftp://adm.example.test",
		"http://",
		"http://user:pass@adm.example.test:43137",
		"http://adm.example.test:43137?token=x",
		"http://adm.example.test:43137?",
		"http://adm.example.test:43137/#fragment",
	} {
		if _, err := ResolveHTTPTarget(raw); err == nil {
			t.Fatalf("ResolveHTTPTarget(%q) unexpectedly succeeded", raw)
		}
	}
}
