package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareFilesReturnsUnifiedRangeDiff(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "left.txt"), []byte("zero\nalpha\nbeta\ngamma\ntail\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "right.txt"), []byte("skip\nskip\nalpha\nBETA\ngamma\ndelta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.CompareFiles("left.txt", 2, 4, "right.txt", 3, 6, 4096, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if result.Equal {
		t.Fatal("different ranges reported equal")
	}
	if result.Left.StartLine != 2 || result.Left.EndLine != 4 || result.Left.LineCount != 3 {
		t.Fatalf("left metadata=%+v", result.Left)
	}
	if result.Right.StartLine != 3 || result.Right.EndLine != 6 || result.Right.LineCount != 4 {
		t.Fatalf("right metadata=%+v", result.Right)
	}
	for _, want := range []string{
		"--- left.txt:2-4",
		"+++ right.txt:3-6",
		"@@ -2,3 +3,4 @@",
		" alpha",
		"-beta",
		"+BETA",
		" gamma",
		"+delta",
	} {
		if !strings.Contains(result.Diff, want) {
			t.Fatalf("diff missing %q:\n%s", want, result.Diff)
		}
	}
}

func TestCompareFilesEqualRangesReturnNoDiff(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x\nsame\nlines\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("prefix\nsame\nlines\nsuffix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.CompareFiles("a.txt", 2, 3, "b.txt", 2, 3, 4096, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Equal || result.Diff != "" {
		t.Fatalf("equal result=%+v", result)
	}
}

func TestCompareFilesBoundsDiffOutput(t *testing.T) {
	root := t.TempDir()
	left := strings.Repeat("left-line\n", 20)
	right := strings.Repeat("right-line\n", 20)
	if err := os.WriteFile(filepath.Join(root, "left.txt"), []byte(left), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "right.txt"), []byte(right), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.CompareFiles("left.txt", 1, 20, "right.txt", 1, 20, 4096, 96)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatalf("expected truncated result: %+v", result)
	}
	if len(result.Diff) > 96 {
		t.Fatalf("diff length=%d", len(result.Diff))
	}
}
