package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCLIAdminClientRejectsIncompatibleGatewayBeforeAdminMCP(t *testing.T) {
	adminCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"adm","version":"v1.3.0","status":"ok","pid":1234,"transport":"http","owner_id":"owner_old"}`)
		case "/admin/mcp":
			adminCalls++
			http.Error(w, "should not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newCLIAdminClient(server.URL)
	if err != nil {
		t.Fatalf("construct admin client: %v", err)
	}
	if client == nil {
		t.Fatal("expected admin client")
	}
	_, err = client.GatewayDiagnostics()
	if err == nil {
		t.Fatal("incompatible Gateway management unexpectedly succeeded")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "incompatible") ||
		!strings.Contains(err.Error(), "management API 0") {
		t.Fatalf("compatibility error=%v", err)
	}
	if adminCalls != 0 {
		t.Fatalf("Admin MCP was called before compatibility rejection: %d", adminCalls)
	}
}
