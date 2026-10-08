package projectanalysis

import (
	"fmt"
	"testing"
)

func TestPHPCallGraphPrefersResolvedEvidenceUnderSmallLimit(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "service.php", `<?php
class Service {
    public function run() {}
}
`)
	for i := 0; i < 15; i++ {
		writeFixture(t, root, fmt.Sprintf("a_unknown_%02d.php", i), fmt.Sprintf(`<?php
class Unknown%d {
    public function invoke($dynamic) { $dynamic->run(); }
}
`, i))
	}
	writeFixture(t, root, "z_real.php", `<?php
class Actual {
    public function invoke() { Service::run(); }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	graph, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: "Service::run", Path: "service.php", Direction: "callers", MaxResults: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, edge := range graph.Edges {
		if edge.Path == "z_real.php" && edge.Kind == "resolved_call" {
			found = true
		}
	}
	if !found {
		t.Fatalf("AI must receive known caller before low-confidence matches when capped: %+v", graph.Edges)
	}
	if graph.Edges[0].Kind != "resolved_call" {
		t.Fatalf("resolved evidence should also be displayed first: %+v", graph.Edges)
	}
	if graph.Returned != 4 || !graph.Truncated {
		t.Fatalf("expected bounded graph result: %+v", graph)
	}
}

func TestPHPCallGraphPrefersResolvedCalleeUnderSmallLimit(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "a.php", `<?php
class Service {
    public function target() {}
}
class Client {
    public function run($x) {
        $x->missingA();
        $x->missingB();
        $x->missingC();
        Service::target();
    }
}
`)
	result, err := Analyze(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	installFixtureIndex(t, root, result)
	graph, err := ProjectPHPCallGraph(root, PHPCallGraphQuery{
		Symbol: "Client::run", Direction: "callees", MaxResults: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, edge := range graph.Edges {
		if edge.Kind == "resolved_call" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a known callee must not be displaced by uncertain calls under cap: %+v", graph.Edges)
	}
}
