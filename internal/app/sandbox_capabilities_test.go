package app

import (
	"path/filepath"
	"reflect"
	"testing"

	"ai-dev-manager-v2/internal/sandbox"
)

func TestSandboxCapabilitiesForCommandOnlyAllowsGoModDownload(t *testing.T) {
	goExe := filepath.Join(t.TempDir(), "go")
	nodeExe := filepath.Join(t.TempDir(), "node")
	tests := []struct {
		name string
		exe  string
		args []string
		want []string
	}{
		{name: "go mod download", exe: goExe, args: []string{"mod", "download"}, want: []string{sandbox.CapabilityInternetClient}},
		{name: "go mod download module", exe: goExe, args: []string{"mod", "download", "golang.org/x/sys@v0.48.0"}, want: []string{sandbox.CapabilityInternetClient}},
		{name: "go test", exe: goExe, args: []string{"test", "./..."}},
		{name: "go run", exe: goExe, args: []string{"run", "./cmd/tool"}},
		{name: "go mod tidy", exe: goExe, args: []string{"mod", "tidy"}},
		{name: "other executable", exe: nodeExe, args: []string{"mod", "download"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sandboxCapabilitiesForCommand(tt.exe, tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("capabilities=%v want=%v", got, tt.want)
			}
		})
	}
}
