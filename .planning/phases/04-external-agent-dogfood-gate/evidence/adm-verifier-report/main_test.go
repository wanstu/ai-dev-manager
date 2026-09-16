package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestReport(t *testing.T) {
	valid := func(status string, exit int, timeout bool) string {
		return fmt.Sprintf("{\"result\":{\"verifier_id\":\"vf_real\",\"kind\":\"test\",\"status\":%q,\"exit_code\":%d,\"duration_ms\":5,\"timed_out\":%t,\"stdout\":\"do-not-print\"}}", status, exit, timeout)
	}
	cases := []struct {
		name, input string
		want        int
	}{
		{"pass", valid("passed", 0, false), 0},
		{"failure", valid("failed", 1, false), 1},
		{"timeout", valid("failed", -1, true), 1},
		{"skipped", valid("skipped", 0, false), 1},
		{"contradictory-pass", valid("passed", 1, false), 2},
		{"pass-timeout", valid("passed", 0, true), 2},
		{"unknown-status", valid("configured", 0, false), 2},
		{"empty", "", 2},
		{"null", "null", 2},
		{"no-result", "{}", 2},
		{"missing-id", strings.Replace(valid("passed", 0, false), "\"verifier_id\":\"vf_real\",", "", 1), 2},
		{"missing-exit", strings.Replace(valid("passed", 0, false), "\"exit_code\":0,", "", 1), 2},
		{"missing-timeout", strings.Replace(valid("passed", 0, false), "\"timed_out\":false,", "", 1), 2},
		{"missing-duration", strings.Replace(valid("passed", 0, false), "\"duration_ms\":5,", "", 1), 2},
		{"negative-duration", strings.Replace(valid("passed", 0, false), "\"duration_ms\":5", "\"duration_ms\":-1", 1), 2},
		{"trailing-json", valid("passed", 0, false) + " {}", 2},
		{"malformed", "{", 2},
		{"oversize", strings.Repeat(" ", 1048577), 2},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if got := run(strings.NewReader(tt.input), &out); got != tt.want {
				t.Errorf("exit=%d want=%d output=%q", got, tt.want, out.String())
			}
			if strings.Contains(out.String(), "do-not-print") {
				t.Error("raw verifier output leaked")
			}
			if tt.want < 2 && !strings.Contains(out.String(), "vf_real") {
				t.Error("missing verifier identity")
			}
		})
	}
}
