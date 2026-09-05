package workspace

import "testing"

func TestApplyMinimumPolicyModeOnlyUpgrades(t *testing.T) {
	for _, tc := range []struct {
		base    string
		minimum string
		want    string
	}{
		{base: "read-only", minimum: "standard", want: "standard"},
		{base: "workspace-write", minimum: "standard", want: "standard"},
		{base: "standard", minimum: "standard", want: "standard"},
		{base: "full", minimum: "standard", want: "full"},
		{base: "standard", minimum: "full", want: "full"},
		{base: "unexpected", minimum: "standard", want: "unexpected"},
	} {
		if got := applyMinimumPolicyMode(tc.base, tc.minimum); got != tc.want {
			t.Fatalf("applyMinimumPolicyMode(%q, %q) = %q, want %q", tc.base, tc.minimum, got, tc.want)
		}
	}
}
