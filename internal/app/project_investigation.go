package app

import (
	"fmt"
	"sort"

	"ai-dev-manager-v2/internal/projectanalysis"
)

// ProjectPHPInvestigation is a compact, bounded AI-first response: symbol
// identity, relevant call edges, and the few source ranges needed to begin
// investigating. All evidence comes from ADM's own verified project index.
type ProjectPHPInvestigation struct {
	Graph          projectanalysis.PHPCallGraphResult `json:"graph"`
	Excerpts       []ProjectInvestigationExcerpt      `json:"excerpts"`
	IndexWarning   string                             `json:"index_warning,omitempty"`
	EvidenceNotice string                             `json:"evidence_notice"`
}

type ProjectInvestigationExcerpt struct {
	Role      string `json:"role"`
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ProjectPHPInvestigationOptions struct {
	Query        projectanalysis.PHPCallGraphQuery
	ContextLines int
	MaxExcerpts  int
}

// ProjectInvestigatePHP lets an agent get the definition and representative
// callers/callees in a single read-only MCP call without reading whole files.
func (s *Service) ProjectInvestigatePHP(environmentID string, opts ProjectPHPInvestigationOptions) (ProjectPHPInvestigation, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return ProjectPHPInvestigation{}, err
	}
	if opts.Query.MaxResults <= 0 {
		opts.Query.MaxResults = 20
	}
	if opts.Query.MaxResults > 40 {
		opts.Query.MaxResults = 40
	}
	if opts.ContextLines <= 0 {
		opts.ContextLines = 3
	}
	if opts.ContextLines > 6 {
		opts.ContextLines = 6
	}
	if opts.MaxExcerpts <= 0 {
		opts.MaxExcerpts = 6
	}
	if opts.MaxExcerpts > 10 {
		opts.MaxExcerpts = 10
	}

	graph, err := s.ProjectPHPCallGraph(environmentID, opts.Query)
	if err != nil {
		return ProjectPHPInvestigation{}, err
	}
	result := ProjectPHPInvestigation{
		Graph:          graph,
		Excerpts:       make([]ProjectInvestigationExcerpt, 0, opts.MaxExcerpts),
		EvidenceNotice: "Call graph edges are static evidence, not proof of runtime execution. Inspect the returned source before changing behavior. inherited_candidate and candidate_call must not be treated as resolved calls.",
	}
	if graph.Truncated {
		result.IndexWarning = "Call graph is partial or result-limited; absence of a call edge is not evidence that no other caller exists. Check project_index_status and project_analyze bounds."
	}

	seen := map[string]bool{}
	add := func(role, path string, line, before, after int) {
		if len(result.Excerpts) >= opts.MaxExcerpts || path == "" || line < 1 {
			return
		}
		key := fmt.Sprintf("%s:%d", path, line)
		if seen[key] {
			return
		}
		seen[key] = true
		start := max(1, line-before)
		end := line + after
		excerpt := ProjectInvestigationExcerpt{Role: role, Path: path, StartLine: start, EndLine: end}
		content, err := rt.ReadLines(path, start, end, 2048)
		if err != nil {
			excerpt.Error = "source excerpt unavailable within the 2048-byte limit; use read with a narrower line range"
		} else {
			excerpt.Content = content
		}
		result.Excerpts = append(result.Excerpts, excerpt)
	}
	add("definition", graph.Symbol.Path, graph.Symbol.Line, 0, opts.ContextLines*2)

	edges := append([]projectanalysis.PHPCallGraphEdge(nil), graph.Edges...)
	priority := func(edge projectanalysis.PHPCallGraphEdge) int {
		switch edge.Kind {
		case "resolved_call":
			return 0
		case "inherited_candidate":
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if priority(edges[i]) != priority(edges[j]) {
			return priority(edges[i]) < priority(edges[j])
		}
		if edges[i].Path != edges[j].Path {
			return edges[i].Path < edges[j].Path
		}
		return edges[i].Line < edges[j].Line
	})
	// Sample different files before taking a second call from the same file.
	touched := map[string]bool{graph.Symbol.Path: true}
	for pass := 0; pass < 2; pass++ {
		for _, edge := range edges {
			if len(result.Excerpts) >= opts.MaxExcerpts {
				break
			}
			if pass == 0 && touched[edge.Path] {
				continue
			}
			add("call_site", edge.Path, edge.Line, opts.ContextLines, opts.ContextLines)
			touched[edge.Path] = true
		}
	}
	return result, nil
}
