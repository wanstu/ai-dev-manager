package app

import (
	"testing"
	"time"

	"ai-dev-manager-v2/internal/model"
)

func TestEnvironmentSummaryHidesWriterOwner(t *testing.T) {
	now := time.Now().UTC()
	env := model.Environment{
		ID:            "env_test",
		Writer:        &model.WriterLease{Owner: "secret-writer-owner", AcquiredAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Minute)},
		PrivateMemory: map[string]string{"secret": "value"},
	}
	summary := environmentSummary(env)
	if summary.Writer == nil {
		t.Fatal("writer lease presence should remain observable")
	}
	if summary.Writer.Owner != "" {
		t.Fatalf("writer owner leaked in management summary: %q", summary.Writer.Owner)
	}
	if summary.Writer.ExpiresAt.IsZero() {
		t.Fatal("writer expiry should remain observable")
	}
	if summary.PrivateMemory != nil || summary.PrivateMemoryCount != 1 {
		t.Fatal("private memory values must remain hidden while count is preserved")
	}
}
