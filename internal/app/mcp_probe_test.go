package app

import (
	"ai-dev-manager-v2/internal/catalog"
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGlobalMCPProbeWithoutEnvironment(t *testing.T) {
	external := mcp.NewServer(&mcp.Implementation{Name: "global-probe-fixture", Version: "dev"}, nil)
	server := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return external }, &mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	s := New(statePath)
	entry, err := s.MCPs.AddMCP("global", server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(statePath)
	result, err := s.MCPProbe(context.Background(), entry.ID)
	if err != nil || result.State != MCPHealthHealthy {
		t.Fatalf("probe=%+v err=%v", result, err)
	}
	after, _ := os.ReadFile(statePath)
	if string(before) != string(after) {
		t.Fatal("probe changed desired state")
	}
	ws, err := s.Workspaces.Add(t.TempDir(), "plain")
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.Environments.Create(ws.ID, "plain", "")
	if err != nil {
		t.Fatal(err)
	}
	activation, status, err := s.ResolveMCPActivation(env.ID, entry.ID)
	if err != nil || activation != nil || status.State != MCPHealthDisabled {
		t.Fatalf("global probe granted Agent access: %+v %v", status, err)
	}
	if _, err := s.MCPProbe(context.Background(), "missing"); err == nil {
		t.Fatal("missing ID accepted")
	}
}

func TestGlobalMCPProbeFailuresAreSanitized(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "state.json"))
	t.Setenv("ADM_GLOBAL_PROBE_SECRET", "global-probe-private-sentinel")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	entry, err := s.MCPs.AddMCPConfig("auth", catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, Endpoint: server.URL + "/?token=${ADM_GLOBAL_PROBE_SECRET}"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.MCPProbe(context.Background(), entry.ID)
	if err != nil || result.ErrorKind != "auth_failure" || strings.Contains(result.Message, "global-probe-private-sentinel") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	entry, err = s.MCPs.AddMCPConfig("missing-ref", catalog.MCPConfig{Transport: catalog.MCPTransportStreamableHTTP, Endpoint: "http://${ADM_MISSING_GLOBAL_PROBE_REF_67335}/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	result, err = s.MCPProbe(context.Background(), entry.ID)
	if err != nil || result.ErrorKind != "unresolved_secret_reference" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer slow.Close()
	entry, err = s.MCPs.AddMCP("timeout", slow.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result, err = s.MCPProbe(ctx, entry.ID)
	if err != nil || result.ErrorKind != "timeout" {
		t.Fatalf("timeout result=%+v err=%v", result, err)
	}
}

func TestGlobalMCPProbeStdioHelper(t *testing.T) {
	if os.Getenv("ADM_GLOBAL_STDIO_HELPER") != "1" {
		return
	}
	root, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	if !strings.HasPrefix(filepath.Base(root), "adm-mcp-probe-") {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv("ADM_GLOBAL_STDIO_CWD_FILE"), []byte(root), 0600); err != nil {
		os.Exit(4)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "global-stdio", Version: "dev"}, nil)
	_ = server.Run(context.Background(), &mcp.StdioTransport{})
	os.Exit(0)
}

func TestGlobalMCPProbeStdioAllowlistAndCleanup(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "state.json"))
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cwdFile := filepath.Join(t.TempDir(), "cwd.txt")
	t.Setenv("ADM_GLOBAL_STDIO_HELPER_VALUE", "1")
	t.Setenv("ADM_GLOBAL_STDIO_CWD_PATH", cwdFile)
	entry, err := s.MCPs.AddMCPConfig("stdio", catalog.MCPConfig{Transport: catalog.MCPTransportStdio, Executable: exe, Args: []string{"-test.run=^TestGlobalMCPProbeStdioHelper$"}, EnvRefs: map[string]string{"ADM_GLOBAL_STDIO_HELPER": "${ADM_GLOBAL_STDIO_HELPER_VALUE}", "ADM_GLOBAL_STDIO_CWD_FILE": "${ADM_GLOBAL_STDIO_CWD_PATH}"}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.MCPProbe(context.Background(), entry.ID)
	if err != nil || result.ErrorKind != "executable_not_allowed" {
		t.Fatalf("denial=%+v err=%v", result, err)
	}
	denials, err := s.ExecDenials()
	if err != nil || len(denials) != 1 {
		t.Fatalf("denials=%+v err=%v", denials, err)
	}
	if err := s.AllowExecutable(exe); err != nil {
		t.Fatal(err)
	}
	result, err = s.MCPProbe(context.Background(), entry.ID)
	if err != nil || result.State != MCPHealthHealthy {
		t.Fatalf("stdio=%+v err=%v", result, err)
	}
	cwd, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(cwd)); !os.IsNotExist(err) {
		t.Fatalf("temporary probe directory remains: %v", err)
	}
	envs, err := s.Environments.List()
	if err != nil || len(envs) != 0 {
		t.Fatal("probe created an Environment")
	}
}
