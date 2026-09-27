package projectanalysis

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IndexQuery struct {
	Query      string `json:"query,omitempty"`
	Path       string `json:"path,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Language   string `json:"language,omitempty"`
	Exact      bool   `json:"exact,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type IndexQueryMatch struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Namespace     string `json:"namespace,omitempty"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name,omitempty"`
	Line          int    `json:"line"`
	Match         string `json:"match"`
}

type IndexQueryResult struct {
	IndexPath        string            `json:"index_path"`
	GeneratedAt      string            `json:"generated_at"`
	SchemaVersion    int               `json:"schema_version"`
	ArtifactVerified bool              `json:"artifact_verified"`
	Matches          []IndexQueryMatch `json:"matches"`
	Returned         int               `json:"returned"`
	Truncated        bool              `json:"truncated,omitempty"`
}

type scoredIndexMatch struct {
	match IndexQueryMatch
	score int
}

func QueryIndex(root string, query IndexQuery) (IndexQueryResult, error) {
	query.Query = strings.TrimSpace(query.Query)
	query.Path = filepath.ToSlash(strings.TrimSpace(query.Path))
	query.Kind = strings.TrimSpace(query.Kind)
	query.Language = strings.TrimSpace(query.Language)
	if query.Query == "" && query.Path == "" && query.Kind == "" && query.Language == "" {
		return IndexQueryResult{}, fmt.Errorf("query, path, kind, or language is required")
	}
	if query.MaxResults <= 0 {
		query.MaxResults = 50
	}
	if query.MaxResults > 200 {
		query.MaxResults = 200
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(IndexManifestRelativePath))
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return IndexQueryResult{}, fmt.Errorf("project index is missing; run project_analyze first")
		}
		return IndexQueryResult{}, err
	}
	var manifest IndexManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return IndexQueryResult{}, fmt.Errorf("decode project index manifest: %w", err)
	}
	if manifest.SchemaVersion != IndexSchemaVersion {
		return IndexQueryResult{}, fmt.Errorf("unsupported project index schema_version=%d; run project_analyze to refresh", manifest.SchemaVersion)
	}
	artifact, ok := manifest.Artifacts["symbols"]
	if !ok || artifact.Path != IndexSymbolsRelativePath || artifact.SHA256 == "" {
		return IndexQueryResult{}, fmt.Errorf("project symbol index manifest entry is invalid; run project_analyze to refresh")
	}

	symbolPath := filepath.Join(root, filepath.FromSlash(IndexSymbolsRelativePath))
	symbolData, err := os.ReadFile(symbolPath)
	if err != nil {
		if os.IsNotExist(err) {
			return IndexQueryResult{}, fmt.Errorf("project symbol index is missing; run project_analyze to refresh")
		}
		return IndexQueryResult{}, err
	}
	sum := sha256.Sum256(symbolData)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), artifact.SHA256) {
		return IndexQueryResult{}, fmt.Errorf("project symbol index hash does not match manifest; run project_analyze to refresh")
	}

	pathFilter := strings.ToLower(query.Path)
	kindFilter := strings.ToLower(query.Kind)
	languageFilter := strings.ToLower(query.Language)
	var matches []scoredIndexMatch
	scanner := bufio.NewScanner(strings.NewReader(string(symbolData)))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record SymbolRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return IndexQueryResult{}, fmt.Errorf("decode project symbol index: %w", err)
		}
		if pathFilter != "" && !strings.Contains(strings.ToLower(filepath.ToSlash(record.Path)), pathFilter) {
			continue
		}
		if kindFilter != "" && strings.ToLower(record.Kind) != kindFilter {
			continue
		}
		if languageFilter != "" && strings.ToLower(record.Language) != languageFilter {
			continue
		}
		score, matchKind, matched := scoreSymbolQuery(record, query.Query, query.Exact)
		if !matched {
			continue
		}
		matches = append(matches, scoredIndexMatch{
			score: score,
			match: IndexQueryMatch{
				Path: record.Path, Language: record.Language, Namespace: record.Namespace,
				Kind: record.Kind, Name: record.Name, QualifiedName: record.QualifiedName,
				Line: record.Line, Match: matchKind,
			},
		})
	}
	if err := scanner.Err(); err != nil {
		return IndexQueryResult{}, fmt.Errorf("read project symbol index: %w", err)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		if matches[i].match.QualifiedName != matches[j].match.QualifiedName {
			return matches[i].match.QualifiedName < matches[j].match.QualifiedName
		}
		if matches[i].match.Path != matches[j].match.Path {
			return matches[i].match.Path < matches[j].match.Path
		}
		return matches[i].match.Line < matches[j].match.Line
	})
	truncated := len(matches) > query.MaxResults
	if truncated {
		matches = matches[:query.MaxResults]
	}
	result := IndexQueryResult{
		IndexPath:        IndexSymbolsRelativePath,
		GeneratedAt:      manifest.GeneratedAt,
		SchemaVersion:    manifest.SchemaVersion,
		ArtifactVerified: true,
		Returned:         len(matches),
		Truncated:        truncated,
	}
	result.Matches = make([]IndexQueryMatch, 0, len(matches))
	for _, item := range matches {
		result.Matches = append(result.Matches, item.match)
	}
	return result, nil
}

func scoreSymbolQuery(record SymbolRecord, query string, exact bool) (int, string, bool) {
	if query == "" {
		return 10, "filter", true
	}
	q := strings.ToLower(query)
	name := strings.ToLower(record.Name)
	qualified := strings.ToLower(record.QualifiedName)
	if qualified == q {
		return 0, "qualified_exact", true
	}
	if name == q {
		return 1, "name_exact", true
	}
	if exact {
		return 0, "", false
	}
	if strings.HasSuffix(qualified, "."+q) || strings.HasSuffix(qualified, "\\"+q) {
		return 2, "qualified_suffix", true
	}
	if strings.HasPrefix(name, q) {
		return 3, "name_prefix", true
	}
	if strings.HasPrefix(qualified, q) {
		return 4, "qualified_prefix", true
	}
	if strings.Contains(name, q) {
		return 5, "name_contains", true
	}
	if strings.Contains(qualified, q) {
		return 6, "qualified_contains", true
	}
	return 0, "", false
}
