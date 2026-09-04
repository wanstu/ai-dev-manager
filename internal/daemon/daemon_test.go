package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMetadataRoundTripAndInstanceSafeRemoval(t *testing.T) {
	root := t.TempDir()
	meta := Metadata{
		InstanceID:      "daemon_test",
		PID:             123,
		ControlEndpoint: "http://127.0.0.1:43210",
		State:           StateRunning,
		StartedAt:       time.Now().UTC().Truncate(time.Second),
	}
	if err := writeMetadata(root, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	got, err := ReadMetadata(root)
	if err != nil {
		t.Fatalf("ReadMetadata() error = %v", err)
	}
	if got.InstanceID != meta.InstanceID || got.PID != meta.PID || got.ControlEndpoint != meta.ControlEndpoint || got.State != meta.State || !got.StartedAt.Equal(meta.StartedAt) {
		t.Fatalf("ReadMetadata() = %+v, want %+v", got, meta)
	}
	if err := removeMetadataIfInstance(root, "other"); err != nil {
		t.Fatalf("removeMetadataIfInstance(other) error = %v", err)
	}
	if _, err := ReadMetadata(root); err != nil {
		t.Fatalf("metadata removed by wrong instance: %v", err)
	}
	if err := removeMetadataIfInstance(root, meta.InstanceID); err != nil {
		t.Fatalf("removeMetadataIfInstance(owner) error = %v", err)
	}
	if _, err := ReadMetadata(root); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("ReadMetadata() after removal error = %v, want ErrNotRunning", err)
	}
}

func TestLeaseRejectsSecondOwnerAndRecoversStaleLease(t *testing.T) {
	root := t.TempDir()
	first, err := acquireLease(root, "one")
	if err != nil {
		t.Fatalf("acquireLease(first) error = %v", err)
	}
	if _, err := acquireLease(root, "two"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("acquireLease(second) error = %v, want ErrAlreadyRunning", err)
	}
	old := time.Now().Add(-leaseStaleAfter - time.Second)
	if err := os.Chtimes(first.path, old, old); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	second, err := acquireLease(root, "two")
	if err != nil {
		t.Fatalf("acquireLease(after stale) error = %v", err)
	}
	if err := second.release(); err != nil {
		t.Fatalf("release() error = %v", err)
	}
}

func TestControlURLRejectsNonLoopbackMetadata(t *testing.T) {
	for _, endpoint := range []string{
		"https://example.com:443",
		"http://192.0.2.10:1234",
		"file:///tmp/socket",
	} {
		if _, err := controlURL(endpoint, "/health"); !errors.Is(err, ErrUnsafeEndpoint) {
			t.Fatalf("controlURL(%q) error = %v, want ErrUnsafeEndpoint", endpoint, err)
		}
	}
	if got, err := controlURL("http://127.0.0.1:1234", "/health"); err != nil || got != "http://127.0.0.1:1234/health" {
		t.Fatalf("controlURL(loopback) = %q, %v", got, err)
	}
}

func TestRunRejectsNonLoopbackListen(t *testing.T) {
	if err := Run(context.Background(), t.TempDir(), "0.0.0.0:0"); err == nil {
		t.Fatal("Run(non-loopback) unexpectedly succeeded")
	}
}

func TestRunRejectsCorruptDesiredStateBeforePublishingHealth(t *testing.T) {
	root := t.TempDir()
	path := desiredStatePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), root, "127.0.0.1:0")
	if !errors.Is(err, ErrCorruptDesiredState) {
		t.Fatalf("Run(corrupt desired state) error = %v, want ErrCorruptDesiredState", err)
	}
	if _, err := ReadMetadata(root); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("daemon metadata published despite corrupt desired state: %v", err)
	}
}

func TestRunHealthStopAndCleanup(t *testing.T) {
	root := t.TempDir()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), root, "127.0.0.1:0")
	}()

	var meta Metadata
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		probeCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		observed, err := Status(probeCtx, root)
		cancel()
		if err == nil {
			meta = observed
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if meta.InstanceID == "" || meta.State != StateRunning {
		t.Fatalf("daemon never became healthy: %+v", meta)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	stopped, err := Stop(stopCtx, root)
	cancel()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopped.InstanceID != meta.InstanceID || stopped.State != StateStopped {
		t.Fatalf("Stop() = %+v, want instance %s stopped", stopped, meta.InstanceID)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not exit after stop")
	}
	if _, err := os.Stat(filepath.Join(root, runtimeDirName, metadataFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("metadata file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, runtimeDirName, leaseFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lease file still exists: %v", err)
	}
}
