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

type IndexStatusChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
	Detail string `json:"detail,omitempty"`
}

type IndexStatusResult struct {
	State            string              `json:"state"`
	Fresh            bool                `json:"fresh"`
	Complete         bool                `json:"complete"`
	GeneratedAt      string              `json:"generated_at,omitempty"`
	SchemaVersion    int                 `json:"schema_version,omitempty"`
	ArtifactVerified bool                `json:"artifact_verified"`
	IndexedFiles     int                 `json:"indexed_files,omitempty"`
	CheckedFiles     int                 `json:"checked_files,omitempty"`
	ChangeCount      int                 `json:"change_count,omitempty"`
	Changes          []IndexStatusChange `json:"changes,omitempty"`
	ChangesTruncated bool                `json:"changes_truncated,omitempty"`
	Reasons          []string            `json:"reasons,omitempty"`
}

func IndexStatus(root string, maxChanges int) (IndexStatusResult, error) {
	return IndexStatusAt(root, root, maxChanges)
}
func IndexStatusAt(root, indexRoot string, maxChanges int) (IndexStatusResult, error) {
	if maxChanges <= 0 {
		maxChanges = 50
	}
	if maxChanges > 200 {
		maxChanges = 200
	}

	manifestPath := filepath.Join(indexRoot, filepath.FromSlash(IndexManifestRelativePath))
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return IndexStatusResult{State: "missing", Reasons: []string{"project index has not been generated"}}, nil
		}
		return IndexStatusResult{}, err
	}
	var manifest IndexManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return IndexStatusResult{State: "invalid", Reasons: []string{"manifest JSON is invalid"}}, nil
	}
	status := IndexStatusResult{
		State: "invalid", GeneratedAt: manifest.GeneratedAt, SchemaVersion: manifest.SchemaVersion, IndexedFiles: manifest.FilesIndexed,
	}
	if manifest.SchemaVersion != IndexSchemaVersion {
		status.Reasons = []string{fmt.Sprintf("unsupported schema_version=%d", manifest.SchemaVersion)}
		return status, nil
	}

	verified := map[string][]byte{}
	for _, item := range []struct{ key, path string }{
		{"overview", OverviewRelativePath}, {"files", IndexFilesRelativePath}, {"symbols", IndexSymbolsRelativePath}, {"calls", IndexCallsRelativePath},
	} {
		artifact, ok := manifest.Artifacts[item.key]
		if !ok || artifact.Path != item.path || artifact.SHA256 == "" {
			status.Reasons = []string{fmt.Sprintf("%s artifact manifest entry is invalid", item.key)}
			return status, nil
		}
		data, readErr := os.ReadFile(filepath.Join(indexRoot, filepath.FromSlash(item.path)))
		if readErr != nil {
			if os.IsNotExist(readErr) {
				status.Reasons = []string{fmt.Sprintf("%s artifact is missing", item.key)}
				return status, nil
			}
			return IndexStatusResult{}, readErr
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), artifact.SHA256) {
			status.Reasons = []string{fmt.Sprintf("%s artifact hash does not match manifest", item.key)}
			return status, nil
		}
		verified[item.key] = data
	}
	status.ArtifactVerified = true

	indexed := map[string]FileRecord{}
	scanner := bufio.NewScanner(strings.NewReader(string(verified["files"])))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record FileRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			status.State = "invalid"
			status.Reasons = []string{"files index contains invalid JSONL"}
			return status, nil
		}
		indexed[filepath.ToSlash(record.Path)] = record
	}
	if err := scanner.Err(); err != nil {
		return IndexStatusResult{}, err
	}

	maxFiles := manifest.Bounds.MaxFiles
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}
	seen := map[string]bool{}
	scanTruncated := false
	addChange := func(change IndexStatusChange) {
		status.ChangeCount++
		if len(status.Changes) < maxChanges {
			status.Changes = append(status.Changes, change)
		} else {
			status.ChangesTruncated = true
		}
	}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if status.CheckedFiles >= maxFiles {
			scanTruncated = true
			return fs.SkipAll
		}
		status.CheckedFiles++
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		record, ok := indexed[rel]
		if !ok {
			addChange(IndexStatusChange{Path: rel, Change: "added"})
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() != record.Size {
			addChange(IndexStatusChange{Path: rel, Change: "modified", Detail: "size changed"})
			return nil
		}
		if record.SHA256 != "" {
			if current := sourceHash(path); current == "" || !strings.EqualFold(current, record.SHA256) {
				addChange(IndexStatusChange{Path: rel, Change: "modified", Detail: "content hash changed"})
			}
			return nil
		}
		if record.ModifiedAt != "" && info.ModTime().UTC().Format(time.RFC3339Nano) != record.ModifiedAt {
			addChange(IndexStatusChange{Path: rel, Change: "modified", Detail: "modified time changed"})
		}
		return nil
	})
	if err != nil {
		return IndexStatusResult{}, err
	}

	if !scanTruncated {
		for path := range indexed {
			if !seen[path] {
				addChange(IndexStatusChange{Path: path, Change: "removed"})
			}
		}
	}

	status.Complete = !manifest.Truncated && !scanTruncated
	switch {
	case status.ChangeCount > 0:
		status.State = "stale"
		status.Reasons = []string{"project files changed after project_analyze"}
	case !status.Complete:
		status.State = "partial"
		status.Reasons = []string{"index or freshness scan reached configured bounds"}
	default:
		status.State = "fresh"
		status.Fresh = true
	}
	return status, nil
}
