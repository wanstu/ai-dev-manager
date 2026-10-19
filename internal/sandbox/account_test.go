package sandbox

import (
	"strings"
	"testing"
)

func TestStableWindowsSandboxUsername(t *testing.T) {
	first, err := StableWindowsSandboxUsername("S-1-5-21-100-200-300-400")
	if err != nil {
		t.Fatal(err)
	}
	second, err := StableWindowsSandboxUsername("s-1-5-21-100-200-300-400")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("stable username changed with SID case: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, windowsSandboxUserPrefix) {
		t.Fatalf("username %q missing ADM prefix", first)
	}
	if len(first) > 20 {
		t.Fatalf("Windows sandbox username too long: %q", first)
	}
}

func TestGenerateSandboxPassword(t *testing.T) {
	for i := 0; i < 16; i++ {
		password, err := GenerateSandboxPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(password) != sandboxPasswordLength {
			t.Fatalf("password length = %d", len(password))
		}
		if err := ValidateSandboxPassword(password); err != nil {
			t.Fatalf("generated password rejected: %v", err)
		}
	}
}

func TestValidateSandboxPasswordRejectsWeakValues(t *testing.T) {
	for _, value := range []string{
		"short",
		"abcdefghijklmnopqrstuvwxyz123456",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ123456",
		"AbcdefghijklmnopqrstuvwxyzABCDE",
	} {
		if err := ValidateSandboxPassword(value); err == nil {
			t.Fatalf("weak password unexpectedly accepted: %q", value)
		}
	}
}
