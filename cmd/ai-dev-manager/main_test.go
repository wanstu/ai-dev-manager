package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestExecutableBootstrapDelegatesToCLISurface(t *testing.T) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	if err := run([]string{"-h"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	output := string(data)
	for _, required := range []string{"adm", "Admin MCP", "adm gateway start"} {
		if !strings.Contains(output, required) {
			t.Fatalf("thin executable bootstrap did not expose CLI help %q:\n%s", required, output)
		}
	}
}
