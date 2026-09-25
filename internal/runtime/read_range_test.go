package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLinesReturnsInclusiveRangeAndPreservesLineEndings(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	content := "one\r\ntwo\r\nthree\nfour"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := rt.ReadLines("sample.txt", 2, 3, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if got != "two\r\nthree\n" {
		t.Fatalf("range=%q", got)
	}
}

func TestReadLinesCanReadSmallRangeFromLargeFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	var b strings.Builder
	for i := 0; i < 20000; i++ {
		b.WriteString("padding-padding-padding-padding\n")
	}
	b.WriteString("target\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := rt.ReadLines("large.txt", 20001, 20001, 32)
	if err != nil {
		t.Fatal(err)
	}
	if got != "target\n" {
		t.Fatalf("range=%q", got)
	}
}

func TestReadLinesValidatesRangeAndSelectedOutputLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rt.ReadLines("sample.txt", -1, 0, 1024); err == nil {
		t.Fatal("expected invalid start_line error")
	}
	if _, err := rt.ReadLines("sample.txt", 3, 2, 1024); err == nil {
		t.Fatal("expected invalid range error")
	}
	if _, err := rt.ReadLines("sample.txt", 1, 2, 5); err == nil {
		t.Fatal("expected max_bytes error")
	}
}
