package gateway

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayWebManagementHonorsOptionalNetworkPolicy(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	handler := NewHTTPHandler(service)
	req := httptest.NewRequest(http.MethodGet, "http://blocked.example.com/api/web/auth/status", nil)
	req.Host = "blocked.example.com:8001"
	req.RemoteAddr = "198.51.100.5:50123"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("default disabled whitelist: %d", res.Code)
	}
	if _, err := service.SetGatewayAccessPolicy(true, []string{"adm.example.com"}, []string{"10.1.0.0/16"}); err != nil {
		t.Fatal(err)
	}
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("unlisted web origin: got %d", res.Code)
	}
	req.Host = "adm.example.com:8001"
	req.RemoteAddr = "10.1.2.3:50123"
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("listed web origin: got %d", res.Code)
	}
}
