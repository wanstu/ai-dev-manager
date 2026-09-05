package runtime

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultSearchFiles     = 2000
	defaultSearchMatches   = 200
	defaultSearchBytes     = 2 << 20
	hardSearchFiles        = 20000
	hardSearchMatches      = 5000
	hardSearchBytesPerFile = 10 << 20
)

type SearchOptions struct {
	Path            string
	Query           string
	MaxFiles        int
	MaxMatches      int
	MaxBytesPerFile int
}

type SearchMatch struct {
	Path string
	Line int
	Text string
}

func (r *Native) Search(options SearchOptions) ([]SearchMatch, error) {
	if options.Query == "" {
		return nil, &RuntimeError{Kind: ErrInvalidPath, Path: options.Path}
	}
	start, _, err := r.guard.Existing(options.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(start)
	if err != nil {
		return nil, &RuntimeError{Kind: ErrIO, Path: options.Path, Err: err}
	}

	maxFiles := options.MaxFiles
	if maxFiles <= 0 {
		maxFiles = defaultSearchFiles
	}
	maxMatches := options.MaxMatches
	if maxMatches <= 0 {
		maxMatches = defaultSearchMatches
	}
	maxBytes := options.MaxBytesPerFile
	if maxBytes <= 0 {
		maxBytes = defaultSearchBytes
	}
	if maxFiles > hardSearchFiles || maxMatches > hardSearchMatches || maxBytes > hardSearchBytesPerFile {
		return nil, &RuntimeError{Kind: ErrLimitExceeded}
	}

	matches := make([]SearchMatch, 0)
	filesSeen := 0
	searchFile := func(path string, fileInfo fs.FileInfo) error {
		if !fileInfo.Mode().IsRegular() {
			return nil
		}
		filesSeen++
		if filesSeen > maxFiles {
			return &RuntimeError{Kind: ErrLimitExceeded}
		}
		if fileInfo.Size() > int64(maxBytes) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for index, line := range lines {
			if !strings.Contains(line, options.Query) {
				continue
			}
			rel, relErr := filepath.Rel(r.root, path)
			if relErr != nil {
				return relErr
			}
			matches = append(matches, SearchMatch{Path: filepath.Clean(rel), Line: index + 1, Text: strings.TrimSuffix(line, "\r")})
			if len(matches) > maxMatches {
				return &RuntimeError{Kind: ErrLimitExceeded}
			}
		}
		return nil
	}

	if info.IsDir() {
		err = filepath.WalkDir(start, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			entryInfo, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			return searchFile(path, entryInfo)
		})
	} else {
		if !info.Mode().IsRegular() {
			return nil, &RuntimeError{Kind: ErrInvalidPath, Path: options.Path}
		}
		err = searchFile(start, info)
	}
	if err != nil {
		if runtimeErr, ok := err.(*RuntimeError); ok {
			return nil, runtimeErr
		}
		return nil, &RuntimeError{Kind: ErrIO, Path: options.Path, Err: err}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		return matches[i].Line < matches[j].Line
	})
	return matches, nil
}
