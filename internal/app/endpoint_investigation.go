package app

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ai-dev-manager-v2/internal/model"
)

const (
	endpointConfidenceNone   = "none"
	endpointConfidenceLow    = "low"
	endpointConfidenceMedium = "medium"
	endpointConfidenceHigh   = "high"
)

var dynamicEndpointSegment = regexp.MustCompile(`(?i)^(\d+|[0-9a-f]{8,}|[0-9a-f]{8}-[0-9a-f-]{13,})$`)

// InvestigateEndpoint returns bounded static route-like evidence. It does not
// execute project code, call a network endpoint, run a verifier, or mutate ADM
// state.
func (s *Service) InvestigateEndpoint(environmentID string, request model.EndpointInvestigationRequest) (model.EndpointInvestigationReport, error) {
	rt, _, err := s.Runtime(environmentID)
	if err != nil {
		return model.EndpointInvestigationReport{}, err
	}
	normalized, err := normalizeEndpointTarget(request.Target)
	if err != nil {
		return model.EndpointInvestigationReport{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	queries := endpointQueries(normalized)
	maxFiles, maxMatches, maxBytes := endpointLimits(request)
	searchRoot, searchLabel, err := endpointSearchRoot(rt.Root(), request.Path)
	if err != nil {
		return model.EndpointInvestigationReport{}, err
	}

	report := model.EndpointInvestigationReport{
		EnvironmentID:  environmentID,
		Target:         request.Target,
		NormalizedPath: normalized,
		Method:         method,
		SearchPath:     searchLabel,
		Queries:        queries,
		Confidence:     endpointConfidenceNone,
	}
	if len(queries) == 0 {
		report.Uncertainties = append(report.Uncertainties, "no_search_queries_generated")
		return report, nil
	}

	evidence, truncated, err := collectEndpointEvidence(rt.Root(), searchRoot, queries, method, maxFiles, maxMatches, maxBytes)
	if err != nil {
		return model.EndpointInvestigationReport{}, err
	}
	report.Evidence = evidence
	report.Truncated = truncated
	report.Confidence = endpointConfidence(evidence)
	report.Uncertainties = endpointUncertainties(report, method, truncated)
	return report, nil
}

func normalizeEndpointTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("endpoint target is required")
	}
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		target = parsed.EscapedPath()
		if target == "" {
			target = "/"
		}
	} else {
		if i := strings.IndexAny(target, "?#"); i >= 0 {
			target = target[:i]
		}
	}
	if unescaped, err := url.PathUnescape(target); err == nil {
		target = unescaped
	}
	target = strings.TrimSpace(target)
	if target == "" {
		target = "/"
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	for strings.Contains(target, "//") {
		target = strings.ReplaceAll(target, "//", "/")
	}
	if len(target) > 1 {
		target = strings.TrimRight(target, "/")
		if target == "" {
			target = "/"
		}
	}
	return target, nil
}

func endpointQueries(path string) []string {
	seen := map[string]bool{}
	var queries []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		queries = append(queries, value)
	}
	add(path)
	if path != "/" {
		add(strings.TrimPrefix(path, "/"))
	}
	for _, dynamic := range endpointDynamicQueries(path) {
		add(dynamic)
		if dynamic != "/" {
			add(strings.TrimPrefix(dynamic, "/"))
		}
	}
	return queries
}

func endpointDynamicQueries(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	segments := strings.Split(trimmed, "/")
	dynamicIndexes := make([]int, 0)
	for i, segment := range segments {
		if endpointLooksDynamic(segment) {
			dynamicIndexes = append(dynamicIndexes, i)
		}
	}
	if len(dynamicIndexes) == 0 {
		return nil
	}
	placeholders := []string{":id", "{id}", "<id>", "[id]"}
	var out []string
	for _, placeholder := range placeholders {
		copySegments := append([]string(nil), segments...)
		for _, index := range dynamicIndexes {
			copySegments[index] = placeholder
		}
		out = append(out, "/"+strings.Join(copySegments, "/"))
	}
	return out
}

func endpointLooksDynamic(segment string) bool {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return false
	}
	if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "{") || strings.HasPrefix(segment, "<") || strings.HasPrefix(segment, "[") {
		return false
	}
	return dynamicEndpointSegment.MatchString(segment)
}

func endpointLimits(request model.EndpointInvestigationRequest) (int, int, int) {
	maxFiles := request.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 2000
	}
	maxMatches := request.MaxMatches
	if maxMatches <= 0 {
		maxMatches = 100
	}
	maxBytes := request.MaxBytesPerFile
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return maxFiles, maxMatches, maxBytes
}

func endpointSearchRoot(root, path string) (string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return root, "", nil
	}
	if filepath.IsAbs(path) {
		return "", "", fmt.Errorf("search path must be environment-relative")
	}
	candidate := filepath.Clean(filepath.Join(root, path))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", err
	}
	if !endpointWithin(root, resolved) {
		return "", "", fmt.Errorf("search path escapes environment root")
	}
	return filepath.Clean(resolved), filepath.ToSlash(filepath.Clean(path)), nil
}

func collectEndpointEvidence(root, searchRoot string, queries []string, method string, maxFiles, maxMatches, maxBytes int) ([]model.EndpointInvestigationEvidence, bool, error) {
	queryKind := make(map[string]string, len(queries))
	for i, query := range queries {
		kind := "route_dynamic_candidate"
		if i == 0 || query == strings.TrimPrefix(queries[0], "/") {
			kind = "route_literal"
		}
		queryKind[query] = kind
	}
	var evidence []model.EndpointInvestigationEvidence
	files := 0
	truncated := false

	walkOne := func(target string) error {
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > int64(maxBytes) {
			return nil
		}
		files++
		if files > maxFiles {
			truncated = true
			return io.EOF
		}
		fileEvidence, err := endpointEvidenceInFile(root, target, queries, queryKind, method, maxMatches-len(evidence), maxBytes)
		if err != nil {
			return err
		}
		evidence = append(evidence, fileEvidence...)
		if len(evidence) >= maxMatches {
			truncated = true
			return io.EOF
		}
		return nil
	}

	info, err := os.Stat(searchRoot)
	if err != nil {
		return nil, false, err
	}
	if info.Mode().IsRegular() {
		err = walkOne(searchRoot)
	} else {
		err = filepath.WalkDir(searchRoot, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if current != searchRoot && endpointSkipDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			return walkOne(current)
		})
	}
	if err != nil && !errorsIsEOF(err) {
		return nil, false, err
	}
	sort.SliceStable(evidence, func(i, j int) bool {
		if evidence[i].Score != evidence[j].Score {
			return evidence[i].Score > evidence[j].Score
		}
		if evidence[i].Path != evidence[j].Path {
			return evidence[i].Path < evidence[j].Path
		}
		return evidence[i].Line < evidence[j].Line
	})
	return evidence, truncated, nil
}

func endpointEvidenceInFile(root, target string, queries []string, queryKind map[string]string, method string, maxMatches, maxBytes int) ([]model.EndpointInvestigationEvidence, error) {
	if maxMatches <= 0 {
		return nil, nil
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, nil
	}
	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxBytes)
	line := 0
	var out []model.EndpointInvestigationEvidence
	for scanner.Scan() {
		line++
		text := scanner.Text()
		for _, query := range queries {
			if !strings.Contains(text, query) {
				continue
			}
			rel, _ := filepath.Rel(root, target)
			score, reasons := endpointEvidenceScore(text, query, method, rel)
			out = append(out, model.EndpointInvestigationEvidence{
				Kind:    queryKind[query],
				Path:    filepath.ToSlash(rel),
				Line:    line,
				Text:    strings.TrimSpace(text),
				Matched: query,
				Score:   score,
				Reasons: reasons,
			})
			if len(out) >= maxMatches {
				return out, nil
			}
			break
		}
	}
	return out, scanner.Err()
}

func endpointEvidenceScore(text, query, method, rel string) (int, []string) {
	score := 40
	reasons := []string{"route_literal_match"}
	if strings.Contains(query, ":id") || strings.Contains(query, "{id}") || strings.Contains(query, "<id>") || strings.Contains(query, "[id]") {
		score = 30
		reasons = []string{"dynamic_route_candidate_match"}
	}
	if method != "" && endpointLineHasMethod(text, method) {
		score += 35
		reasons = append(reasons, "method_on_same_line")
	} else if method != "" {
		reasons = append(reasons, "method_not_on_same_line")
	}
	lowerPath := strings.ToLower(filepath.ToSlash(rel))
	for _, hint := range []string{"route", "routes", "router", "controller", "handler", "api"} {
		if strings.Contains(lowerPath, hint) {
			score += 10
			reasons = append(reasons, "path_hint_"+hint)
			break
		}
	}
	if strings.Contains(text, "TODO") || strings.Contains(text, "test") {
		score -= 5
	}
	return score, reasons
}

func endpointLineHasMethod(text, method string) bool {
	upper := strings.ToUpper(text)
	method = strings.ToUpper(method)
	for _, token := range []string{
		method,
		"METHOD" + method,
		"METHOD_" + method,
		"HTTP.METHOD" + method,
		"HTTPMETHOD" + method,
		"HTTP_METHOD_" + method,
	} {
		if strings.Contains(strings.ReplaceAll(upper, " ", ""), token) {
			return true
		}
	}
	return strings.Contains(upper, `"`+method+`"`) || strings.Contains(upper, `'`+method+`'`)
}

func endpointConfidence(evidence []model.EndpointInvestigationEvidence) string {
	if len(evidence) == 0 {
		return endpointConfidenceNone
	}
	top := evidence[0].Score
	if top >= 80 {
		return endpointConfidenceHigh
	}
	if top >= 55 {
		return endpointConfidenceMedium
	}
	return endpointConfidenceLow
}

func endpointUncertainties(report model.EndpointInvestigationReport, method string, truncated bool) []string {
	var uncertainties []string
	if len(report.Evidence) == 0 {
		uncertainties = append(uncertainties, "no_route_literal_evidence_found")
	}
	if method != "" {
		methodMatched := false
		for _, item := range report.Evidence {
			for _, reason := range item.Reasons {
				if reason == "method_on_same_line" {
					methodMatched = true
				}
			}
		}
		if !methodMatched {
			uncertainties = append(uncertainties, "method_not_confirmed")
		}
	}
	if truncated {
		uncertainties = append(uncertainties, "result_truncated_by_limits")
	}
	uncertainties = append(uncertainties, "static_literal_search_only")
	uncertainties = append(uncertainties, "framework_specific_registration_may_be_indirect")
	return uncertainties
}

func endpointSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", "target", "bin", "obj", "coverage", "tmp", "temp":
		return true
	default:
		return false
	}
}

func endpointWithin(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func errorsIsEOF(err error) bool { return err == io.EOF }
