package projectanalysis

import (
	"strings"
	"testing"
)

func TestDefaultIndexLimitsCoverMoreThanLegacySymbolBudget(t *testing.T) {
	root := t.TempDir()
	var php strings.Builder
	php.WriteString("<?php\nclass Controller {\n")
	for i := 0; i < 1501; i++ {
		php.WriteString("public function method")
		php.WriteString(strings.Repeat("x", i/26))
		php.WriteByte(byte('a' + i%26))
		php.WriteString("() {}\n")
	}
	php.WriteString("}\n")
	writeFixture(t, root, "Controller.php", php.String())
	full, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if full.Truncated || full.Symbols != 1502 || DefaultMaxSymbols <= 1200 || DefaultMaxFiles <= 4000 {
		t.Fatalf("default coverage unexpectedly incomplete: %+v", full)
	}
	installFixtureIndex(t, root, full)
	complete, err := QueryIndex(root, IndexQuery{Query: "method", MaxResults: 20})
	if err != nil || !complete.IndexComplete || complete.CoverageWarning != "" {
		t.Fatalf("complete query must not warn of missing coverage: %+v err=%v", complete, err)
	}
}

func TestPartialSymbolIndexExposesCoverageWarningToAgent(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "api.php", `<?php
class Api {
 public function alpha() {}
 public function important() {}
}
`)
	partial, err := Analyze(root, Options{MaxFiles: 100, MaxSymbols: 1})
	if err != nil || !partial.Truncated {
		t.Fatalf("partial index=%+v err=%v", partial, err)
	}
	installFixtureIndex(t, root, partial)
	result, err := QueryIndex(root, IndexQuery{Query: "important", Exact: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Returned != 0 || result.IndexComplete || result.CoverageWarning == "" || !result.Truncated {
		t.Fatalf("zero matches in partial index must not imply symbol absent: %+v", result)
	}
	full, err := Analyze(root, Options{})
	if err != nil || full.Truncated {
		t.Fatalf("full index=%+v err=%v", full, err)
	}
	installFixtureIndex(t, root, full)
	exists, err := QueryIndex(root, IndexQuery{Query: "important", Exact: true})
	if err != nil || exists.Returned != 1 || !exists.IndexComplete || exists.CoverageWarning != "" {
		t.Fatalf("fresh index must find previously missed method: %+v err=%v", exists, err)
	}
}
