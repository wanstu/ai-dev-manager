package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"ai-dev-manager-v2/internal/app"
)

func TestGatewayStartLegacyListenCanOnlySelectPort(t *testing.T) {
	svc := app.New(filepath.Join(t.TempDir(), "state.json"))
	original := startGatewayDetached
	t.Cleanup(func() { startGatewayDetached = original })
	var bind string
	startGatewayDetached = func(addr string) error { bind = addr; return nil }
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"legacy localhost", []string{"start", "--detach", "--listen", "127.0.0.1:45555"}, "0.0.0.0:45555"},
		{"legacy private IP", []string{"start", "--detach", "--listen", "192.168.1.7:42222"}, "0.0.0.0:42222"},
		{"legacy bare port", []string{"start", "--detach", "--listen", ":43333"}, "0.0.0.0:43333"},
		{"port only", []string{"start", "--detach", "--port", "44444"}, "0.0.0.0:44444"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bind = ""
			if err := runGateway(svc, tc.args); err != nil {
				t.Fatal(err)
			}
			if bind != tc.want {
				t.Fatalf("bind=%q want %q", bind, tc.want)
			}
		})
	}
	if err := runGateway(svc, []string{"start", "--detach", "--listen", "localhost:45555", "--port", "45555"}); err == nil {
		t.Fatal("--listen and --port should not be mixed")
	}
	if err := runGateway(svc, []string{"start", "--detach", "--listen", "not-a-valid-port"}); err == nil {
		t.Fatal("invalid legacy port must fail")
	}
}

func TestGatewayMainHelpExplainsActualBindAndOptionalWhitelist(t *testing.T) {
	help := captureStdout(t, func() {
		if err := Run([]string{"--help"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, text := range []string{"0.0.0.0", "--port", "白名单默认关闭"} {
		if !strings.Contains(help, text) {
			t.Errorf("main CLI help missing %q", text)
		}
	}
	if strings.Contains(help, "未写 --listen 时") {
		t.Fatalf("outdated user-facing bind-IP help remains: %s", help)
	}
}
