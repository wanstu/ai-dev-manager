package projectanalysis

import (
	"strings"
	"testing"
)

func TestPHPReferencesInheritedCallStaysCandidate(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "base.php", `<?php
class Base {
  public function run() {}
}
class Child extends Base {
  public function inherited() { $this->run(); self::run(); }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Base::run", Kind: "method", Path: "base.php"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.References) != 2 {
		t.Fatalf("expected 2 inherited call candidates: %+v", refs)
	}
	for _, ref := range refs.References {
		if ref.Kind != "candidate_call" {
			t.Fatalf("inheritance not analyzed; must not claim resolved: %+v", ref)
		}
	}
}

func TestPHPReferencesAddedSourceRequiresIndexRefresh(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "base.php", `<?php class Foo {}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	writeFixture(t, root, "added.php", `<?php function helper() { Foo::run(); }
`)
	_, err = FindPHPCallReferences(root, PHPReferenceQuery{Name: "Foo::run", Kind: "method"})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("added PHP source must invalidate index before reference query: %v", err)
	}
}
