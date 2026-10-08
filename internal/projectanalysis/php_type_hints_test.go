package projectanalysis

import (
	"strings"
	"testing"
)

func TestPHPCallGraphInheritedAndTypedParameterEvidence(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "models/Base.php", `<?php
namespace Demo\Models;
class Base {
 public function run() {}
}
`)
	writeFixture(t, root, "Controller.php", `<?php
namespace Demo\Controller;
use Demo\Models\Base as ParentService;
class Child extends ParentService {
 public function invoke(ParentService $typed, $unknown) {
  $this->run();
  self::run();
  $typed->run();
  $unknown->run();
 }
}
`)
	first, err := AnalyzeIncremental(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, first)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{
		Name: `Demo\Models\Base::run`, Path: "models/Base.php", Kind: "method",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.References) != 4 {
		t.Fatalf("expected 4 call sites: %+v", refs)
	}
	inherited, typed, dynamic := 0, 0, 0
	for _, r := range refs.References {
		switch {
		case r.Kind == "inherited_candidate" && r.Reason == "direct_extends_clause":
			inherited++
		case r.Kind == "candidate_call" && r.Reason == "parameter_type_matches_definition" && r.TypeHint == `Demo\Models\Base`:
			typed++
		case r.Kind == "candidate_call" && r.Reason == "dynamic_receiver":
			dynamic++
		default:
			t.Fatalf("unexpected confidence classification: %+v", r)
		}
	}
	if inherited != 2 || typed != 1 || dynamic != 1 {
		t.Fatalf("wrong confidence counts inherited=%d typed=%d dynamic=%d %+v", inherited, typed, dynamic, refs)
	}
	graph, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: `Demo\Controller\Child::invoke`,
		Path:   "Controller.php", Direction: "callees", MaxDepth: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if graph.Returned != 4 {
		t.Fatalf("expected 4 independent evidence edges %+v", graph)
	}
	baseEdges := 0
	for _, edge := range graph.Edges {
		if edge.Kind == "resolved_call" {
			t.Fatalf("candidate must never be promoted to certain edge: %+v", edge)
		}
		if edge.TypeHint == `Demo\Models\Base` && edge.Reason != "declared_parameter_type_not_runtime" {
			t.Fatalf("type hint must explain uncertainty: %+v", edge)
		}
		for _, n := range graph.Nodes {
			if edge.To == n.ID && n.Name == `Demo\Models\Base::run` {
				baseEdges++
			}
		}
	}
	if baseEdges != 3 {
		t.Fatalf("target should be linked by 2 parent hints plus type hint: %+v", graph)
	}
	// Existing call index can be reused without losing evidence annotations.
	again, err := AnalyzeIncremental(root, Options{})
	if err != nil || again.ReusedPHPCallFiles != 2 {
		t.Fatalf("not reused: %+v %v", again, err)
	}
	installFixtureIndex(t, root, again)
	after, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: `Demo\Models\Base::run`, Kind: "method", Path: "models/Base.php"})
	if err != nil || len(after.References) != 4 || after.References[2].TypeHint != `Demo\Models\Base` {
		t.Fatalf("cached evidence lost: %+v err=%v", after, err)
	}
}

func TestPHPCallGraphOverrideDoesNotFabricateInheritedReference(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "types.php", `<?php
class ParentType {
 public function run() {}
}
class ChildType extends ParentType {
 public function run() {}
 public function action() {
  $this->run();
  self::run();
 }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "ParentType::run", Kind: "method", Path: "types.php"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.References) != 0 {
		t.Fatalf("child override should exclude lexical calls to its own implementation: %+v", refs)
	}
	g, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: "ChildType::action", Direction: "callees",
	})
	if err != nil || g.Returned != 2 {
		t.Fatalf("child graph=%+v err=%v", g, err)
	}
	for _, edge := range g.Edges {
		if edge.Kind != "resolved_call" {
			t.Fatalf("child local override should be directly resolved: %+v", edge)
		}
	}
}

func TestPHPTypeHintNeverClaimsUnionOrBuiltinIsClass(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "typehints.php", `<?php
class Caller {
 public function invoke(int $count, Service|Other $union, ?Service $optional) {
  $count->run();
  $union->run();
  $optional->run();
 }
}
class Service { public function run() {} }
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	refs, err := FindPHPCallReferences(root, PHPReferenceQuery{Name: "Service::run", Kind: "method"})
	if err != nil {
		t.Fatal(err)
	}
	hinted := 0
	for _, r := range refs.References {
		if strings.Contains(r.Context, "$optional") {
			if r.TypeHint != "Service" {
				t.Fatalf("optional declared type should be an advisory hint: %+v", r)
			}
			hinted++
		} else if r.TypeHint != "" {
			t.Fatalf("union or builtin should not be inferred as single class: %+v", r)
		}
	}
	if hinted != 1 {
		t.Fatalf("expected one safe annotation: %+v", refs)
	}
}
