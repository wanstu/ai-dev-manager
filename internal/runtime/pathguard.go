package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

type PathGuard struct {
	root string
}

func (g *PathGuard) Existing(path string) (string, string, error) {
	target, err := g.lexical(path)
	if err != nil {
		return "", "", err
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", &RuntimeError{Kind: ErrNotFound, Path: path, Err: err}
		}
		return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: err}
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", "", &RuntimeError{Kind: ErrInvalidPath, Path: path, Err: err}
	}
	if !within(g.root, resolved) {
		return "", "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
	}
	rel, err := filepath.Rel(g.root, resolved)
	if err != nil {
		return "", "", &RuntimeError{Kind: ErrInvalidPath, Path: path, Err: err}
	}
	return filepath.Clean(resolved), filepath.Clean(rel), nil
}

func (g *PathGuard) WriteTarget(path string) (string, string, error) {
	target, err := g.lexical(path)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(g.root, target)
	if err != nil {
		return "", "", &RuntimeError{Kind: ErrInvalidPath, Path: path, Err: err}
	}
	if blockedWrite(rel) {
		return "", "", &RuntimeError{Kind: ErrPathBlocked, Path: filepath.Clean(rel)}
	}

	if info, statErr := os.Lstat(target); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, evalErr := filepath.EvalSymlinks(target)
			if evalErr != nil {
				return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: evalErr}
			}
			if !within(g.root, resolved) {
				return "", "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
			}
		}
	} else if !os.IsNotExist(statErr) {
		return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: statErr}
	}

	parent := filepath.Dir(target)
	for {
		info, statErr := os.Lstat(parent)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				resolvedParent, evalErr := filepath.EvalSymlinks(parent)
				if evalErr != nil {
					return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: evalErr}
				}
				if !within(g.root, resolvedParent) {
					return "", "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
				}
			} else {
				resolvedParent, evalErr := filepath.EvalSymlinks(parent)
				if evalErr != nil {
					return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: evalErr}
				}
				if !within(g.root, resolvedParent) {
					return "", "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
				}
			}
			break
		}
		if !os.IsNotExist(statErr) {
			return "", "", &RuntimeError{Kind: ErrIO, Path: path, Err: statErr}
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
		}
		parent = next
	}

	return filepath.Clean(target), filepath.Clean(rel), nil
}

func (g *PathGuard) lexical(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	var target string
	if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Join(g.root, path)
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", &RuntimeError{Kind: ErrInvalidPath, Path: path, Err: err}
	}
	absolute = filepath.Clean(absolute)
	if !within(g.root, absolute) {
		return "", &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: path}
	}
	return absolute, nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func blockedWrite(rel string) bool {
	clean := filepath.Clean(rel)
	parts := strings.FieldsFunc(clean, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return false
	}
	if strings.EqualFold(parts[0], ".git") {
		return true
	}
	return len(parts) >= 2 && strings.EqualFold(parts[0], ".ai-dev-manager") && strings.EqualFold(parts[1], "runtime")
}
