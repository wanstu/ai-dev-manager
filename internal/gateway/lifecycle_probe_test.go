package gateway

import "testing"

func TestHTTPProbeBaseURLUsesLoopbackForWildcardListen(t *testing.T) {
	tests := map[string]string{
		"0.0.0.0:8001":   "http://127.0.0.1:8001",
		":8001":          "http://127.0.0.1:8001",
		"[::]:8001":      "http://[::1]:8001",
		"127.0.0.1:8001": "http://127.0.0.1:8001",
	}
	for listen, want := range tests {
		got, err := HTTPProbeBaseURL(listen)
		if err != nil {
			t.Fatalf("HTTPProbeBaseURL(%q): %v", listen, err)
		}
		if got != want {
			t.Fatalf("HTTPProbeBaseURL(%q)=%q want %q", listen, got, want)
		}
	}
}
