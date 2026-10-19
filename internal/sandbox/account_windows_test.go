//go:build windows

package sandbox

import "testing"

func TestWindowsSandboxAccountNameUsesCurrentUserSID(t *testing.T) {
	name, err := WindowsSandboxAccountName()
	if err != nil {
		t.Fatal(err)
	}
	if name == "" || len(name) > 20 {
		t.Fatalf("invalid sandbox account name %q", name)
	}
}
