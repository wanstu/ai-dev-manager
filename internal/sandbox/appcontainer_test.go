package sandbox

import (
	"strings"
	"testing"
)

func TestStableAppContainerProfileName(t *testing.T) {
	first, err := StableAppContainerProfileName("env_123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := StableAppContainerProfileName("env_123")
	if err != nil {
		t.Fatal(err)
	}
	other, err := StableAppContainerProfileName("env_456")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("profile name is not stable: %q != %q", first, second)
	}
	if first == other {
		t.Fatalf("different sandbox scopes shared profile name %q", first)
	}
	if !strings.HasPrefix(first, appContainerProfilePrefix) || len(first) > 64 {
		t.Fatalf("invalid AppContainer profile name %q", first)
	}
}

func TestStableAppContainerProfileNameRejectsEmptyScope(t *testing.T) {
	if _, err := StableAppContainerProfileName("  "); err == nil {
		t.Fatal("empty sandbox scope was accepted")
	}
}
