package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// ForCompare returns a stable absolute path string suitable for containment and
// equality checks. It resolves the longest existing path prefix so Windows 8.3
// short-name prefixes such as RUNNER~1 compare consistently with long paths
// such as runneradmin, while still preserving missing leaf segments.
func ForCompare(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return filepath.Clean(path)
	}
	return canonicalizeLongestExisting(abs)
}

func Same(a, b string) bool {
	return strings.EqualFold(ForCompare(a), ForCompare(b))
}

func Within(base, target string) bool {
	base = ForCompare(base)
	target = ForCompare(target)
	if base == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func canonicalizeLongestExisting(abs string) string {
	clean := filepath.Clean(abs)
	current := clean
	missing := []string{}
	for {
		if _, err := os.Stat(current); err == nil {
			resolved := current
			if value, err := filepath.EvalSymlinks(current); err == nil {
				resolved = value
			}
			resolved = normalizeExistingPath(resolved)
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return clean
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
