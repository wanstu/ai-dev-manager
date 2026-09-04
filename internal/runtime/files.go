package runtime

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultTreeDepth   = 4
	defaultTreeEntries = 800
	hardTreeDepth      = 32
	hardTreeEntries    = 10000
	defaultReadBytes   = 1 << 20
	hardReadBytes      = 10 << 20
)

type FileInfo struct {
	Path  string
	IsDir bool
	Size  int64
}

type WriteResult struct {
	Path  string
	Bytes int
}

type EditResult struct {
	Path         string
	Replacements int
	BytesBefore  int
	BytesAfter   int
}

type TreeOptions struct {
	MaxDepth   int
	MaxEntries int
}

func (r *Native) Tree(path string, options TreeOptions) ([]FileInfo, error) {
	start, _, err := r.guard.Existing(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(start)
	if err != nil {
		return nil, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	if !info.IsDir() {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: path}
	}
	maxDepth := options.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultTreeDepth
	}
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultTreeEntries
	}
	if maxDepth > hardTreeDepth || maxEntries > hardTreeEntries {
		return nil, &RuntimeError{Kind: ErrLimitExceeded}
	}

	entries := make([]FileInfo, 0)
	err = filepath.WalkDir(start, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == start {
			return nil
		}
		relFromStart, relErr := filepath.Rel(start, current)
		if relErr != nil {
			return relErr
		}
		depth := pathDepth(relFromStart)
		if entry.IsDir() && depth > maxDepth {
			return filepath.SkipDir
		}
		if depth > maxDepth {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 && entry.IsDir() {
			return filepath.SkipDir
		}
		stat, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		rel, relErr := filepath.Rel(r.root, current)
		if relErr != nil {
			return relErr
		}
		entries = append(entries, FileInfo{Path: filepath.Clean(rel), IsDir: entry.IsDir(), Size: stat.Size()})
		if len(entries) > maxEntries {
			return &RuntimeError{Kind: ErrLimitExceeded}
		}
		return nil
	})
	if err != nil {
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			return nil, runtimeErr
		}
		return nil, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func (r *Native) Read(path string, maxBytes int) ([]byte, FileInfo, error) {
	target, rel, err := r.guard.Existing(path)
	if err != nil {
		return nil, FileInfo{}, err
	}
	if maxBytes <= 0 {
		maxBytes = defaultReadBytes
	}
	if maxBytes > hardReadBytes {
		return nil, FileInfo{}, &RuntimeError{Kind: ErrLimitExceeded}
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, FileInfo{}, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	if !info.Mode().IsRegular() {
		return nil, FileInfo{}, &RuntimeError{Kind: ErrInvalidPath, Path: path}
	}
	if info.Size() > int64(maxBytes) {
		return nil, FileInfo{}, &RuntimeError{Kind: ErrLimitExceeded}
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, FileInfo{}, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	return data, FileInfo{Path: rel, IsDir: false, Size: info.Size()}, nil
}

func (r *Native) Write(path string, data []byte, createParents bool) (WriteResult, error) {
	if !r.canWrite() {
		return WriteResult{}, &RuntimeError{Kind: ErrReadOnly}
	}
	target, rel, err := r.guard.WriteTarget(path)
	if err != nil {
		return WriteResult{}, err
	}
	parent := filepath.Dir(target)
	if createParents {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return WriteResult{}, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
		}
	} else if _, err := os.Stat(parent); err != nil {
		if os.IsNotExist(err) {
			return WriteResult{}, &RuntimeError{Kind: ErrNotFound, Path: filepath.Dir(rel), Err: err}
		}
		return WriteResult{}, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	if err := atomicWrite(target, data); err != nil {
		return WriteResult{}, &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	return WriteResult{Path: rel, Bytes: len(data)}, nil
}

func (r *Native) Edit(path, oldText, newText string, expectedReplacements int) (EditResult, error) {
	if !r.canWrite() {
		return EditResult{}, &RuntimeError{Kind: ErrReadOnly}
	}
	if oldText == "" {
		return EditResult{}, &RuntimeError{Kind: ErrInvalidEdit, Path: path}
	}
	if expectedReplacements <= 0 {
		expectedReplacements = 1
	}
	data, _, err := r.Read(path, hardReadBytes)
	if err != nil {
		return EditResult{}, err
	}
	before := string(data)
	count := strings.Count(before, oldText)
	if count != expectedReplacements {
		return EditResult{}, &RuntimeError{Kind: ErrInvalidEdit, Path: path}
	}
	after := strings.Replace(before, oldText, newText, expectedReplacements)
	writeResult, err := r.Write(path, []byte(after), false)
	if err != nil {
		return EditResult{}, err
	}
	return EditResult{
		Path:         writeResult.Path,
		Replacements: count,
		BytesBefore:  len(data),
		BytesAfter:   len(after),
	}, nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".adm-runtime-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func pathDepth(path string) int {
	clean := filepath.Clean(path)
	if clean == "." || clean == "" {
		return 0
	}
	return len(strings.FieldsFunc(clean, func(r rune) bool { return r == '/' || r == '\\' }))
}
