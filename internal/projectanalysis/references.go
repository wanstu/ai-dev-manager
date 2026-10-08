package projectanalysis

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ReferenceCandidate is a lexical call-site candidate, not a semantically
// resolved reference. PHP's dynamic dispatch prevents exact attribution.
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

// FindPHPCallCandidates reuses only ADM's own verified project index and
// examines PHP source without executing any of it. Static receivers can be
// resolved to one indexed method; dynamic calls remain candidates.
func FindPHPCallCandidates(root, symbolName, kind string, maxResults int) (ReferenceCandidateResult, error) {
	return FindPHPCallReferences(root, PHPReferenceQuery{Name: symbolName, Kind: kind, MaxResults: maxResults})
}

// FindPHPCallReferences additionally uses an optional definition path to
// disambiguate same-named methods in distinct namespaces and files.
func FindPHPCallReferences(root string, query PHPReferenceQuery) (ReferenceCandidateResult, error) {
	symbolName, kind, maxResults := query.Name, query.Kind, query.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}
	if maxResults > 500 {
		maxResults = 500
	}
	if kind != "" && kind != "method" && kind != "function" {
		return ReferenceCandidateResult{}, fmt.Errorf("PHP call candidates require a method or function symbol")
	}
	name := strings.TrimSpace(symbolName)
	isMethod := kind == "method" || strings.Contains(name, "::")
	if pos := strings.LastIndex(name, "::"); pos >= 0 {
		name = name[pos+2:]
	}
	if pos := strings.LastIndex(name, "\\"); pos >= 0 {
		name = name[pos+1:]
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		return ReferenceCandidateResult{}, fmt.Errorf("PHP call candidate symbol name is invalid")
	}
	filesData, symbolsData, err := readVerifiedPHPIndex(root)
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	status, err := IndexStatus(root, 1)
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	if status.State != "fresh" && status.State != "partial" {
		return ReferenceCandidateResult{}, fmt.Errorf("ADM project index is %s; run project_analyze before querying PHP references", status.State)
	}
	resolver, err := newPHPReferenceResolver(symbolsData, query)
	if err != nil {
		return ReferenceCandidateResult{}, err
	}
	var matcher *regexp.Regexp
	if isMethod {
		matcher = regexp.MustCompile(`(?i)(?:->|::)[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*\(`)
	} else {
		matcher = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `[ \t]*\(`)
	}
	result := ReferenceCandidateResult{
		References: make([]ReferenceCandidate, 0),
		Truncated:  status.State == "partial",
	}
	scanner := bufio.NewScanner(strings.NewReader(string(filesData)))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var record FileRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return ReferenceCandidateResult{}, fmt.Errorf("invalid file index record: %w", err)
		}
		if record.Language != "PHP" {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(record.Path))
		if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return ReferenceCandidateResult{}, fmt.Errorf("invalid indexed source path")
		}
		full := filepath.Join(root, clean)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			return ReferenceCandidateResult{}, fmt.Errorf("indexed PHP source unavailable or not regular: %s", record.Path)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return ReferenceCandidateResult{}, err
		}
		if record.SHA256 == "" {
			return ReferenceCandidateResult{}, fmt.Errorf("indexed PHP source has no content hash: %s", record.Path)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), record.SHA256) {
			return ReferenceCandidateResult{}, fmt.Errorf("indexed PHP source is stale: %s", record.Path)
		}
		if len(data) > 1<<20 {
			data = data[:1<<20]
			result.Truncated = true
		}
		masked := maskSource(data, true)
		caller := newPHPCallerFile(masked)
		for _, m := range matcher.FindAllIndex(masked, -1) {
			start := m[0]
			referenceKind, include := resolver.classifyCall(caller, masked, start, isMethod)
			if !include {
				continue
			}
			lineStart := bytes.LastIndexByte(data[:start], '\n') + 1
			lineEnd := start
			for lineEnd < len(data) && data[lineEnd] != '\n' {
				lineEnd++
			}
			if !isMethod {
				prefix := strings.TrimSpace(string(masked[lineStart:start]))
				if regexp.MustCompile(`(?i)(?:^|\s)function\s*$`).MatchString(prefix) {
					continue
				}
			}
			if len(result.References) >= maxResults {
				result.Truncated = true
				continue
			}
			// Report the symbol's start column, not the operator or any
			// intervening whitespace (e.g. "Foo :: method()").
			nameStart := start
			if isMethod {
				nameStart += 2
				for nameStart < len(masked) && (masked[nameStart] == ' ' || masked[nameStart] == '\t') {
					nameStart++
				}
			}
			result.References = append(result.References, ReferenceCandidate{
				Path: filepath.ToSlash(record.Path), Line: lineAt(masked, start),
				Column: nameStart + 1 - lineStart, Name: name, Kind: referenceKind,
				Context: phpReferenceContext(data, lineStart, lineEnd, start, m[1]),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return ReferenceCandidateResult{}, err
	}
	return result, nil
}

func phpReferenceContext(data []byte, lineStart, lineEnd, callStart, callEnd int) string {
	start := max(lineStart, callStart-120)
	end := min(lineEnd, callEnd+120)
	// Preserve valid UTF-8 when clipping a long line.
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

func readVerifiedPHPIndex(root string) ([]byte, []byte, error) {
	manifestData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(IndexManifestRelativePath)))
	if err != nil {
		return nil, nil, fmt.Errorf("ADM project index is missing; run project_analyze first: %w", err)
	}
	var manifest IndexManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil || manifest.SchemaVersion != IndexSchemaVersion {
		return nil, nil, fmt.Errorf("ADM project index manifest is invalid or unsupported")
	}
	var data [2][]byte
	for i, item := range []struct{ key, path string }{
		{"files", IndexFilesRelativePath},
		{"symbols", IndexSymbolsRelativePath},
	} {
		artifact := manifest.Artifacts[item.key]
		if artifact.Path != item.path || artifact.SHA256 == "" {
			return nil, nil, fmt.Errorf("ADM project index %s artifact is invalid", item.key)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.path)))
		if err != nil {
			return nil, nil, err
		}
		hash := sha256.Sum256(content)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), artifact.SHA256) {
			return nil, nil, fmt.Errorf("ADM project index %s hash mismatch", item.key)
		}
		data[i] = content
	}
	return data[0], data[1], nil
}
