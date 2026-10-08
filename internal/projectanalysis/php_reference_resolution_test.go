package projectanalysis

import (
	"strings"
	"testing"
)

func TestPHPReferencesResolveCrossFileAndPreserveUnknownCalls(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/BillingService.php", `<?php
namespace Demo\Services;
class BillingService {
    public function run() {}
    public function own() {
        $this->run();
        self::run();
        static::run();
    }
}
`)
	writeFixture(t, root, "src/Controller.php", `<?php
namespace Demo\Controller;
use Demo\Services\BillingService as Target;
class Other {
    public static function run() {}
}
class Controller {
    public function action($someObject) {
        Target::run();
        \Demo\Services\BillingService::run();
        Other::run();
        $someObject->run();
        // Target::run();
        $literal = "Target::run()";
    }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{
		Name: `Demo\Services\BillingService::run`,
		Path: "src/BillingService.php",
		Kind: "method",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.References) != 6 {
		t.Fatalf("got %d refs: %+v", len(refs.References), refs.References)
	}
	var resolved, candidate int
	for _, ref := range refs.References {
		if strings.Contains(ref.Context, "Other::run()") {
			t.Fatalf("other class call must be excluded: %+v", ref)
		}
		switch ref.Kind {
		case "resolved_call":
			resolved++
		case "candidate_call":
			candidate++
		default:
			t.Fatalf("unknown reference classification: %+v", ref)
		}
	}
	if resolved != 4 || candidate != 2 {
		t.Fatalf("resolved=%d candidate=%d: %+v", resolved, candidate, refs.References)
	}
	if !strings.Contains(refs.References[3].Path, "Controller.php") {
		t.Fatalf("missing cross file call: %+v", refs.References)
	}
}

func TestPHPReferencesAmbiguousClassDoesNotInventResolvedEdges(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.php", `<?php
namespace Alpha;
class Handler {
  public function go() {}
  public function test() { $this->go(); }
}
`)
	writeFixture(t, root, "b.php", `<?php
namespace Beta;
class Handler {
  public function go() {}
  public function test() { self::go(); }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	ambiguous, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Handler::go", Kind: "method"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ambiguous.References) != 2 {
		t.Fatalf("ambiguous=%+v", ambiguous)
	}
	for _, ref := range ambiguous.References {
		if ref.Kind != "candidate_call" {
			t.Fatalf("ambiguous symbol must remain candidate: %+v", ref)
		}
	}
	specific, err := FindPHPCallReferences(root, PHPReferenceQuery{
		Name: `Alpha\Handler::go`, Path: "a.php", Kind: "method",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specific.References) != 1 || specific.References[0].Kind != "resolved_call" ||
		specific.References[0].Path != "a.php" {
		t.Fatalf("specific=%+v", specific)
	}
}
