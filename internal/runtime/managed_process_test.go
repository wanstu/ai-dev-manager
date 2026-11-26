package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"
)

func TestManagedProcessRuntimeSample(t *testing.T) {
	if goruntime.GOOS != "windows" && goruntime.GOOS != "linux" {
		t.Skip("resource sampling not implemented")
	}
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManagedProcessRuntimeHelper$")
	cmd.Env = append(os.Environ(), "ADM_RUNTIME_PROCESS_READY="+ready)
	configureCommand(cmd)
	managed, err := StartManagedProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait(); managed.Close() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	sample, err := managed.Sample()
	if err != nil || sample.ProcessCount < 1 || sample.MemoryBytes == 0 || sample.SampledAt.IsZero() {
		t.Fatalf("sample %+v %v", sample, err)
	}
	if goruntime.GOOS == "linux" && (sample.Scope != "process_group" || sample.CPUBasis == "") {
		t.Fatalf("Linux scope %+v", sample)
	}
	if goruntime.GOOS == "windows" && sample.Scope != "job" {
		t.Fatalf("Windows scope %+v", sample)
	}
}
func TestManagedProcessRuntimeHelper(t *testing.T) {
	ready := os.Getenv("ADM_RUNTIME_PROCESS_READY")
	if ready == "" {
		return
	}
	if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)
}
