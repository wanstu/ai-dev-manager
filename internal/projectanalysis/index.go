package projectanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	OverviewRelativePath      = ".adm/project-overview.md"
	IndexManifestRelativePath = ".adm/index/manifest.json"
	IndexFilesRelativePath    = ".adm/index/files.jsonl"
	IndexSymbolsRelativePath  = ".adm/index/symbols.jsonl"
	IndexCallsRelativePath    = ".adm/index/calls.jsonl"
	IndexSchemaVersion        = 5
)

type IndexBounds struct {
	MaxFiles   int `json:"max_files"`
	MaxSymbols int `json:"max_symbols"`
}

type IndexArtifact struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type IndexManifest struct {
	IndexMode             string                   `json:"index_mode,omitempty"`
	ReusedSourceFiles     int                      `json:"reused_source_files,omitempty"`
	ReindexedSourceFiles  int                      `json:"reindexed_source_files,omitempty"`
	CallsIndexed          int                      `json:"calls_indexed"`
	ReusedPHPCallFiles    int                      `json:"reused_php_call_files,omitempty"`
	ReindexedPHPCallFiles int                      `json:"reindexed_php_call_files,omitempty"`
	SchemaVersion         int                      `json:"schema_version"`
	GeneratedAt           string                   `json:"generated_at"`
	Languages             []string                 `json:"languages"`
	FilesScanned          int                      `json:"files_scanned"`
	FilesIndexed          int                      `json:"files_indexed"`
	SymbolsIndexed        int                      `json:"symbols_indexed"`
	GoFiles               int                      `json:"go_files"`
	PHPFiles              int                      `json:"php_files"`
	JSFiles               int                      `json:"js_files"`
	TSFiles               int                      `json:"ts_files"`
	GoModule              string                   `json:"go_module,omitempty"`
	ComposerPackage       string                   `json:"composer_package,omitempty"`
	ParseIssues           int                      `json:"parse_issues,omitempty"`
	Truncated             bool                     `json:"truncated,omitempty"`
	Bounds                IndexBounds              `json:"bounds"`
	Artifacts             map[string]IndexArtifact `json:"artifacts"`
}

type FileRecord struct {
	SymbolsComplete bool   `json:"symbols_complete,omitempty"`
	CallsComplete   bool   `json:"calls_complete,omitempty"`
	Path            string `json:"path"`
	Extension       string `json:"extension,omitempty"`
	Language        string `json:"language,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Size            int64  `json:"size"`
	ModifiedAt      string `json:"modified_at,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
}

type SymbolRecord struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Namespace     string `json:"namespace,omitempty"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name,omitempty"`
	Line          int    `json:"line"`
}

func newFileRecord(path, extension string, info os.FileInfo) FileRecord {
	record := FileRecord{
		Path:      filepath.ToSlash(path),
		Extension: strings.TrimPrefix(strings.ToLower(extension), "."),
	}
	if info != nil {
		record.Size = info.Size()
		if !info.ModTime().IsZero() {
			record.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339Nano)
		}
	}
	return record
}

func sourceHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func symbolRecordsForOutline(outline fileOutline) []SymbolRecord {
	records := make([]SymbolRecord, 0, len(outline.Symbols))
	for _, item := range outline.Symbols {
		qualified := item.Name
		if outline.Namespace != "" {
			separator := "."
			if outline.Language == "PHP" {
				separator = "\\"
			}
			qualified = outline.Namespace + separator + item.Name
		}
		records = append(records, SymbolRecord{
			Path:          outline.Path,
			Language:      outline.Language,
			Namespace:     outline.Namespace,
			Kind:          item.Kind,
			Name:          item.Name,
			QualifiedName: qualified,
			Line:          item.Line,
		})
	}
	return records
}

func encodeJSONLines[T any](records []T) (string, error) {
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func artifactFor(path, content string) IndexArtifact {
	sum := sha256.Sum256([]byte(content))
	return IndexArtifact{
		Path:   path,
		Bytes:  len(content),
		SHA256: hex.EncodeToString(sum[:]),
	}
}

func buildIndexArtifacts(result Result, options Options, files []FileRecord, symbols []SymbolRecord, calls []PHPCallRecord) (string, string, string, string, error) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Path != symbols[j].Path {
			return symbols[i].Path < symbols[j].Path
		}
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].QualifiedName < symbols[j].QualifiedName
	})

	filesJSONL, err := encodeJSONLines(files)
	if err != nil {
		return "", "", "", "", fmt.Errorf("encode file index: %w", err)
	}
	symbolsJSONL, err := encodeJSONLines(symbols)
	if err != nil {
		return "", "", "", "", fmt.Errorf("encode symbol index: %w", err)
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Path != calls[j].Path {
			return calls[i].Path < calls[j].Path
		}
		if calls[i].Line != calls[j].Line {
			return calls[i].Line < calls[j].Line
		}
		if calls[i].Column != calls[j].Column {
			return calls[i].Column < calls[j].Column
		}
		return calls[i].Name < calls[j].Name
	})
	callsJSONL, err := encodeJSONLines(calls)
	if err != nil {
		return "", "", "", "", fmt.Errorf("encode call index: %w", err)
	}

	manifest := IndexManifest{
		IndexMode:             result.IndexMode,
		ReusedSourceFiles:     result.ReusedSourceFiles,
		ReindexedSourceFiles:  result.ReindexedSourceFiles,
		CallsIndexed:          result.CallsIndexed,
		ReusedPHPCallFiles:    result.ReusedPHPCallFiles,
		ReindexedPHPCallFiles: result.ReindexedPHPCallFiles,
		SchemaVersion:         IndexSchemaVersion,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		Languages:             append([]string(nil), result.Languages...),
		FilesScanned:          result.FilesScanned,
		FilesIndexed:          len(files),
		SymbolsIndexed:        len(symbols),
		GoFiles:               result.GoFiles,
		PHPFiles:              result.PHPFiles,
		JSFiles:               result.JSFiles,
		TSFiles:               result.TSFiles,
		GoModule:              result.GoModule,
		ComposerPackage:       result.ComposerPackage,
		ParseIssues:           result.ParseIssues,
		Truncated:             result.Truncated,
		Bounds: IndexBounds{
			MaxFiles:   options.MaxFiles,
			MaxSymbols: options.MaxSymbols,
		},
		Artifacts: map[string]IndexArtifact{
			"overview": artifactFor(OverviewRelativePath, result.Markdown),
			"files":    artifactFor(IndexFilesRelativePath, filesJSONL),
			"symbols":  artifactFor(IndexSymbolsRelativePath, symbolsJSONL),
			"calls":    artifactFor(IndexCallsRelativePath, callsJSONL),
		},
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", "", "", "", fmt.Errorf("encode index manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	return string(manifestJSON), filesJSONL, symbolsJSONL, callsJSONL, nil
}
