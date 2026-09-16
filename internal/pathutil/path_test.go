package pathutil

import (
	"path/filepath"
	"testing"
)

func TestSameAndWithinPreserveMissingLeafSegments(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "future", "child")

	if !Same(missing, filepath.Join(root, ".", "future", "child")) {
		t.Fatalf("Same should normalize equivalent paths")
	}
	if !Within(root, missing) {
		t.Fatalf("Within should accept a missing descendant under an existing root")
	}
	if Within(root, filepath.Join(root, "..", "outside")) {
		t.Fatalf("Within should reject paths outside the root")
	}
}
