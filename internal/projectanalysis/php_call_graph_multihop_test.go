package projectanalysis

import (
	"testing"
)

func TestPHPCallGraphCrossFileMultiHopKeepsEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Controller.php", `<?php
class Controller {
 public function entry() {
  Service::work();
 }
}
`)
	writeFixture(t, root, "Service.php", `<?php
class Service {
 public static function work() {
  Repository::save();
  $unknown->save();
 }
}
`)
	writeFixture(t, root, "Repository.php", `<?php
class Repository {
 public static function save() {}
}
`)
	first, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	got, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: "Controller::entry", Path: "Controller.php", Direction: "callees", MaxDepth: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Returned != 3 {
		t.Fatalf("multi-hop edges=%+v", got.Edges)
	}
	resolved := 0
	candidate := 0
	for _, edge := range got.Edges {
		if edge.Kind == "resolved_call" {
			resolved++
		} else if edge.Kind == "candidate_call" {
			candidate++
		}
		if edge.Path == "" || edge.Line <= 0 || edge.Column <= 0 {
			t.Fatalf("missing evidence location: %+v", edge)
		}
	}
	if resolved != 2 || candidate != 1 {
		t.Fatalf("must preserve static vs dynamic classification: %+v", got.Edges)
	}
	if len(got.Nodes) != 4 {
		t.Fatalf("expected 3 concrete definitions plus one unresolved node: %+v", got.Nodes)
	}
	// Candidate calls must not induce a fabricated third-hop graph edge.
	for _, node := range got.Nodes {
		if node.Kind == "unresolved_method" && node.Path != "" {
			t.Fatalf("unresolved variable receiver must not be assigned a made-up path: %+v", node)
		}
	}
}
