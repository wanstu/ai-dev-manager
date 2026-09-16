package skill

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/model"
	"ai-dev-manager-v2/internal/pathutil"
)

const defaultMaxBytes = 1 << 20

type Content struct {
	SkillID string `json:"skill_id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

func CanonicalSource(root string, supportRoots []string, defaultInclude bool) (model.SkillSource, error) {
	resolvedRoot, resolvedSupportRoots, err := resolveSourcePaths(root, supportRoots)
	if err != nil {
		return model.SkillSource{}, err
	}
	return model.SkillSource{Root: resolvedRoot, SupportRoots: resolvedSupportRoots, DefaultIncludeInEnv: defaultInclude}, nil
}

func Discover(root string, supportRoots []string, defaultInclude bool) ([]model.CatalogEntry, error) {
	source, err := CanonicalSource(root, supportRoots, defaultInclude)
	if err != nil {
		return nil, err
	}
	source.ID = StableSourceID(source.Root)
	entries, err := DiscoverSource(source)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("skill discovery root %s contains no SKILL.md artifacts", source.Root)
	}
	return entries, nil
}

func DiscoverSource(source model.SkillSource) ([]model.CatalogEntry, error) {
	source.ID = strings.TrimSpace(source.ID)
	if source.ID == "" {
		return nil, fmt.Errorf("skill source id is required")
	}
	resolvedRoot, resolvedSupportRoots, err := resolveSourcePaths(source.Root, source.SupportRoots)
	if err != nil {
		return nil, err
	}

	byID := map[string]model.CatalogEntry{}
	err = filepath.WalkDir(resolvedRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		dir := filepath.Dir(path)
		name := strings.TrimSpace(filepath.Base(dir))
		if name == "" || name == "." || name == string(filepath.Separator) {
			return fmt.Errorf("invalid skill directory for %s", path)
		}
		artifact, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		artifact = filepath.Clean(artifact)
		relative, err := RelativeArtifactPath(resolvedRoot, artifact)
		if err != nil {
			return err
		}
		id := SourceArtifactID(source.ID, relative)
		byID[id] = model.CatalogEntry{
			ID:                   id,
			SourceID:             source.ID,
			Name:                 name,
			DefaultIncludeInEnv:  source.DefaultIncludeInEnv,
			ArtifactPath:         artifact,
			RelativeArtifactPath: relative,
			SourceRoot:           resolvedRoot,
			SupportRoots:         append([]string(nil), resolvedSupportRoots...),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover skills under %s: %w", resolvedRoot, err)
	}
	out := make([]model.CatalogEntry, 0, len(byID))
	for _, entry := range byID {
		out = append(out, entry)
	}
	sortCatalogEntries(out)
	return out, nil
}

func StableID(name string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(name))))
	return fmt.Sprintf("skill_%x", sum[:16])
}

func StableSourceID(root string) string {
	sum := sha256.Sum256([]byte("source|" + strings.ToLower(filepath.Clean(root))))
	return fmt.Sprintf("skill_source_%x", sum[:16])
}

func SourceArtifactID(sourceID, relativeArtifactPath string) string {
	identity := strings.ToLower(strings.TrimSpace(sourceID)) + "|" + normalizeRelativePath(relativeArtifactPath)
	sum := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("skill_%x", sum[:16])
}

func RelativeArtifactPath(sourceRoot, artifactPath string) (string, error) {
	rel, err := filepath.Rel(filepath.Clean(sourceRoot), filepath.Clean(artifactPath))
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("skill artifact %s is outside source root %s", artifactPath, sourceRoot)
	}
	return normalizeRelativePath(rel), nil
}

func Configured(entry model.CatalogEntry) bool {
	return strings.TrimSpace(entry.ArtifactPath) != "" && strings.TrimSpace(entry.SourceRoot) != ""
}

func Read(entry model.CatalogEntry, requestedPath string, maxBytes int) (Content, error) {
	if !Configured(entry) {
		return Content{}, fmt.Errorf("skill %q is not backed by a configured artifact", entry.ID)
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	artifact, err := canonicalFile(entry.ArtifactPath)
	if err != nil {
		return Content{}, fmt.Errorf("skill %q artifact: %w", entry.ID, err)
	}
	artifactDir := filepath.Dir(artifact)

	target := artifact
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath != "" && requestedPath != "SKILL.md" {
		if filepath.IsAbs(requestedPath) {
			target = filepath.Clean(requestedPath)
		} else {
			target = filepath.Clean(filepath.Join(artifactDir, requestedPath))
		}
	}
	resolvedTarget, err := canonicalFile(target)
	if err != nil {
		return Content{}, fmt.Errorf("skill %q read %q: %w", entry.ID, requestedPath, err)
	}

	allowedRoots := []string{artifactDir}
	for _, root := range entry.SupportRoots {
		resolved, err := canonicalDirectory(root)
		if err != nil {
			return Content{}, fmt.Errorf("skill %q support root: %w", entry.ID, err)
		}
		allowedRoots = append(allowedRoots, resolved)
	}
	allowed := false
	for _, root := range allowedRoots {
		if within(root, resolvedTarget) {
			allowed = true
			break
		}
	}
	if !allowed {
		return Content{}, fmt.Errorf("skill %q path is outside configured artifact/support roots", entry.ID)
	}

	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return Content{}, err
	}
	if !info.Mode().IsRegular() {
		return Content{}, fmt.Errorf("skill path is not a regular file")
	}
	if info.Size() > int64(maxBytes) {
		return Content{}, fmt.Errorf("skill file exceeds max_bytes=%d", maxBytes)
	}
	data, err := os.ReadFile(resolvedTarget)
	if err != nil {
		return Content{}, err
	}
	if containsNUL(data) {
		return Content{}, fmt.Errorf("binary skill file is not readable as text")
	}
	return Content{SkillID: entry.ID, Name: entry.Name, Path: resolvedTarget, Content: string(data)}, nil
}

func resolveSourcePaths(root string, supportRoots []string) (string, []string, error) {
	resolvedRoot, err := canonicalDirectory(root)
	if err != nil {
		return "", nil, fmt.Errorf("skill discovery root: %w", err)
	}
	resolvedSupportRoots := make([]string, 0, len(supportRoots))
	seenSupportRoots := map[string]struct{}{}
	for _, supportRoot := range supportRoots {
		if strings.TrimSpace(supportRoot) == "" {
			continue
		}
		resolved, err := canonicalDirectory(supportRoot)
		if err != nil {
			return "", nil, fmt.Errorf("skill support root: %w", err)
		}
		key := strings.ToLower(filepath.Clean(resolved))
		if _, exists := seenSupportRoots[key]; exists {
			continue
		}
		seenSupportRoots[key] = struct{}{}
		resolvedSupportRoots = append(resolvedSupportRoots, resolved)
	}
	sort.Slice(resolvedSupportRoots, func(i, j int) bool {
		return strings.ToLower(resolvedSupportRoots[i]) < strings.ToLower(resolvedSupportRoots[j])
	})
	return resolvedRoot, resolvedSupportRoots, nil
}

func canonicalDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("path is not a real directory: %s", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func canonicalFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(pathutil.ForCompare(root), pathutil.ForCompare(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func samePath(a, b string) bool { return pathutil.Same(a, b) }

func containsNUL(data []byte) bool {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if strings.IndexByte(scanner.Text(), 0) >= 0 {
			return true
		}
	}
	return strings.IndexByte(string(data), 0) >= 0
}

func normalizeRelativePath(path string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(strings.TrimSpace(path))))
}

func sortCatalogEntries(entries []model.CatalogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := strings.ToLower(entries[i].SourceID + "/" + entries[i].RelativeArtifactPath + "/" + entries[i].Name)
		right := strings.ToLower(entries[j].SourceID + "/" + entries[j].RelativeArtifactPath + "/" + entries[j].Name)
		return left < right
	})
}

func NowUTC() time.Time { return time.Now().UTC() }
