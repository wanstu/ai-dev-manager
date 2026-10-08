package projectanalysis

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// readVerifiedPHPCallIndex checks integrity of all persisted indexes, but
// never opens a PHP source file. The full content freshness check remains a
// separate, explicit project_index_status operation.
func readVerifiedPHPCallIndex(root string) (IndexManifest, []byte, []byte, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(IndexManifestRelativePath)))
	if err != nil {
		return IndexManifest{}, nil, nil, nil, fmt.Errorf("ADM project index missing; run project_analyze: %w", err)
	}
	var manifest IndexManifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.SchemaVersion != IndexSchemaVersion {
		return IndexManifest{}, nil, nil, nil, fmt.Errorf("ADM project index schema is invalid or unsupported; run project_analyze")
	}
	output := map[string][]byte{}
	for _, item := range []struct{ key, path string }{
		{"overview", OverviewRelativePath},
		{"files", IndexFilesRelativePath},
		{"symbols", IndexSymbolsRelativePath},
		{"calls", IndexCallsRelativePath},
	} {
		artifact, ok := manifest.Artifacts[item.key]
		if !ok || artifact.Path != item.path || artifact.SHA256 == "" {
			return IndexManifest{}, nil, nil, nil, fmt.Errorf("ADM index %s metadata invalid; run project_analyze", item.key)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.path)))
		if err != nil {
			return IndexManifest{}, nil, nil, nil, fmt.Errorf("ADM index %s unreadable: %w", item.key, err)
		}
		sum := sha256.Sum256(data)
		if len(data) != artifact.Bytes || !strings.EqualFold(hex.EncodeToString(sum[:]), artifact.SHA256) {
			return IndexManifest{}, nil, nil, nil, fmt.Errorf("ADM index %s hash mismatch; run project_analyze", item.key)
		}
		output[item.key] = data
	}
	return manifest, output["files"], output["symbols"], output["calls"], nil
}

func eachJSONLine(data []byte, visitor func([]byte) error) error {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		if err := visitor(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func decodePHPCallRecord(line []byte, target *PHPCallRecord) error {
	if err := json.Unmarshal(line, target); err != nil {
		return fmt.Errorf("ADM call index corrupt: %w", err)
	}
	if target.Path == "" || target.Line <= 0 || target.Column <= 0 ||
		target.Name == "" || (target.CallKind != "method" && target.CallKind != "function") ||
		(target.CallerQualifiedName != "" && (target.CallerKind == "" || target.CallerLine <= 0)) {
		return fmt.Errorf("ADM call index contains invalid record")
	}
	return nil
}

// Metadata check detects added, removed, and normally edited source files
// without reading their content. It is intentionally weaker than IndexStatus:
// same-byte-length/same-mtime edits are only caught by that explicit hash scan.
func verifyIndexMetadata(root string, manifest IndexManifest, filesData []byte) error {
	files := map[string]FileRecord{}
	if err := eachJSONLine(filesData, func(raw []byte) error {
		var record FileRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		rel := filepath.Clean(filepath.FromSlash(record.Path))
		if record.Path == "" || rel == "." || rel == ".." || filepath.IsAbs(rel) ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid indexed file path")
		}
		if _, exists := files[filepath.ToSlash(rel)]; exists {
			return fmt.Errorf("duplicate indexed file")
		}
		files[filepath.ToSlash(rel)] = record
		return nil
	}); err != nil {
		return err
	}
	if len(files) != manifest.FilesIndexed {
		return fmt.Errorf("file index count mismatch")
	}
	checked := map[string]bool{}
	maxFiles := manifest.Bounds.MaxFiles
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}
	scanned := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.IsDir() {
			if ignoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if scanned >= maxFiles {
			return fs.SkipAll
		}
		scanned++
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		record, exists := files[rel]
		if !exists {
			return fmt.Errorf("added file %q", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() != record.Size {
			return fmt.Errorf("changed file %q", rel)
		}
		if record.ModifiedAt != "" && info.ModTime().UTC().Format(time.RFC3339Nano) != record.ModifiedAt {
			return fmt.Errorf("modified file %q", rel)
		}
		checked[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	for path := range files {
		if !checked[path] {
			return fmt.Errorf("deleted indexed file %q", path)
		}
	}
	return nil
}
