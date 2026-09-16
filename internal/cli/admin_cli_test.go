package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/adminmcp"
	"ai-dev-manager-v2/internal/app"
	"ai-dev-manager-v2/internal/gateway"
	"ai-dev-manager-v2/internal/pathutil"
)

var _ cliManagementBackend = (*adminmcp.Client)(nil)

func startCLIAdminTestServer(t *testing.T, home string) *app.Service {
	t.Helper()
	service := app.New(filepath.Join(home, "state.json"))
	server := httptest.NewServer(gateway.NewHTTPHandler(service))
	t.Cleanup(server.Close)
	t.Setenv(admBaseURLEnv, server.URL)
	return service
}

func TestTopLevelCLIManagementUsesAdminMCPWithoutLocalStateFallback(t *testing.T) {
	localHome := t.TempDir()
	t.Setenv("ADM_V2_HOME", localHome)

	remoteState := filepath.Join(t.TempDir(), "remote-state.json")
	remote := app.New(remoteState)
	server := httptest.NewServer(gateway.NewHTTPHandler(remote))
	defer server.Close()
	t.Setenv(admBaseURLEnv, server.URL)

	root := t.TempDir()
	captureStdout(t, func() {
		if err := run([]string{"workspace", "add", "--path", root, "--name", "remote-only"}); err != nil {
			t.Fatal(err)
		}
	})
	items, err := remote.Workspaces.List()
	if err != nil || len(items) != 1 || !pathutil.Same(items[0].Path, root) {
		t.Fatalf("remote Admin MCP workspace state=%+v err=%v", items, err)
	}
	if _, err := os.Stat(filepath.Join(localHome, "state.json")); !os.IsNotExist(err) {
		t.Fatalf("normal CLI management unexpectedly touched local state.json: %v", err)
	}
}

func TestTopLevelCLIManagementDoesNotFallbackWhenAdminMCPStops(t *testing.T) {
	localHome := t.TempDir()
	t.Setenv("ADM_V2_HOME", localHome)
	local := app.New(filepath.Join(localHome, "state.json"))
	if _, err := local.Workspaces.Add(t.TempDir(), "local-only"); err != nil {
		t.Fatal(err)
	}

	remote := app.New(filepath.Join(t.TempDir(), "remote-state.json"))
	server := httptest.NewServer(gateway.NewHTTPHandler(remote))
	baseURL := server.URL
	server.Close()
	t.Setenv(admBaseURLEnv, baseURL)

	err := run([]string{"workspace", "list"})
	if err == nil || !strings.Contains(err.Error(), "Admin MCP") {
		t.Fatalf("stopped Admin MCP should fail without local fallback, got %v", err)
	}
}

func TestCLIADMURLFlagOverridesEnvironmentAndSupportsBasePath(t *testing.T) {
	remote := app.New(filepath.Join(t.TempDir(), "remote-state.json"))
	server := httptest.NewServer(http.StripPrefix("/control", gateway.NewHTTPHandler(remote)))
	defer server.Close()
	t.Setenv(admBaseURLEnv, "http://127.0.0.1:1")

	baseURL := server.URL + "/control/"
	output := captureStdout(t, func() {
		if err := run([]string{"--adm-url", baseURL, "workspace", "list"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(output) != "[]" {
		t.Fatalf("workspace list output=%q", output)
	}

	target, err := gateway.ResolveHTTPTarget(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := normalizeCLIADMBaseURL(baseURL); err != nil || got != target.BaseURL {
		t.Fatalf("normalized base URL=%q err=%v target=%+v", got, err, target)
	}
	if target.AdminMCPURL != server.URL+"/control/admin/mcp" {
		t.Fatalf("Admin MCP endpoint=%q", target.AdminMCPURL)
	}
}

func TestCLITargetSemanticsMatchGatewayTarget(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:43137/",
		"http://localhost:48001",
		"http://[::1]:48002/",
		"https://127.0.0.1:443/",
		"https://adm.example.test:8443/control/",
	} {
		target, err := gateway.ResolveHTTPTarget(raw)
		if err != nil {
			t.Fatalf("ResolveHTTPTarget(%q): %v", raw, err)
		}
		if got, err := normalizeCLIADMBaseURL(raw); err != nil || got != target.BaseURL {
			t.Fatalf("normalizeCLIADMBaseURL(%q)=%q err=%v want %q", raw, got, err, target.BaseURL)
		}
		listen, cliErr := localGatewayListenFromBaseURL(raw)
		sharedListen, sharedErr := gateway.LocalHTTPListenFromBaseURL(raw)
		if (cliErr == nil) != (sharedErr == nil) || listen != sharedListen {
			t.Fatalf("local decision mismatch for %q: cli=%q/%v shared=%q/%v", raw, listen, cliErr, sharedListen, sharedErr)
		}
	}
}

func TestCLIManagementHelpDoesNotRequireRunningADM(t *testing.T) {
	t.Setenv(admBaseURLEnv, "http://127.0.0.1:1")
	output := captureStdout(t, func() {
		if err := run([]string{"workspace", "-h"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "workspace add --path") {
		t.Fatalf("workspace help missing expected usage: %s", output)
	}
}
