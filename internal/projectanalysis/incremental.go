package projectanalysis

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type indexSnapshot struct {
	files   map[string]FileRecord
	symbols map[string][]SymbolRecord
	calls   map[string][]PHPCallRecord
}

func normalizedAnalyzeOptions(options Options) Options {
	if options.MaxFiles <= 0 {
		options.MaxFiles = 4000
	}
	if options.MaxSymbols <= 0 {
		options.MaxSymbols = 1200
	}
	return options
}

// AnalyzeIncremental reuses only verified symbol records for unchanged source
// files. Any missing, damaged, old-schema, partial, or differently bounded
// index causes a full rebuild. It does not depend on an IDE or other service.
func AnalyzeIncremental(root string, options Options) (Result, error) {
	options = normalizedAnalyzeOptions(options)
	cache := loadIndexSnapshot(root, options)
	return analyze(root, options, cache)
}

func loadIndexSnapshot(root string, options Options) *indexSnapshot {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(IndexManifestRelativePath)))
	if err != nil {
		return nil
	}
	var manifest IndexManifest
	if json.Unmarshal(raw, &manifest) != nil ||
		manifest.SchemaVersion != IndexSchemaVersion ||
		manifest.Bounds.MaxFiles != options.MaxFiles ||
		manifest.Bounds.MaxSymbols != options.MaxSymbols {
		return nil
	}

	verified := make(map[string][]byte, 4)
	for _, item := range []struct{ key, path string }{
		{"overview", OverviewRelativePath},
		{"files", IndexFilesRelativePath},
		{"symbols", IndexSymbolsRelativePath},
		{"calls", IndexCallsRelativePath},
	} {
		artifact, ok := manifest.Artifacts[item.key]
		if !ok || artifact.Path != item.path || artifact.SHA256 == "" {
			return nil
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.path)))
		if err != nil || len(data) != artifact.Bytes {
			return nil
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), artifact.SHA256) {
			return nil
		}
		verified[item.key] = data
	}

	snapshot := &indexSnapshot{
		files:   make(map[string]FileRecord, manifest.FilesIndexed),
		symbols: make(map[string][]SymbolRecord),
		calls:   make(map[string][]PHPCallRecord),
	}
	scanner := bufio.NewScanner(strings.NewReader(string(verified["files"])))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		var record FileRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.Path == "" {
			return nil
		}
		path := filepath.ToSlash(record.Path)
		if path == "." || path == ".." || filepath.IsAbs(filepath.FromSlash(path)) ||
			strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
			return nil
		}
		if _, exists := snapshot.files[path]; exists {
			return nil
		}
		if record.Language != "" && record.SHA256 == "" {
			return nil
		}
		snapshot.files[path] = record
	}
	if scanner.Err() != nil || len(snapshot.files) != manifest.FilesIndexed {
		return nil
	}
	scanner = bufio.NewScanner(strings.NewReader(string(verified["symbols"])))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	count := 0
	for scanner.Scan() {
		var record SymbolRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.Path == "" || record.Line <= 0 {
			return nil
		}
		if _, exists := snapshot.files[record.Path]; !exists {
			return nil
		}
		snapshot.symbols[record.Path] = append(snapshot.symbols[record.Path], record)
		count++
	}
	if scanner.Err() != nil || count != manifest.SymbolsIndexed {
		return nil
	}
	scanner = bufio.NewScanner(strings.NewReader(string(verified["calls"])))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	count = 0
	for scanner.Scan() {
		var record PHPCallRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil || record.Path == "" ||
			record.Line <= 0 || record.Column <= 0 || (record.CallKind != "method" && record.CallKind != "function") ||
			(record.CallerQualifiedName != "" && (record.CallerKind == "" || record.CallerLine <= 0)) {
			return nil
		}
		file, ok := snapshot.files[record.Path]
		if !ok || file.Language != "PHP" {
			return nil
		}
		snapshot.calls[record.Path] = append(snapshot.calls[record.Path], record)
		count++
	}
	if scanner.Err() != nil || count != manifest.CallsIndexed {
		return nil
	}
	return snapshot
}

func cachedOutline(cache *indexSnapshot, rel string, record FileRecord, limit int) (fileOutline, bool) {
	if cache == nil || limit <= 0 {
		return fileOutline{}, false
	}
	old, exists := cache.files[rel]
	if !exists || !old.SymbolsComplete || old.Language != record.Language || old.Namespace != record.Namespace ||
		old.SHA256 == "" || old.SHA256 != record.SHA256 {
		return fileOutline{}, false
	}
	prior := cache.symbols[rel]
	if len(prior) > limit {
		return fileOutline{}, false
	}
	out := fileOutline{
		Path: rel, Language: record.Language, Namespace: record.Namespace,
		Symbols: make([]symbol, 0, len(prior)),
	}
	for _, item := range prior {
		out.Symbols = append(out.Symbols, symbol{Kind: item.Kind, Name: item.Name, Line: item.Line})
	}
	return out, true
}
