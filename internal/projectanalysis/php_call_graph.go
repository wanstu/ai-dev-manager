package projectanalysis

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type PHPCallGraphQuery struct {
	Symbol     string `json:"symbol"`
	Path       string `json:"path,omitempty"`
	Direction  string `json:"direction,omitempty"`
	MaxDepth   int    `json:"max_depth,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type PHPCallGraphNode struct {
	ID   string `json:"id"`
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type PHPCallGraphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Kind     string `json:"kind"`
	Reason   string `json:"reason,omitempty"`
	TypeHint string `json:"type_hint,omitempty"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Context  string `json:"context,omitempty"`
}

type PHPCallGraphResult struct {
	Symbol    PHPCallGraphNode   `json:"symbol"`
	Direction string             `json:"direction"`
	Nodes     []PHPCallGraphNode `json:"nodes"`
	Edges     []PHPCallGraphEdge `json:"edges"`
	Returned  int                `json:"returned"`
	Truncated bool               `json:"truncated,omitempty"`
}

// phpGraphEvidencePriority ranks bounded answers for an AI investigator.
// Only lexical evidence (not inferred runtime certainty) receives rank 0.
func phpGraphEvidencePriority(kind, hint string) int {
	switch kind {
	case "resolved_call":
		return 0
	case "inherited_candidate":
		return 1
	}
	if hint != "" {
		return 2
	}
	return 3
}

// ProjectPHPCallGraph queries only ADM's own artifact-verified index.
// Resolved edges mean statically identified lexical ownership, never proof
// of runtime dispatch. Candidates must not be expanded transitively.
func ProjectPHPCallGraph(root string, q PHPCallGraphQuery) (PHPCallGraphResult, error) {
	q.Symbol = strings.TrimSpace(q.Symbol)
	q.Direction = strings.ToLower(strings.TrimSpace(q.Direction))
	if q.Direction == "" {
		q.Direction = "both"
	}
	if q.Direction != "both" && q.Direction != "callers" && q.Direction != "callees" {
		return PHPCallGraphResult{}, fmt.Errorf("direction must be callers, callees, or both")
	}
	if q.MaxDepth <= 0 {
		q.MaxDepth = 1
	}
	if q.MaxDepth > 3 {
		q.MaxDepth = 3
	}
	if q.MaxResults <= 0 {
		q.MaxResults = 100
	}
	if q.MaxResults > 500 {
		q.MaxResults = 500
	}
	if q.Symbol == "" {
		return PHPCallGraphResult{}, fmt.Errorf("symbol is required")
	}
	if q.Path != "" {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(q.Path)))
		if filepath.IsAbs(filepath.FromSlash(clean)) || clean == ".." || strings.HasPrefix(clean, "../") {
			return PHPCallGraphResult{}, fmt.Errorf("invalid symbol path")
		}
		q.Path = clean
	}
	manifest, filesData, symbolData, callsData, err := readVerifiedPHPCallIndex(root)
	if err != nil {
		return PHPCallGraphResult{}, err
	}
	if err := verifyIndexMetadata(root, manifest, filesData); err != nil {
		return PHPCallGraphResult{}, fmt.Errorf("ADM project index is stale: %w; run project_analyze", err)
	}
	symbols := make([]SymbolRecord, 0)
	byFQN := map[string][]SymbolRecord{}
	byName := map[string][]SymbolRecord{}
	err = eachJSONLine(symbolData, func(line []byte) error {
		var s SymbolRecord
		if err := json.Unmarshal(line, &s); err != nil {
			return err
		}
		if s.Language != "PHP" || (s.Kind != "method" && s.Kind != "function") {
			return nil
		}
		if s.Path == "" || s.Line < 1 || s.QualifiedName == "" {
			return fmt.Errorf("invalid indexed PHP symbol")
		}
		symbols = append(symbols, s)
		byFQN[strings.ToLower(s.QualifiedName)] = append(byFQN[strings.ToLower(s.QualifiedName)], s)
		byName[strings.ToLower(phpCallerShortName(s.QualifiedName))] = append(byName[strings.ToLower(phpCallerShortName(s.QualifiedName))], s)
		return nil
	})
	if err != nil {
		return PHPCallGraphResult{}, err
	}
	candidates := byFQN[strings.ToLower(strings.TrimPrefix(q.Symbol, "\\"))]
	if len(candidates) == 0 {
		short := phpCallerShortName(q.Symbol)
		owner := ""
		if pos := strings.LastIndex(q.Symbol, "::"); pos >= 0 {
			owner = strings.TrimPrefix(q.Symbol[:pos], "\\")
		}
		for _, s := range byName[strings.ToLower(short)] {
			if owner != "" {
				symbolOwner := strings.TrimSuffix(s.QualifiedName, "::"+phpCallerShortName(s.QualifiedName))
				shortOwner := symbolOwner
				if i := strings.LastIndex(shortOwner, "\\"); i >= 0 {
					shortOwner = shortOwner[i+1:]
				}
				if !strings.EqualFold(owner, symbolOwner) && !strings.EqualFold(owner, shortOwner) {
					continue
				}
			}
			candidates = append(candidates, s)
		}
	}
	matches := make([]SymbolRecord, 0)
	for _, s := range candidates {
		if q.Path == "" || strings.EqualFold(filepath.ToSlash(s.Path), q.Path) {
			matches = append(matches, s)
		}
	}
	if len(matches) != 1 {
		if len(matches) == 0 {
			return PHPCallGraphResult{}, fmt.Errorf("PHP symbol %q not found in index; run project_analyze", q.Symbol)
		}
		return PHPCallGraphResult{}, fmt.Errorf("PHP symbol %q is ambiguous (%d definitions); supply fully qualified symbol and path", q.Symbol, len(matches))
	}
	target := matches[0]
	toNode := func(s SymbolRecord) PHPCallGraphNode {
		return PHPCallGraphNode{ID: fmt.Sprintf("php:%s:%d:%s", s.Path, s.Line, s.QualifiedName),
			Path: s.Path, Line: s.Line, Name: s.QualifiedName, Kind: s.Kind}
	}
	rootNode := toNode(target)
	result := PHPCallGraphResult{
		Symbol: rootNode, Direction: q.Direction,
		Nodes: []PHPCallGraphNode{rootNode}, Edges: []PHPCallGraphEdge{},
		Truncated: manifest.Truncated || manifest.ParseIssues > 0,
	}
	allCalls := make([]PHPCallRecord, 0, manifest.CallsIndexed)
	byCaller := map[string][]PHPCallRecord{}
	byTargetName := map[string][]PHPCallRecord{}
	err = eachJSONLine(callsData, func(line []byte) error {
		var call PHPCallRecord
		if err := decodePHPCallRecord(line, &call); err != nil {
			return err
		}
		allCalls = append(allCalls, call)
		byTargetName[strings.ToLower(call.Name)] = append(byTargetName[strings.ToLower(call.Name)], call)
		if call.CallerQualifiedName != "" {
			key := fmt.Sprintf("%s:%d:%s", call.Path, call.CallerLine, strings.ToLower(call.CallerQualifiedName))
			byCaller[key] = append(byCaller[key], call)
		}
		return nil
	})
	if err != nil {
		return PHPCallGraphResult{}, err
	}
	if len(allCalls) != manifest.CallsIndexed {
		return PHPCallGraphResult{}, fmt.Errorf("ADM call index record count mismatch; run project_analyze")
	}
	nodes := map[string]bool{rootNode.ID: true}
	edges := map[string]bool{}
	type pending struct {
		symbol SymbolRecord
		depth  int
	}
	queue := []pending{{symbol: target, depth: 0}}
	expanded := map[string]bool{}
	add := func(from, to PHPCallGraphNode, kind, reason string, call PHPCallRecord) bool {
		edgeID := fmt.Sprintf("%s|%s|%s|%d|%d|%s", from.ID, to.ID, call.Path, call.Line, call.Column, kind)
		if edges[edgeID] {
			return false
		}
		if len(result.Edges) >= q.MaxResults {
			result.Truncated = true
			return false
		}
		edges[edgeID] = true
		if !nodes[from.ID] {
			result.Nodes = append(result.Nodes, from)
			nodes[from.ID] = true
		}
		if !nodes[to.ID] {
			result.Nodes = append(result.Nodes, to)
			nodes[to.ID] = true
		}
		result.Edges = append(result.Edges, PHPCallGraphEdge{
			From: from.ID, To: to.ID, Kind: kind, Reason: reason, TypeHint: call.TypeHint,
			Path: call.Path, Line: call.Line, Column: call.Column, Context: call.Context,
		})
		return true
	}
	lookupCaller := func(c PHPCallRecord) (PHPCallGraphNode, SymbolRecord, bool) {
		if c.CallerQualifiedName != "" {
			for _, s := range byFQN[strings.ToLower(c.CallerQualifiedName)] {
				if s.Path == c.Path && s.Line == c.CallerLine {
					return toNode(s), s, true
				}
			}
		}
		// File-level code is not falsely attached to an enclosing class.
		if c.CallerQualifiedName != "" {
			return PHPCallGraphNode{ID: fmt.Sprintf("php-pending:%s:%d", c.Path, c.CallerLine), Path: c.Path, Line: c.CallerLine, Name: c.CallerQualifiedName, Kind: "unresolved_caller"}, SymbolRecord{}, false
		}
		return PHPCallGraphNode{ID: "php-file:" + c.Path, Name: "(file scope)", Path: c.Path, Kind: "file"}, SymbolRecord{}, false
	}
	type proposedEdge struct {
		from, to     PHPCallGraphNode
		kind, reason string
		call         PHPCallRecord
		nextSymbol   SymbolRecord
		canExpand    bool
	}
	rank := func(edge proposedEdge) int {
		return phpGraphEvidencePriority(edge.kind, edge.call.TypeHint)
	}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		src := next.symbol
		srcNode := toNode(src)
		if expanded[srcNode.ID] || next.depth >= q.MaxDepth {
			continue
		}
		expanded[srcNode.ID] = true
		proposals := make([]proposedEdge, 0)
		method := src.Kind == "method"
		short := phpCallerShortName(src.QualifiedName)
		if q.Direction == "both" || q.Direction == "callers" {
			resolver, err := newPHPReferenceResolver(symbolData, PHPReferenceQuery{Name: src.QualifiedName, Kind: src.Kind, Path: src.Path})
			if err != nil {
				return PHPCallGraphResult{}, err
			}
			for _, c := range byTargetName[strings.ToLower(short)] {
				if (c.CallKind == "method") != method {
					continue
				}
				kind, reason, include := resolver.classifyIndexedCall(c)
				if !include {
					continue
				}
				from, callerSymbol, found := lookupCaller(c)
				proposals = append(proposals, proposedEdge{
					from: from, to: srcNode, kind: kind, reason: reason, call: c,
					nextSymbol: callerSymbol, canExpand: found,
				})
			}
		}
		if q.Direction == "both" || q.Direction == "callees" {
			key := fmt.Sprintf("%s:%d:%s", src.Path, src.Line, strings.ToLower(src.QualifiedName))
			for _, c := range byCaller[key] {
				kind := "candidate_call"
				reason := "dynamic_receiver"
				var dst PHPCallGraphNode
				var callee SymbolRecord
				found := false
				if c.CallKind == "method" && c.OwnerKnown {
					full := strings.ToLower(c.Owner + "::" + c.Name)
					matches := byFQN[full]
					if len(matches) == 1 {
						callee = matches[0]
						dst = toNode(callee)
						kind = "resolved_call"
						reason = "lexical_owner_matches_definition"
						found = true
					} else if c.OwnerParent != "" {
						parentMatches := byFQN[strings.ToLower(c.OwnerParent+"::"+c.Name)]
						if len(parentMatches) == 1 {
							dst = toNode(parentMatches[0])
							kind = "inherited_candidate"
							reason = "direct_extends_clause"
						}
					}
				} else if c.CallKind == "method" && c.TypeHint != "" {
					hintMatches := byFQN[strings.ToLower(c.TypeHint+"::"+c.Name)]
					if len(hintMatches) == 1 {
						dst = toNode(hintMatches[0])
						reason = "declared_parameter_type_not_runtime"
					}
				} else if c.CallKind == "function" {
					// Without expression/type analysis a namespaced function call has
					// multiple possible targets; resolve only one globally unique name.
					matches := byName[strings.ToLower(c.Name)]
					if len(matches) == 1 && matches[0].Kind == "function" {
						callee = matches[0]
						dst = toNode(callee)
						kind = "candidate_call"
						reason = "unique_function_name_not_namespace_resolved"
					}
				}
				if dst.ID == "" {
					dst = PHPCallGraphNode{ID: fmt.Sprintf("php-unresolved:%s:%s", c.CallKind, strings.ToLower(c.Name)),
						Name: c.Name, Kind: "unresolved_" + c.CallKind}
				}
				proposals = append(proposals, proposedEdge{
					from: srcNode, to: dst, kind: kind, reason: reason, call: c,
					nextSymbol: callee, canExpand: found,
				})
			}
		}
		// Rank *before* applying the result cap. Otherwise a directory full
		// of uncertain calls can hide known callers or callees from the AI.
		sort.SliceStable(proposals, func(i, j int) bool {
			a, b := proposals[i], proposals[j]
			if rank(a) != rank(b) {
				return rank(a) < rank(b)
			}
			if a.call.Path != b.call.Path {
				return a.call.Path < b.call.Path
			}
			if a.call.Line != b.call.Line {
				return a.call.Line < b.call.Line
			}
			if a.call.Column != b.call.Column {
				return a.call.Column < b.call.Column
			}
			if a.from.ID != b.from.ID {
				return a.from.ID < b.from.ID
			}
			return a.to.ID < b.to.ID
		})
		for _, item := range proposals {
			if add(item.from, item.to, item.kind, item.reason, item.call) &&
				item.canExpand && item.kind == "resolved_call" {
				queue = append(queue, pending{symbol: item.nextSymbol, depth: next.depth + 1})
			}
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool {
		priorityI := phpGraphEvidencePriority(result.Edges[i].Kind, result.Edges[i].TypeHint)
		priorityJ := phpGraphEvidencePriority(result.Edges[j].Kind, result.Edges[j].TypeHint)
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}
		if result.Edges[i].Path != result.Edges[j].Path {
			return result.Edges[i].Path < result.Edges[j].Path
		}
		if result.Edges[i].Line != result.Edges[j].Line {
			return result.Edges[i].Line < result.Edges[j].Line
		}
		if result.Edges[i].Column != result.Edges[j].Column {
			return result.Edges[i].Column < result.Edges[j].Column
		}
		return result.Edges[i].From < result.Edges[j].From
	})
	result.Returned = len(result.Edges)
	return result, nil
}
