package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"ai-dev-manager-v2/internal/model"
)

var discoveryExcluded = []string{".git", "node_modules", "vendor", "dist", "build", "target", "bin", "obj", "coverage", ".next", ".venv", "venv", "__pycache__", ".tmp"}

func discoveryLimits(r model.DiscoveryRequest) (model.DiscoveryLimits, error) {
	values := []int{r.MaxDepth, r.MaxEntries, r.MaxCandidates, r.MaxDigestEntries, r.MaxOutputBytes}
	defaults := []int{4, 2000, 50, 100, 65536}
	caps := []int{8, 10000, 200, 500, 262144}
	for i := range values {
		if values[i] < 0 || values[i] > caps[i] {
			return model.DiscoveryLimits{}, fmt.Errorf("discovery budget out of range")
		}
		if values[i] == 0 {
			values[i] = defaults[i]
		}
	}
	if values[4] < 4096 {
		return model.DiscoveryLimits{}, fmt.Errorf("max_output_bytes must be at least 4096")
	}
	if utf8.RuneCountInString(r.Query) > 200 {
		return model.DiscoveryLimits{}, fmt.Errorf("query exceeds 200 characters")
	}
	return model.DiscoveryLimits{MaxDepth: values[0], MaxEntries: values[1], MaxCandidates: values[2], MaxDigestEntries: values[3], MaxOutputBytes: values[4]}, nil
}

func discoveryPath(path string) (string, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	if path == "" {
		return ".", nil
	}
	if strings.Contains(path, ":") || !filepath.IsLocal(filepath.FromSlash(path)) {
		return "", fmt.Errorf("discovery path must be root-relative")
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return "", fmt.Errorf("parent traversal is not allowed")
		}
	}
	return filepath.Clean(filepath.FromSlash(path)), nil
}

// ScanDiscovery inspects directory metadata only. Scope is supplied by the app
// after stable-ID authority resolution; it is included in the output byte budget.
func ScanDiscovery(ctx context.Context, root string, scope model.DiscoveryScope, request model.DiscoveryRequest, withCandidates bool) (model.DiscoveryReport, error) {
	return scanDiscovery(ctx, root, scope, request, withCandidates, nil)
}

// The hook is used by package tests to inject filesystem changes/cancellation at
// a batch boundary, without relying on OS permissions or wall-clock timing.
func scanDiscovery(ctx context.Context, root string, scope model.DiscoveryScope, request model.DiscoveryRequest, withCandidates bool, beforeOpen func(string)) (model.DiscoveryReport, error) {
	limits, err := discoveryLimits(request)
	if err != nil {
		return model.DiscoveryReport{}, err
	}
	if !withCandidates && request.Query != "" {
		return model.DiscoveryReport{}, fmt.Errorf("query is only supported for discovery")
	}
	start, err := discoveryPath(request.Path)
	if err != nil {
		return model.DiscoveryReport{}, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || discoveryLink(info) {
		return model.DiscoveryReport{}, fmt.Errorf("discovery root is unavailable or is a link")
	}
	boundary, err := os.OpenRoot(root)
	if err != nil {
		return model.DiscoveryReport{}, fmt.Errorf("discovery root cannot be opened")
	}
	defer boundary.Close()
	pinned, err := boundary.Stat(".")
	if err != nil || !os.SameFile(info, pinned) {
		return model.DiscoveryReport{}, fmt.Errorf("discovery root changed")
	}
	// Explicit paths cannot select a link, even one pointing back into the root.
	part := "."
	for _, component := range strings.Split(filepath.ToSlash(start), "/") {
		if component == "." {
			continue
		}
		part = filepath.Join(part, component)
		node, err := boundary.Lstat(part)
		if err != nil || !node.IsDir() || discoveryLink(node) {
			return model.DiscoveryReport{}, fmt.Errorf("discovery subpath is unavailable or is a link")
		}
	}
	out := model.DiscoveryReport{Scope: scope, ScanPath: filepath.ToSlash(start), Query: request.Query, ObservedAt: time.Now().UTC(), Limits: limits,
		Candidates: []model.ProjectCandidate{}, Digest: []model.DirectoryDigestEntry{}, StopReasons: []string{}, Diagnostics: []model.DiscoveryDiagnostic{},
		ExcludedDirectories: append([]string{}, discoveryExcluded...), Coverage: "complete_under_exclusion_policy"}
	reason := func(code string) {
		out.Truncated = true
		for _, v := range out.StopReasons {
			if v == code {
				return
			}
		}
		out.StopReasons = append(out.StopReasons, code)
	}
	diagnostic := func(path, code string) {
		if len(out.Diagnostics) < 32 {
			out.Diagnostics = append(out.Diagnostics, model.DiscoveryDiagnostic{Path: filepath.ToSlash(path), Reason: code})
		} else {
			out.OmittedDiagnostics++
		}
	}
	type pending struct {
		path  string
		depth int
	}
	queue := []pending{{start, 0}}
	candidates := map[string]*model.ProjectCandidate{}
	addCandidate := func(path string) *model.ProjectCandidate {
		if c := candidates[path]; c != nil {
			return c
		}
		name := filepath.Base(path)
		if path == "." {
			name = filepath.Base(root)
		}
		c := &model.ProjectCandidate{Root: filepath.ToSlash(path), Name: name, SuggestedEnvironmentRoot: filepath.ToSlash(path), Evidence: "directory-only", Markers: []model.ProjectMarker{}}
		candidates[path] = c
		return c
	}
	excluded := map[string]bool{}
	for _, v := range discoveryExcluded {
		excluded[v] = true
	}
	for len(queue) > 0 {
		if ctx.Err() != nil {
			reason("canceled")
			break
		}
		if out.VisitedEntries >= limits.MaxEntries {
			reason("entry_limit")
			break
		}
		current := queue[0]
		queue = queue[1:]
		d := model.DirectoryDigestEntry{Path: filepath.ToSlash(current.path), Kind: "directory"}
		if current.depth >= limits.MaxDepth {
			reason("depth_limit")
			out.Digest = append(out.Digest, d)
			continue
		}
		if beforeOpen != nil {
			beforeOpen(current.path)
		}
		if ctx.Err() != nil {
			reason("canceled")
			break
		}
		f, openErr := openDiscoveryDir(boundary, current.path)
		if openErr != nil {
			if current.depth == 0 {
				return model.DiscoveryReport{}, fmt.Errorf("discovery scan directory cannot be read")
			}
			reason("directory_unreadable")
			diagnostic(current.path, "directory_unreadable")
			out.Digest = append(out.Digest, d)
			continue
		}
		for {
			if ctx.Err() != nil {
				reason("canceled")
				break
			}
			remaining := limits.MaxEntries - out.VisitedEntries
			if remaining <= 0 {
				reason("entry_limit")
				break
			}
			n := min(64, remaining)
			entries, readErr := f.ReadDir(n)
			out.ReadBatches++
			for _, entry := range entries {
				out.VisitedEntries++
				d.ObservedChildren++
				if ctx.Err() != nil {
					reason("canceled")
					break
				}
				rel := filepath.Join(current.path, entry.Name())
				meta, statErr := boundary.Lstat(rel)
				if statErr != nil {
					reason("entry_unreadable")
					diagnostic(rel, "entry_unreadable")
					continue
				}
				if discoveryLink(meta) {
					out.SkippedLinks++
					diagnostic(rel, "link_skipped")
					continue
				}
				marker := discoveryMarker(entry.Name(), meta)
				if marker != "" {
					d.ObservedMarkers++
					if withCandidates {
						c := addCandidate(current.path)
						c.Evidence = "marker"
						if len(c.Markers) < 64 {
							c.Markers = append(c.Markers, model.ProjectMarker{Path: filepath.ToSlash(rel), Kind: marker})
						} else {
							out.OmittedMarkers++
							reason("marker_limit")
						}
					}
				}
				if meta.IsDir() {
					if excluded[strings.ToLower(entry.Name())] {
						out.SkippedDirectories++
						continue
					}
					if withCandidates && current.depth == 0 {
						addCandidate(rel)
					}
					queue = append(queue, pending{rel, current.depth + 1})
				}
			}
			if ctx.Err() != nil {
				reason("canceled")
				break
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					d.ChildrenComplete = true
				} else {
					if current.depth == 0 {
						f.Close()
						return model.DiscoveryReport{}, fmt.Errorf("discovery scan directory cannot be read")
					}
					reason("directory_unreadable")
					diagnostic(current.path, "directory_unreadable")
				}
				break
			}
		}
		f.Close()
		out.Digest = append(out.Digest, d)
	}
	if out.SkippedLinks > 0 {
		reason("links_skipped")
	}
	for _, c := range candidates {
		c.QueryMatch = discoveryMatch(request.Query, c.Root, c.Name)
		if request.Query != "" && c.QueryMatch == "none" {
			continue
		}
		sort.Slice(c.Markers, func(i, j int) bool { return c.Markers[i].Path < c.Markers[j].Path })
		out.Candidates = append(out.Candidates, *c)
	}
	ranks := map[string]int{"exact_path": 0, "exact_name": 1, "substring": 2, "none": 3}
	sort.Slice(out.Candidates, func(i, j int) bool {
		a, b := out.Candidates[i], out.Candidates[j]
		if ranks[a.QueryMatch] != ranks[b.QueryMatch] {
			return ranks[a.QueryMatch] < ranks[b.QueryMatch]
		}
		if a.Evidence != b.Evidence {
			return a.Evidence == "marker"
		}
		if strings.ToLower(a.Root) != strings.ToLower(b.Root) {
			return strings.ToLower(a.Root) < strings.ToLower(b.Root)
		}
		return a.Root < b.Root
	})
	sort.Slice(out.Digest, func(i, j int) bool { return out.Digest[i].Path < out.Digest[j].Path })
	if len(out.Candidates) > limits.MaxCandidates {
		out.OmittedCandidates += len(out.Candidates) - limits.MaxCandidates
		out.Candidates = out.Candidates[:limits.MaxCandidates]
		reason("candidate_limit")
	}
	if len(out.Digest) > limits.MaxDigestEntries {
		out.OmittedDigestEntries += len(out.Digest) - limits.MaxDigestEntries
		out.Digest = out.Digest[:limits.MaxDigestEntries]
		reason("digest_limit")
	}
	if out.Truncated {
		out.Coverage = "partial; unvisited entries may depend on filesystem enumeration order"
	}
	for {
		sort.Strings(out.StopReasons)
		data, err := json.Marshal(out)
		if err != nil {
			return model.DiscoveryReport{}, err
		}
		if len(data) <= limits.MaxOutputBytes {
			break
		}
		reason("output_byte_limit")
		out.Coverage = "partial; unvisited entries may depend on filesystem enumeration order"
		switch {
		case len(out.Diagnostics) > 0:
			out.Diagnostics = out.Diagnostics[:len(out.Diagnostics)-1]
			out.OmittedDiagnostics++
		case len(out.Digest) > 0:
			out.Digest = out.Digest[:len(out.Digest)-1]
			out.OmittedDigestEntries++
		case len(out.Candidates) > 0:
			out.Candidates = out.Candidates[:len(out.Candidates)-1]
			out.OmittedCandidates++
		default:
			return model.DiscoveryReport{}, fmt.Errorf("discovery identity metadata exceeds output budget")
		}
	}
	return out, nil
}

func openDiscoveryDir(root *os.Root, path string) (*os.File, error) {
	before, err := root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if discoveryLink(before) || !before.IsDir() {
		return nil, fmt.Errorf("not an ordinary directory")
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		f.Close()
		return nil, fmt.Errorf("directory changed")
	}
	return f, nil
}

func discoveryMarker(name string, info os.FileInfo) string {
	if name == ".git" && (info.IsDir() || info.Mode().IsRegular()) {
		return "git"
	}
	if !info.Mode().IsRegular() {
		return ""
	}
	switch name {
	case "go.mod", "go.work":
		return "go"
	case "package.json":
		return "node"
	case "pyproject.toml":
		return "python"
	case "Cargo.toml":
		return "rust"
	case "pom.xml", "build.gradle", "build.gradle.kts":
		return "jvm"
	case "composer.json":
		return "php"
	}
	if strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".csproj") {
		return "dotnet"
	}
	return ""
}
func discoveryMatch(query, path, name string) string {
	if query == "" {
		return "none"
	}
	q := strings.ToLower(query)
	if q == strings.ToLower(path) {
		return "exact_path"
	}
	if q == strings.ToLower(name) {
		return "exact_name"
	}
	if strings.Contains(strings.ToLower(path), q) || strings.Contains(strings.ToLower(name), q) {
		return "substring"
	}
	return "none"
}
