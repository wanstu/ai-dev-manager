package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerifierSandboxJavaRootInfersJDKFromAbsoluteExecutable(t *testing.T) {
	jdkRoot := filepath.Join(t.TempDir(), "jdk-25")
	binDir := filepath.Join(jdkRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(binDir, "java.exe")

	got := verifierSandboxJavaRoot(nil, executable)
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(jdkRoot)) {
		t.Fatalf("java root=%q, want %q", got, jdkRoot)
	}
}

func TestVerifierSandboxJavaRootPrefersExecutableJDKOverStaleJavaHome(t *testing.T) {
	root := t.TempDir()
	executableJDK := filepath.Join(root, "jdk-executable")
	staleJavaHome := filepath.Join(root, "jdk-stale")
	for _, dir := range []string{filepath.Join(executableJDK, "bin"), staleJavaHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(executableJDK, "bin", "javac.exe")

	got := verifierSandboxJavaRoot([]string{"JAVA_HOME=" + staleJavaHome}, executable)
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(executableJDK)) {
		t.Fatalf("java root=%q, want executable JDK %q", got, executableJDK)
	}
}

func TestVerifierSandboxJavaRootFallsBackToJavaHome(t *testing.T) {
	javaHome := filepath.Join(t.TempDir(), "jdk")
	if err := os.MkdirAll(javaHome, 0o755); err != nil {
		t.Fatal(err)
	}

	got := verifierSandboxJavaRoot([]string{"JAVA_HOME=" + javaHome}, "java")
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(javaHome)) {
		t.Fatalf("java root=%q, want JAVA_HOME %q", got, javaHome)
	}
}

func TestVerifierSandboxToolRootsGrantWholeJDKForJava(t *testing.T) {
	jdkRoot := filepath.Join(t.TempDir(), "jdk")
	if err := os.MkdirAll(filepath.Join(jdkRoot, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	roots := verifierSandboxToolRoots(nil, filepath.Join(jdkRoot, "bin", "java.exe"))
	if len(roots) != 1 || !strings.EqualFold(filepath.Clean(roots[0]), filepath.Clean(jdkRoot)) {
		t.Fatalf("tool roots=%v, want only JDK root %q", roots, jdkRoot)
	}
}

func TestVerifierSandboxToolRootsIncludeJavaForGradleAndMaven(t *testing.T) {
	root := t.TempDir()
	javaHome := filepath.Join(root, "jdk")
	gradleHome := filepath.Join(root, "gradle")
	mavenHome := filepath.Join(root, "maven")
	for _, dir := range []string{javaHome, gradleHome, mavenHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{
		"JAVA_HOME=" + javaHome,
		"GRADLE_HOME=" + gradleHome,
		"MAVEN_HOME=" + mavenHome,
	}

	assertRoots := func(name string, executable string, want ...string) {
		t.Helper()
		roots := verifierSandboxToolRoots(env, executable)
		if len(roots) != len(want) {
			t.Fatalf("%s roots=%v, want %v", name, roots, want)
		}
		for i := range want {
			if !strings.EqualFold(filepath.Clean(roots[i]), filepath.Clean(want[i])) {
				t.Fatalf("%s root[%d]=%q, want %q", name, i, roots[i], want[i])
			}
		}
	}

	assertRoots("gradle", "gradle", gradleHome, javaHome)
	assertRoots("maven", "mvn", mavenHome, javaHome)
}

func newSandboxTraversalTestService(t *testing.T, nested bool) (*Service, string) {
	t.Helper()

	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	environmentRoot := workspaceRoot
	if nested {
		environmentRoot = filepath.Join(workspaceRoot, "nested", "environment")
		if err := os.MkdirAll(environmentRoot, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	service := New(filepath.Join(t.TempDir(), "state.json"))
	workspace, err := service.Workspaces.Add(workspaceRoot, "sandbox-traversal-test")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := service.Environments.Create(workspace.ID, "sandbox-traversal-test", environmentRoot)
	if err != nil {
		t.Fatal(err)
	}
	return service, environment.ID
}

func TestSandboxExecutionTraversalLifecycleJoinsCleanupError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows traversal broker lifecycle")
	}

	service, environmentID := newSandboxTraversalTestService(t, true)
	enterCount := 0
	leaveCount := 0
	service.SetSandboxExecutionTraversalRequester(
		func(context.Context, string) error {
			enterCount++
			return nil
		},
		func(context.Context, string) error {
			leaveCount++
			return errors.New("leave failed")
		},
	)

	finish, err := service.beginSandboxExecutionTraversal(context.Background(), environmentID)
	if err != nil {
		t.Fatalf("begin traversal: %v", err)
	}
	if enterCount != 1 {
		t.Fatalf("enter count=%d, want 1", enterCount)
	}

	executionErr := errors.New("execution failed")
	finalErr := finish(executionErr)
	if leaveCount != 1 {
		t.Fatalf("leave count=%d, want 1", leaveCount)
	}
	if finalErr == nil || !strings.Contains(finalErr.Error(), "execution failed") || !strings.Contains(finalErr.Error(), "leave failed") {
		t.Fatalf("final error=%v, want execution and cleanup errors", finalErr)
	}
}

func TestSandboxExecutionTraversalRequiresBrokerOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows traversal broker lifecycle")
	}

	service, environmentID := newSandboxTraversalTestService(t, true)
	if _, err := service.beginSandboxExecutionTraversal(context.Background(), environmentID); err == nil || !strings.Contains(err.Error(), "broker is not configured") {
		t.Fatalf("missing broker error=%v", err)
	}
}

func TestSandboxExecutionTraversalSkipsBrokerWithoutAncestorTraversal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows traversal broker lifecycle")
	}

	service, environmentID := newSandboxTraversalTestService(t, false)
	finish, err := service.beginSandboxExecutionTraversal(context.Background(), environmentID)
	if err != nil {
		t.Fatalf("begin traversal without ancestors: %v", err)
	}
	if err := finish(nil); err != nil {
		t.Fatalf("finish traversal without ancestors: %v", err)
	}
}
