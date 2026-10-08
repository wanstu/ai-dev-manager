package projectanalysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPHPCallCandidatesUsesVerifiedIndex(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "src/Controller.php", `<?php
class User_goods {
    public function getDealBaseInfo() {}
    public function action() {
        $this->getDealBaseInfo();
        User_goods::getDealBaseInfo();
        // $this->getDealBaseInfo();
        $text = "User_goods::getDealBaseInfo()";
    }
}
`)
	writeFixture(t, root, "src/Other.php", `<?php
function invoke($any) {
    $any->getDealBaseInfo();
    $any->{"getDealBaseInfo"}();
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	references, err := FindPHPCallCandidates(root, "User_goods::getDealBaseInfo", "method", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(references.References) != 3 || references.Truncated {
		t.Fatalf("references=%+v", references)
	}
	for _, ref := range references.References {
		if ref.Kind != "candidate_call" || ref.Line <= 0 || ref.Column <= 0 ||
			!strings.Contains(ref.Context, "getDealBaseInfo") {
			t.Fatalf("invalid reference=%+v", ref)
		}
	}
	bounded, err := FindPHPCallCandidates(root, "getDealBaseInfo", "method", 2)
	if err != nil || len(bounded.References) != 2 || !bounded.Truncated {
		t.Fatalf("bounded=%+v err=%v", bounded, err)
	}

	if err := os.WriteFile(filepath.Join(root, "src", "Other.php"), []byte("<?php $x->getDealBaseInfo();"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FindPHPCallCandidates(root, "getDealBaseInfo", "method", 10); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("modified source should invalidate reference candidates: %v", err)
	}
}

func TestFindPHPCallCandidatesNeedsExistingIndex(t *testing.T) {
	if _, err := FindPHPCallCandidates(t.TempDir(), "run", "function", 5); err == nil {
		t.Fatal("missing index must not be presented as no references")
	}
}
