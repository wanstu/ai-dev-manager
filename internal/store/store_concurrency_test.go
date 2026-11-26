package store

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
)

func TestStoreIndependentInstancesSerialize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	const workers = 8
	const writes = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			s := New(path)
			for i := 0; i < writes; i++ {
				err := s.Update(func(state *model.State) error {
					n, _ := strconv.Atoi(state.GlobalMemory["counter"])
					time.Sleep(time.Millisecond)
					state.GlobalMemory["counter"] = strconv.Itoa(n + 1)
					state.GlobalMemory[fmt.Sprintf("%d/%d", worker, i)] = "saved"
					return nil
				})
				if err != nil {
					errs <- err
					return
				}
				if _, err := s.Load(); err != nil {
					errs <- err
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	state, err := New(path).Load()
	if err != nil || state.GlobalMemory["counter"] != strconv.Itoa(workers*writes) || len(state.GlobalMemory) != workers*writes+1 {
		t.Fatalf("lost concurrent updates: %+v %v", state.GlobalMemory, err)
	}
}

func TestStoreLockTimeoutAndRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	lock, err := acquireStateLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if other, err := acquireStateLockWithin(path, 80*time.Millisecond); err == nil {
		other.Close()
		t.Fatal("competing lock succeeded")
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	s := New(path)
	if err := s.Update(func(state *model.State) error { state.GlobalMemory["stable"] = "old"; return nil }); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("rollback")
	if err := s.Update(func(state *model.State) error { state.GlobalMemory["stable"] = "new"; return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	state, err := New(path).Load()
	if err != nil || state.GlobalMemory["stable"] != "old" {
		t.Fatalf("rollback %+v %v", state, err)
	}
	if _, err := os.Stat(path + ".lock"); err != nil {
		t.Fatal("stable lock file must remain:", err)
	}
}

func TestStoreConcurrentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	const workers = 3
	const writes = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestStoreProcessHelper$")
			cmd.Env = append(os.Environ(), "ADM_STORE_TEST_PATH="+path, "ADM_STORE_TEST_WORKER="+strconv.Itoa(worker))
			out, err := cmd.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("worker %d: %w: %s", worker, err, out)
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	state, err := New(path).Load()
	if err != nil || state.GlobalMemory["counter"] != strconv.Itoa(workers*writes) || len(state.GlobalMemory) != workers*writes+1 {
		t.Fatalf("process updates lost: %+v %v", state.GlobalMemory, err)
	}
}

func TestStoreProcessHelper(t *testing.T) {
	path := os.Getenv("ADM_STORE_TEST_PATH")
	if path == "" {
		return
	}
	worker := os.Getenv("ADM_STORE_TEST_WORKER")
	s := New(path)
	for i := 0; i < 12; i++ {
		if err := s.Update(func(state *model.State) error {
			n, _ := strconv.Atoi(state.GlobalMemory["counter"])
			time.Sleep(2 * time.Millisecond)
			state.GlobalMemory["counter"] = strconv.Itoa(n + 1)
			state.GlobalMemory[worker+"/"+strconv.Itoa(i)] = "saved"
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStoreLockReleasedAfterProcessExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestStoreLockExitHelper$")
	cmd.Env = append(os.Environ(), "ADM_STORE_LOCK_EXIT_PATH="+path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v %s", err, out)
	}
	lock, err := acquireStateLockWithin(path, 2*time.Second)
	if err != nil {
		t.Fatal("process exit retained lock:", err)
	}
	lock.Close()
}

func TestStoreLockExitHelper(t *testing.T) {
	path := os.Getenv("ADM_STORE_LOCK_EXIT_PATH")
	if path == "" {
		return
	}
	// Deliberately terminate without closing the lock handle.
	if _, err := acquireStateLock(path); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}
