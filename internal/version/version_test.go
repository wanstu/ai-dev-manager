package version

import "testing"

func TestCurrentUsesLinkerVersion(t *testing.T) {
	before := Version
	t.Cleanup(func() { Version = before })
	Version = "  v9.8.7  "
	if got := Current(); got != "v9.8.7" {
		t.Fatalf("Current()=%q want v9.8.7", got)
	}
}

func TestCurrentDevelopmentFallback(t *testing.T) {
	before := Version
	t.Cleanup(func() { Version = before })
	Version = ""
	got := Current()
	if got == "" {
		t.Fatal("Current() must never be empty")
	}
	Version = "dev"
	if got := Current(); got != "dev" {
		t.Fatalf("explicit development build version = %q, want dev", got)
	}
}
