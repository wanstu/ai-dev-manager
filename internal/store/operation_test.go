package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriterOperationGenerationBarrier(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "state.json")
	a, b := New(path), New(path)
	old := OperationIdentity{Root: root, EnvironmentID: "a", Generation: "old"}
	first, err := a.BeginOperation(old)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := b.BeginOperation(old)
	if err != nil {
		t.Fatal("same generation must coexist:", err)
	}
	defer second.Close()
	for _, next := range []OperationIdentity{
		{Root: root, EnvironmentID: "a", Generation: "new"},
		{Root: filepath.Join(root, "child"), EnvironmentID: "child", Generation: "new"},
	} {
		if g, err := b.BeginOperation(next); err == nil {
			g.Close()
			t.Fatal("overlapping new lease admitted")
		} else if !strings.Contains(err.Error(), "writer_handoff_pending") {
			t.Fatal(err)
		}
	}
	sibling, err := b.BeginOperation(OperationIdentity{Root: filepath.Join(filepath.Dir(root), "unrelated"), EnvironmentID: "other", Generation: "new"})
	if err != nil {
		t.Fatal(err)
	}
	sibling.Close()
	first.Close()
	if g, err := b.BeginOperation(OperationIdentity{Root: root, EnvironmentID: "a", Generation: "new"}); err == nil {
		g.Close()
		t.Fatal("second old operation ignored")
	}
	second.Close()
	g, err := b.BeginOperation(OperationIdentity{Root: root, EnvironmentID: "a", Generation: "new"})
	if err != nil {
		t.Fatal(err)
	}
	g.Close()
}

func TestWriterOperationAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "state.json")
	ready := filepath.Join(filepath.Dir(path), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWriterOperationProcessHelper$")
	cmd.Env = append(os.Environ(), "ADM_OPERATION_TEST_STATE="+path, "ADM_OPERATION_TEST_ROOT="+root, "ADM_OPERATION_TEST_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not register operation")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s := New(path)
	next := OperationIdentity{Root: root, EnvironmentID: "a", Generation: "replacement"}
	if g, err := s.BeginOperation(next); err == nil {
		g.Close()
		t.Fatal("cross-process handoff admitted")
	} else if !strings.Contains(err.Error(), "writer_handoff_pending") {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	g, err := s.BeginOperation(next)
	if err != nil {
		t.Fatal("dead process retained operation:", err)
	}
	g.Close()
}

func TestWriterOperationProcessHelper(t *testing.T) {
	path := os.Getenv("ADM_OPERATION_TEST_STATE")
	if path == "" {
		return
	}
	g, err := New(path).BeginOperation(OperationIdentity{Root: os.Getenv("ADM_OPERATION_TEST_ROOT"), EnvironmentID: "a", Generation: "original"})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if err := os.WriteFile(os.Getenv("ADM_OPERATION_TEST_READY"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Second)
}
