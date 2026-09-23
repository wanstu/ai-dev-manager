package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestWebManagementFirstAdminRequiresLoopbackAndCreatesSession(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	handler := newWebManagementHandler(service, nil)

	statusReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:43137/api/web/auth/status", nil)
	statusReq.RemoteAddr = "127.0.0.1:55000"
	statusReq.Host = "127.0.0.1:43137"
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var status webAuthResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Initialized || !status.RegistrationAllowed {
		t.Fatalf("unexpected initial status: %+v", status)
	}

	registerReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43137/api/web/auth/register", strings.NewReader(`{"username":"admin","password":"correct-horse-battery"}`))
	registerReq.RemoteAddr = "127.0.0.1:55000"
	registerReq.Host = "127.0.0.1:43137"
	registerReq.Header.Set("Content-Type", "application/json")
	registerReq.Header.Set("X-ADM-Web", "1")
	registerRec := httptest.NewRecorder()
	handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("register code=%d body=%s", registerRec.Code, registerRec.Body.String())
	}
	cookies := registerRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected session cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != webSessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}

	manageReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43137/api/web/manage", strings.NewReader(`{"method":"GetSnapshot"}`))
	manageReq.RemoteAddr = "127.0.0.1:55000"
	manageReq.Host = "127.0.0.1:43137"
	manageReq.Header.Set("Content-Type", "application/json")
	manageReq.Header.Set("X-ADM-Web", "1")
	manageReq.AddCookie(cookie)
	manageRec := httptest.NewRecorder()
	handler.ServeHTTP(manageRec, manageReq)
	if manageRec.Code != http.StatusOK {
		t.Fatalf("manage code=%d body=%s", manageRec.Code, manageRec.Body.String())
	}
	if !strings.Contains(manageRec.Body.String(), `"result"`) {
		t.Fatalf("management response missing result: %s", manageRec.Body.String())
	}

	setKeyReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43137/api/web/manage", strings.NewReader(`{"method":"ConfigureGatewayAdminAPIKey","args":["1234567890abcdef"]}`))
	setKeyReq.RemoteAddr = "127.0.0.1:55000"
	setKeyReq.Host = "127.0.0.1:43137"
	setKeyReq.Header.Set("Content-Type", "application/json")
	setKeyReq.Header.Set("X-ADM-Web", "1")
	setKeyReq.AddCookie(cookie)
	setKeyRec := httptest.NewRecorder()
	handler.ServeHTTP(setKeyRec, setKeyReq)
	if setKeyRec.Code != http.StatusOK {
		t.Fatalf("configure admin key code=%d body=%s", setKeyRec.Code, setKeyRec.Body.String())
	}
	accessStatus, err := service.GatewayAccessStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !accessStatus.AdminAPIKeyConfigured {
		t.Fatal("ConfigureGatewayAdminAPIKey through Web did not persist the Admin API key")
	}
}

func TestWebManagementRejectsRemoteFirstAdminRegistration(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	handler := newWebManagementHandler(service, nil)

	req := httptest.NewRequest(http.MethodPost, "http://adm.example.com/api/web/auth/register", strings.NewReader(`{"username":"admin","password":"correct-horse-battery"}`))
	req.RemoteAddr = "203.0.113.10:55000"
	req.Host = "adm.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ADM-Web", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("remote registration code=%d body=%s", rec.Code, rec.Body.String())
	}

	status, err := service.WebAdminStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized {
		t.Fatal("remote first registration must not initialize an admin")
	}
}

func TestWebManagementLoginAndLogout(t *testing.T) {
	service := app.New(filepath.Join(t.TempDir(), "state.json"))
	if _, err := service.InitializeWebAdmin("admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	handler := newWebManagementHandler(service, nil)

	loginReq := httptest.NewRequest(http.MethodPost, "http://adm.example.com/api/web/auth/login", strings.NewReader(`{"username":"admin","password":"correct-horse-battery"}`))
	loginReq.RemoteAddr = "203.0.113.10:55000"
	loginReq.Host = "adm.example.com"
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-ADM-Web", "1")
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected login cookie, got %d", len(cookies))
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "http://adm.example.com/api/web/auth/logout", nil)
	logoutReq.RemoteAddr = "203.0.113.10:55000"
	logoutReq.Host = "adm.example.com"
	logoutReq.Header.Set("X-ADM-Web", "1")
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout code=%d body=%s", logoutRec.Code, logoutRec.Body.String())
	}

	manageReq := httptest.NewRequest(http.MethodPost, "http://adm.example.com/api/web/manage", strings.NewReader(`{"method":"GetSnapshot"}`))
	manageReq.RemoteAddr = "203.0.113.10:55000"
	manageReq.Host = "adm.example.com"
	manageReq.Header.Set("Content-Type", "application/json")
	manageReq.Header.Set("X-ADM-Web", "1")
	manageReq.AddCookie(cookies[0])
	manageRec := httptest.NewRecorder()
	handler.ServeHTTP(manageRec, manageReq)
	if manageRec.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out session code=%d body=%s", manageRec.Code, manageRec.Body.String())
	}
}
