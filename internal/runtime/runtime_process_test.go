package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPrepareCommandReusesExecAuthority(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, []string{os.Args[0]})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := rt.PrepareCommand(context.Background(), os.Args[0], []string{"-test.run=^$"}, "nested")
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(cmd.Dir, nested) {
		t.Fatalf("prepared cwd=%q want %q", cmd.Dir, nested)
	}
	if _, err := rt.PrepareCommand(context.Background(), "definitely-not-allowed", nil, ""); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("forbidden executable must fail locally, got %v", err)
	}
	if _, err := rt.PrepareCommand(context.Background(), os.Args[0], nil, "../escape"); err == nil || !strings.Contains(err.Error(), "escapes environment root") {
		t.Fatalf("escaped cwd must fail locally, got %v", err)
	}
}

func TestCommandUsesConfiguredBaseEnvironment(t *testing.T) {
	rt, err := NewWithEnvironment(t.TempDir(), []string{os.Args[0]}, []string{
		"ADM_RUNTIME_BASE=refreshed",
		"PATH=" + os.Getenv("PATH"),
	})
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(values []string, key string) string {
		for _, value := range values {
			parts := strings.SplitN(value, "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], key) {
				return parts[1]
			}
		}
		return ""
	}
	prepared, err := rt.PrepareCommand(context.Background(), os.Args[0], nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := lookup(prepared.Env, "ADM_RUNTIME_BASE"); got != "refreshed" {
		t.Fatalf("PrepareCommand base environment = %q; want refreshed", got)
	}
	command, err := rt.Command(context.Background(), os.Args[0], nil, map[string]string{
		"ADM_RUNTIME_BASE": "override",
		"ADM_RUNTIME_EXTRA": "extra",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := lookup(command.Env, "ADM_RUNTIME_BASE"); got != "override" {
		t.Fatalf("Command override environment = %q; want override", got)
	}
	if got := lookup(command.Env, "ADM_RUNTIME_EXTRA"); got != "extra" {
		t.Fatalf("Command extra environment = %q; want extra", got)
	}
}

func TestListeningTCPPortsObservesOnlyOwnedPID(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows owned-port observation")
	}
	portFile := filepath.Join(t.TempDir(), "port.txt")
	cmd := exec.Command(os.Args[0], "-test.run=^TestOwnedPortHelper$")
	cmd.Env = append(os.Environ(), "ADM_TEST_OWNED_PORT_HELPER=1", "ADM_TEST_OWNED_PORT_FILE="+portFile)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	var port int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(portFile)
		if err == nil {
			port, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if port > 0 {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if port == 0 {
		t.Fatal("owned port helper did not report a port")
	}

	found := false
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ports, err := ListeningTCPPorts(cmd.Process.Pid)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range ports {
			if got == port {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		diagnostic, _ := exec.Command("netstat", "-ano", "-p", "tcp").CombinedOutput()
		var matching []string
		for _, line := range strings.Split(string(diagnostic), "\n") {
			if strings.Contains(line, strconv.Itoa(cmd.Process.Pid)) {
				matching = append(matching, strings.TrimSpace(line))
			}
		}
		t.Fatalf("owned pid %d did not report listening port %d; netstat=%q", cmd.Process.Pid, port, matching)
	}
	if ports, err := ListeningTCPPorts(os.Getpid()); err != nil {
		t.Fatal(err)
	} else {
		for _, got := range ports {
			if got == port {
				t.Fatalf("port %d leaked into unrelated pid %d facts", port, os.Getpid())
			}
		}
	}
}

func TestOwnedPortHelper(t *testing.T) {
	if os.Getenv("ADM_TEST_OWNED_PORT_HELPER") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(os.Getenv("ADM_TEST_OWNED_PORT_FILE"), []byte(fmt.Sprint(port)), 0o644); err != nil {
		panic(err)
	}
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		_ = connection.Close()
	}
}
