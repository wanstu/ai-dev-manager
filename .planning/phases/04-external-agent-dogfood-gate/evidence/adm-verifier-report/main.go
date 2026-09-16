package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type result struct {
	ID       string `json:"verifier_id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Exit     *int   `json:"exit_code"`
	Duration *int64 `json:"duration_ms"`
	Timeout  *bool  `json:"timed_out"`
}

func run(in io.Reader, out io.Writer) int {
	invalid := func() int { fmt.Fprintln(out, "invalid verifier evidence"); return 2 }
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(in, limit+1))
	if err != nil || len(data) > limit {
		return invalid()
	}
	var envelope struct {
		Result *result `json:"result"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if d.Decode(&envelope) != nil {
		return invalid()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return invalid()
	}
	r := envelope.Result
	if r == nil || strings.TrimSpace(r.ID) == "" || r.Exit == nil || r.Timeout == nil || r.Duration == nil || *r.Duration < 0 {
		return invalid()
	}
	switch r.Kind {
	case "test", "lint", "build", "custom":
	default:
		return invalid()
	}
	switch r.Status {
	case "passed":
		if *r.Exit != 0 || *r.Timeout {
			return invalid()
		}
	case "failed", "skipped":
	default:
		return invalid()
	}
	fmt.Fprintf(out, "%s %s exit=%d timed_out=%t\n", r.ID, r.Status, *r.Exit, *r.Timeout)
	if r.Status == "passed" {
		return 0
	}
	return 1
}

func main() { os.Exit(run(os.Stdin, os.Stdout)) }
