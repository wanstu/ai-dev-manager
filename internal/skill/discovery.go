package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ai-dev-manager/internal/model"
)

type ErrorKind string

const (
	ErrInvalidRoot ErrorKind = "invalid_root"
	ErrRootMissing ErrorKind = "root_not_found"
	ErrWalkFailed  ErrorKind = "walk_failed"
)

type DiscoveryError struct {
	Kind ErrorKind
	Root string
	Err  error
}

func (e *DiscoveryError) Error() string {
	switch e.Kind {
	case ErrInvalidRoot:
		return fmt.Sprintf("invalid skill root %q", e.Root)
	case ErrRootMissing:
		return fmt.Sprintf("skill root not found: %q", e.Root)
	case ErrWalkFailed:
		return fmt.Sprintf("walk skill root %q: %v", e.Root, e.Err)
	default:
		return "skill discovery error"
	}
}

func (e *DiscoveryError) Unwrap() error { return e.Err }

// ExpandLayer discovers SKILL.md files from only the roots explicitly declared
// on layer. The returned layer is fully detached from the input and retains the
// same Scope. Later roots override earlier discovered Skills; explicit Skills
// from the layer override all discovery results in that layer.
func ExpandLayer(layer model.ConfigLayer, baseDir string) (model.ConfigLayer, error) {
	result := cloneLayer(layer)
	result.Skills = make(map[string]model.SkillDefinition)

	for _, configuredRoot := range layer.SkillRoots {
		root, err := resolveRoot(configuredRoot, baseDir)
		if err != nil {
			return model.ConfigLayer{}, err
		}
		info, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			return model.ConfigLayer{}, &DiscoveryError{Kind: ErrRootMissing, Root: configuredRoot}
		}
		if err != nil {
			return model.ConfigLayer{}, &DiscoveryError{Kind: ErrWalkFailed, Root: configuredRoot, Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return model.ConfigLayer{}, &DiscoveryError{Kind: ErrInvalidRoot, Root: configuredRoot}
		}

		discovered, err := discoverRoot(root, configuredRoot)
		if err != nil {
			return model.ConfigLayer{}, err
		}
		for id, definition := range discovered {
			result.Skills[id] = definition
		}
	}

	for id, definition := range layer.Skills {
		result.Skills[id] = cloneSkill(definition)
	}
	return result, nil
}

func resolveRoot(configuredRoot, baseDir string) (string, error) {
	root := strings.TrimSpace(configuredRoot)
	if root == "" {
		return "", &DiscoveryError{Kind: ErrInvalidRoot, Root: configuredRoot}
	}

	if root == "~" || strings.HasPrefix(root, "~/") || strings.HasPrefix(root, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", &DiscoveryError{Kind: ErrInvalidRoot, Root: configuredRoot, Err: err}
		}
		if root == "~" {
			return filepath.Clean(home), nil
		}
		return filepath.Clean(filepath.Join(home, root[2:])), nil
	}

	if filepath.IsAbs(root) {
		return filepath.Clean(root), nil
	}
	if strings.TrimSpace(baseDir) == "" {
		return "", &DiscoveryError{Kind: ErrInvalidRoot, Root: configuredRoot}
	}
	return filepath.Clean(filepath.Join(baseDir, root)), nil
}

func discoverRoot(root, configuredRoot string) (map[string]model.SkillDefinition, error) {
	result := make(map[string]model.SkillDefinition)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		id := filepath.Base(dir)
		if id == "" || id == "." || id == string(filepath.Separator) {
			return nil
		}
		result[id] = model.SkillDefinition{ID: id, Path: filepath.Clean(dir)}
		return nil
	})
	if err != nil {
		return nil, &DiscoveryError{Kind: ErrWalkFailed, Root: configuredRoot, Err: err}
	}
	return result, nil
}

func cloneLayer(source model.ConfigLayer) model.ConfigLayer {
	clone := model.ConfigLayer{
		Scope:      source.Scope,
		MCPs:       make(map[string]model.MCPDefinition, len(source.MCPs)),
		Skills:     make(map[string]model.SkillDefinition, len(source.Skills)),
		SkillRoots: append([]string(nil), source.SkillRoots...),
		Verifiers:  make(map[string]model.VerifierDefinition, len(source.Verifiers)),
	}
	if source.Policy != nil {
		clone.Policy = &model.Policy{
			Mode:               source.Policy.Mode,
			AllowedExecutables: append([]string(nil), source.Policy.AllowedExecutables...),
			ToolPaths:          cloneMap(source.Policy.ToolPaths),
		}
	}
	for id, definition := range source.MCPs {
		clone.MCPs[id] = cloneMCP(definition)
	}
	for id, definition := range source.Skills {
		clone.Skills[id] = cloneSkill(definition)
	}
	for id, definition := range source.Verifiers {
		clone.Verifiers[id] = cloneVerifier(definition)
	}
	return clone
}

func cloneMCP(source model.MCPDefinition) model.MCPDefinition {
	clone := source
	clone.Enabled = cloneBool(source.Enabled)
	clone.Env = cloneMap(source.Env)
	clone.EnvRefs = cloneMap(source.EnvRefs)
	return clone
}

func cloneSkill(source model.SkillDefinition) model.SkillDefinition {
	clone := source
	clone.Enabled = cloneBool(source.Enabled)
	return clone
}

func cloneVerifier(source model.VerifierDefinition) model.VerifierDefinition {
	clone := source
	clone.Enabled = cloneBool(source.Enabled)
	clone.Args = append([]string(nil), source.Args...)
	return clone
}

func cloneBool(source *bool) *bool {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func cloneMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
