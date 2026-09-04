package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ai-dev-manager/internal/model"
)

func TestNewNativeDefaultsToReadOnlyAndSnapshotsPolicy(t *testing.T) {
	root := t.TempDir()
	readOnly, err := NewNative(model.Workspace{ID: "ws_a", Path: root}, model.EffectiveConfig{})
	if err != nil {
		t.Fatalf("NewNative(read-only) error = %v", err)
	}
	if readOnly.Mode() != ModeReadOnly {
		t.Fatalf("default mode = %q, want read-only", readOnly.Mode())
	}
	wantReadOnlyCaps := []Capability{CapabilityRead, CapabilityTree, CapabilitySearch}
	if got := readOnly.Capabilities(); !reflect.DeepEqual(got, wantReadOnlyCaps) {
		t.Fatalf("read-only capabilities = %+v, want %+v", got, wantReadOnlyCaps)
	}

	policy := model.Policy{
		Mode:               string(ModeStandard),
		AllowedExecutables: []string{"helper"},
		ToolPaths:          map[string]string{"helper": `C:\example\helper.exe`},
	}
	cfg := model.EffectiveConfig{Policy: &model.ResolvedPolicy{Policy: policy, Source: model.ScopeProject}}
	standard, err := NewNative(model.Workspace{ID: "ws_b", Path: root}, cfg)
	if err != nil {
		t.Fatalf("NewNative(standard) error = %v", err)
	}
	cfg.Policy.Policy.Mode = string(ModeFull)
	cfg.Policy.Policy.AllowedExecutables[0] = "changed"
	cfg.Policy.Policy.ToolPaths["helper"] = "changed"

	if standard.Mode() != ModeStandard || standard.policy.AllowedExecutables[0] != "helper" || standard.policy.ToolPaths["helper"] != `C:\example\helper.exe` {
		t.Fatalf("runtime policy aliases caller config: %+v", standard.policy)
	}
}

func TestNewNativeRejectsInvalidPolicy(t *testing.T) {
	_, err := NewNative(model.Workspace{ID: "ws", Path: t.TempDir()}, model.EffectiveConfig{
		Policy: &model.ResolvedPolicy{Policy: model.Policy{Mode: "unknown"}},
	})
	assertRuntimeErrorKind(t, err, ErrInvalidPolicy)
}

func TestNativeWorkspaceContainmentAndBlockedWrites(t *testing.T) {
	parent := t.TempDir()
	rootA := filepath.Join(parent, "workspace-a")
	rootB := filepath.Join(parent, "workspace-b")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatalf("MkdirAll A error = %v", err)
	}
	if err := os.MkdirAll(rootB, 0o755); err != nil {
		t.Fatalf("MkdirAll B error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootA, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatalf("WriteFile A error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "b.txt"), []byte("B"), 0o644); err != nil {
		t.Fatalf("WriteFile B error = %v", err)
	}

	runtimeA := mustNative(t, rootA, model.Policy{Mode: string(ModeWorkspaceWrite)})
	if _, _, err := runtimeA.Read("a.txt", 0); err != nil {
		t.Fatalf("Read in workspace A error = %v", err)
	}
	_, _, err := runtimeA.Read(filepath.Join(rootB, "b.txt"), 0)
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
	_, _, err = runtimeA.Read(filepath.Join("..", "workspace-b", "b.txt"), 0)
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)

	_, err = runtimeA.Write(filepath.Join(rootB, "new.txt"), []byte("x"), false)
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
	_, err = runtimeA.Write(filepath.Join(".git", "config"), []byte("x"), true)
	assertRuntimeErrorKind(t, err, ErrPathBlocked)
	_, err = runtimeA.Write(filepath.Join(".ai-dev-manager", "runtime", "state.json"), []byte("x"), true)
	assertRuntimeErrorKind(t, err, ErrPathBlocked)
}

func TestTwoNativeRuntimesKeepIndependentRootsAndPolicies(t *testing.T) {
	parent := t.TempDir()
	rootA := filepath.Join(parent, "a")
	rootB := filepath.Join(parent, "b")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatalf("MkdirAll A error = %v", err)
	}
	if err := os.MkdirAll(rootB, 0o755); err != nil {
		t.Fatalf("MkdirAll B error = %v", err)
	}
	runtimeA := mustNative(t, rootA, model.Policy{Mode: string(ModeWorkspaceWrite)})
	runtimeB := mustNative(t, rootB, model.Policy{Mode: string(ModeReadOnly)})

	if _, err := runtimeA.Write("a.txt", []byte("A"), false); err != nil {
		t.Fatalf("runtime A write error = %v", err)
	}
	_, err := runtimeB.Write("b.txt", []byte("B"), false)
	assertRuntimeErrorKind(t, err, ErrReadOnly)
	if _, _, err := runtimeB.Read(filepath.Join(rootA, "a.txt"), 0); err == nil {
		t.Fatal("runtime B unexpectedly read runtime A workspace")
	} else {
		assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
	}
	if runtimeA.Root() == runtimeB.Root() || runtimeA.Mode() == runtimeB.Mode() {
		t.Fatalf("runtime contexts contaminated: A(root=%q mode=%q) B(root=%q mode=%q)", runtimeA.Root(), runtimeA.Mode(), runtimeB.Root(), runtimeB.Mode())
	}
}

func TestNativeRejectsSymlinkEscapeWhenSupported(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root error = %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll outside error = %v", err)
	}
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("WriteFile outside error = %v", err)
	}

	fileLink := filepath.Join(root, "file-link.txt")
	if err := os.Symlink(outsideFile, fileLink); err != nil {
		t.Skipf("symlink creation unavailable on this host: %v", err)
	}
	dirLink := filepath.Join(root, "dir-link")
	if err := os.Symlink(outside, dirLink); err != nil {
		t.Skipf("directory symlink creation unavailable on this host: %v", err)
	}

	runtime := mustNative(t, root, model.Policy{Mode: string(ModeWorkspaceWrite)})
	_, _, err := runtime.Read("file-link.txt", 0)
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
	_, err = runtime.Write(filepath.Join("dir-link", "new.txt"), []byte("x"), false)
	assertRuntimeErrorKind(t, err, ErrPathOutsideWorkspace)
}

func mustNative(t *testing.T, root string, policy model.Policy) *Native {
	t.Helper()
	runtime, err := NewNative(model.Workspace{ID: "ws_test", Path: root}, model.EffectiveConfig{
		Policy: &model.ResolvedPolicy{Policy: policy, Source: model.ScopeProject},
	})
	if err != nil {
		t.Fatalf("NewNative() error = %v", err)
	}
	return runtime
}

func assertRuntimeErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want RuntimeError kind %q", want)
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error type = %T, want *RuntimeError: %v", err, err)
	}
	if runtimeErr.Kind != want {
		t.Fatalf("RuntimeError.Kind = %q, want %q (error=%v)", runtimeErr.Kind, want, err)
	}
}
