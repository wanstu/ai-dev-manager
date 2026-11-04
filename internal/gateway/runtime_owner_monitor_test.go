package gateway

import (
	"testing"
	"time"
)

func TestRuntimeOwnerMonitorInterval(t *testing.T) {
	if got := runtimeOwnerMonitorInterval(false); got != 5*time.Second {
		t.Fatalf("idle monitor interval = %s, want 5s", got)
	}
	if got := runtimeOwnerMonitorInterval(true); got != time.Second {
		t.Fatalf("active monitor interval = %s, want 1s", got)
	}
	if runtimeOwnerIdleMonitorTick < time.Second {
		t.Fatalf("idle monitor interval must never regress to sub-second full state scans: %s", runtimeOwnerIdleMonitorTick)
	}
}
