package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPHPCallGraphCallersCalleesAndCycles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Service.php", `<?php
namespace Demo;
class Service {
  public function run() {
    self::helper();
  }
  public static function helper() { Service::run(); }
}
`)
	writeFixture(t, root, "Controller.php", `<?php
namespace Demo;
class Controller {
  public function action($dynamic) {
    Service::run();
    $dynamic->run();
  }
}
`)
	first, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	for _, tc := range []struct {
		direction string
		want      int
	}{
		{"callers", 3}, {"callees", 1}, {"both", 4},
	} {
		got, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
			Symbol: `Demo\Service::run`, Path: "Service.php", Direction: tc.direction,
			MaxDepth: 1, MaxResults: 100,
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.direction, err)
		}
		if got.Returned != tc.want {
			t.Fatalf("%s got %d want %d: %+v", tc.direction, got.Returned, tc.want, got.Edges)
		}
		var foundResolved, foundCandidate bool
		for _, edge := range got.Edges {
			if edge.Kind == "resolved_call" {
				foundResolved = true
			}
			if edge.Kind == "candidate_call" {
				foundCandidate = true
			}
		}
		if !foundResolved || tc.direction != "callees" && !foundCandidate {
			t.Fatalf("%s missing expected confidence labels: %+v", tc.direction, got.Edges)
		}
	}
	depth, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: `Demo\Service::run`, Direction: "callees", MaxDepth: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if depth.Returned != 2 || len(depth.Nodes) != 2 {
		t.Fatalf("cycle should have 2 edges, 2 nodes: %+v", depth)
	}
	if depth.Truncated {
		t.Fatalf("complete graph should not be truncated")
	}
	bounded, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: `Demo\Service::run`, Direction: "both", MaxResults: 1,
	})
	if err != nil || bounded.Returned != 1 || !bounded.Truncated {
		t.Fatalf("bounded=%+v err=%v", bounded, err)
	}
}

func TestPHPCallGraphAmbiguousAndStale(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "one.php", `<?php
namespace One;
class Handler {
 public function run() {}
 }
`)
	writeFixture(t, root, "two.php", `<?php
namespace Two;
class Handler {
 public function run() {}
 }
`)
	first, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	if _, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{Symbol: "Handler::run"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous declaration must not pick arbitrary symbol: %v", err)
	}
	if _, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{Symbol: `One\Handler::run`, Path: "one.php", Direction: "both"}); err != nil {
		t.Fatalf("fully qualified path lookup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.php"), []byte("<?php\nclass Other { public function run() {} }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{Symbol: `One\Handler::run`, Path: "one.php"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale graph must be rejected: %v", err)
	}
}

func TestPHPCallGraphIndexReuseAfterRefresh(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "A.php", `<?php
class A {
 public function one() { self::two(); }
 public function two() {}
}
`)
	first, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	second, err := AnalyzeIncremental(root, Options{})
	if err != nil || second.ReusedPHPCallFiles != 1 || second.ReindexedPHPCallFiles != 0 {
		t.Fatalf("incremental reuse=%+v err=%v", second, err)
	}
	installFixtureIndex(t, root, second)
	got, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{Symbol: "A::one", Direction: "callees"})
	if err != nil || got.Returned != 1 || got.Edges[0].Kind != "resolved_call" {
		t.Fatalf("graph after reused index: %+v err=%v", got, err)
	}
}
