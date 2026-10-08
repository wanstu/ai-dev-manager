package projectanalysis

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ReferenceCandidate struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Context string `json:"context,omitempty"`
}

type ReferenceCandidateResult struct {
	References []ReferenceCandidate `json:"references"`
	Truncated  bool                 `json:"truncated,omitempty"`
}

func FindPHPCallCandidates(root, symbolName, kind string, maxResults int) (ReferenceCandidateResult, error) {
	return FindPHPCallReferences(root, PHPReferenceQuery{Name: symbolName, Kind: kind, MaxResults: maxResults})
}

// FindPHPCallReferences uses only persisted, hash-verified index artifacts.
// It does not read or parse PHP source files. Source freshness is checked with
// filesystem metadata; use project_index_status for full content-hash checks.
func FindPHPCallReferences(root string, query PHPReferenceQuery) (ReferenceCandidateResult, error) {
	maxResults := query.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}
	if maxResults > 500 {
		maxResults = 500
	}
	if query.Kind != "" && query.Kind != "method" && query.Kind != "function" {
		return ReferenceCandidateResult{}, fmt.Errorf("PHP references require method or function")
	}
	name := strings.TrimSpace(query.Name)
	isMethod := query.Kind == "method" || strings.Contains(name, "::")
	if pos := strings.LastIndex(name, "::"); pos >= 0 {
		name = name[pos+2:]
	}
	if pos := strings.LastIndex(name, "\\"); pos >= 0 {
		name = name[pos+1:]
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return ReferenceCandidateResult{}, fmt.Errorf("PHP reference name is invalid")
	}
	manifest, filesData, symbolsData, callsData, err := readVerifiedPHPCallIndex(root)
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	if err := verifyIndexMetadata(root, manifest, filesData); err != nil {
		return ReferenceCandidateResult{}, fmt.Errorf("ADM project index is stale: %w; run project_analyze", err)
	}
	resolver, err := newPHPReferenceResolver(symbolsData, query)
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	result := ReferenceCandidateResult{
		References: make([]ReferenceCandidate, 0),
		Truncated:  manifest.Truncated || manifest.ParseIssues > 0,
	}
	var parsed int
	err = eachJSONLine(callsData, func(line []byte) error {
		var call PHPCallRecord
		if err := decodePHPCallRecord(line, &call); err != nil {
			return err
		}
		parsed++
		if !strings.EqualFold(call.Name, name) {
			return nil
		}
		if isMethod && call.CallKind != "method" || !isMethod && call.CallKind != "function" {
			return nil
		}
		kind := "candidate_call"
		if isMethod && resolver.targetFQN != "" && call.OwnerKnown {
			if strings.EqualFold(call.Owner, resolver.targetClass) {
				kind = "resolved_call"
			} else if resolver.directOwners[strings.ToLower(call.Owner)] {
				return nil // This method belongs to a different indexed class.
			}
		}
		if len(result.References) >= maxResults {
			result.Truncated = true
			return nil
		}
		result.References = append(result.References, ReferenceCandidate{
			Path: call.Path, Line: call.Line, Column: call.Column,
			Name: call.Name, Kind: kind, Context: call.Context,
		})
		return nil
	})
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	if parsed != manifest.CallsIndexed {
		return ReferenceCandidateResult{}, fmt.Errorf("ADM call index record count mismatch; run project_analyze")
	}
	return result, nil
}

func phpReferenceContext(data []byte, lineStart, lineEnd, callStart, callEnd int) string {
	start := max(lineStart, callStart-120)
	end := min(lineEnd, callEnd+120)
	for start < end && !utf8.RuneStart(data[start]) {
		start++
	}
	for end > start && end < len(data) && !utf8.RuneStart(data[end]) {
		end--
	}
	value := strings.TrimSpace(string(data[start:end]))
	if start > lineStart {
		value = "…" + value
	}
	if end < lineEnd {
		value += "…"
	}
	return value
}
