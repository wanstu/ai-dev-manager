package management

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const maxHostDirectoryEntries = 500

type HostDirectoryEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type HostDirectoryListing struct {
	Path        string               `json:"path"`
	Parent      string               `json:"parent,omitempty"`
	Directories []HostDirectoryEntry `json:"directories"`
	Truncated   bool                 `json:"truncated,omitempty"`
}

func (s *Service) HostDirectoryList(path string) (HostDirectoryListing, error) {
	return listHostDirectories(path)
}

func listHostDirectories(path string) (HostDirectoryListing, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		if runtime.GOOS == "windows" {
			return windowsDriveListing(), nil
		}
		path = string(filepath.Separator)
	}
	if !filepath.IsAbs(path) {
		return HostDirectoryListing{}, fmt.Errorf("directory path must be absolute")
	}

	current := filepath.Clean(path)
	info, err := os.Stat(current)
	if err != nil {
		return HostDirectoryListing{}, fmt.Errorf("inspect directory %q: %w", current, err)
	}
	if !info.IsDir() {
		return HostDirectoryListing{}, fmt.Errorf("%q is not a directory", current)
	}

	entries, err := os.ReadDir(current)
	if err != nil {
		return HostDirectoryListing{}, fmt.Errorf("read directory %q: %w", current, err)
	}
	directories := make([]HostDirectoryEntry, 0, len(entries))
	truncated := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if len(directories) >= maxHostDirectoryEntries {
			truncated = true
			break
		}
		directories = append(directories, HostDirectoryEntry{
			Name: entry.Name(),
			Path: filepath.Join(current, entry.Name()),
		})
	}
	parent := filepath.Dir(current)
	if parent == current {
		parent = ""
	}
	return HostDirectoryListing{
		Path:        current,
		Parent:      parent,
		Directories: directories,
		Truncated:   truncated,
	}, nil
}

func windowsDriveListing() HostDirectoryListing {
	directories := make([]HostDirectoryEntry, 0, 8)
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := fmt.Sprintf("%c:\\", letter)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		directories = append(directories, HostDirectoryEntry{Name: root, Path: root})
	}
	sort.Slice(directories, func(i, j int) bool {
		return strings.ToLower(directories[i].Path) < strings.ToLower(directories[j].Path)
	})
	return HostDirectoryListing{Directories: directories}
}
