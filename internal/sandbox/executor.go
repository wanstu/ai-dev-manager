package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type ExecutionRequest struct {
	Scope           string            `json:"scope"`
	WorktreeRoot    string            `json:"worktree_root"`
	Executable      string            `json:"executable"`
	Args            []string          `json:"args,omitempty"`
	Cwd             string            `json:"cwd"`
	BaseEnvironment []string          `json:"-"`
	Environment     map[string]string `json:"-"`
	ReadOnlyRoots   []string          `json:"read_only_roots,omitempty"`
	Stdout          io.Writer         `json:"-"`
	Stderr          io.Writer         `json:"-"`
}

type ExecutionResult struct {
	LaunchResult
	ProfileName string `json:"profile_name,omitempty"`
}

func Execute(ctx context.Context, request ExecutionRequest) (result ExecutionResult, err error) {
	support := PlatformSupport()
	if !support.Supported || !support.StrongFileBoundarySupported || !support.ProcessTreeBoundary {
		return ExecutionResult{}, fmt.Errorf("strong sandbox execution is unavailable on %s", support.Platform)
	}

	worktreeRoot, executable, cwd, err := validateExecutionRequest(request)
	if err != nil {
		return ExecutionResult{}, err
	}

	runtimeRoot, err := os.MkdirTemp("", "adm-sandbox-runtime-*")
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("create sandbox runtime root: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(runtimeRoot); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove sandbox runtime root: %w", cleanupErr))
		}
	}()

	dirs := DefaultRuntimeDirs(runtimeRoot)
	if err := createRuntimeDirectories(dirs); err != nil {
		return ExecutionResult{}, err
	}

	profileName, err := StableAppContainerProfileName(request.Scope)
	if err != nil {
		return ExecutionResult{}, err
	}
	profile, _, err := EnsureWindowsAppContainerProfile(profileName)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("ensure AppContainer profile: %w", err)
	}
	result.ProfileName = profile.Name
	defer func() {
		if cleanupErr := DeleteWindowsAppContainerProfile(profile.Name); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("delete AppContainer profile %q: %w", profile.Name, cleanupErr))
		}
	}()

	granted := make([]string, 0, 3+len(request.ReadOnlyRoots))
	grant := func(path string, access PathAccess) error {
		path, resolveErr := canonicalSandboxGrantPath(path)
		if resolveErr != nil {
			return resolveErr
		}
		for _, existing := range granted {
			if sameSandboxPath(existing, path) {
				return nil
			}
		}
		if grantErr := GrantWindowsAppContainerPath(path, profile.SID, access); grantErr != nil {
			return grantErr
		}
		granted = append(granted, path)
		return nil
	}
	defer func() {
		for i := len(granted) - 1; i >= 0; i-- {
			if cleanupErr := RevokeWindowsAppContainerPath(granted[i], profile.SID); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("revoke AppContainer path %s: %w", granted[i], cleanupErr))
			}
		}
	}()

	if err := grant(worktreeRoot, PathAccessReadWrite); err != nil {
		return result, fmt.Errorf("grant sandbox worktree: %w", err)
	}
	if err := grant(runtimeRoot, PathAccessReadWrite); err != nil {
		return result, fmt.Errorf("grant sandbox runtime: %w", err)
	}
	if !pathWithin(worktreeRoot, executable) && !pathWithin(runtimeRoot, executable) {
		if err := grant(executable, PathAccessReadOnly); err != nil {
			return result, fmt.Errorf("grant sandbox executable: %w", err)
		}
	}
	for _, root := range request.ReadOnlyRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		resolved, resolveErr := canonicalSandboxGrantPath(root)
		if resolveErr != nil {
			return result, fmt.Errorf("resolve sandbox read-only root %q: %w", root, resolveErr)
		}
		if pathWithin(worktreeRoot, resolved) || pathWithin(runtimeRoot, resolved) || sameSandboxPath(resolved, executable) {
			continue
		}
		if err := grant(resolved, PathAccessReadOnly); err != nil {
			return result, fmt.Errorf("grant sandbox read-only root %s: %w", resolved, err)
		}
	}

	environment, err := BuildEnvironment(request.BaseEnvironment, dirs, request.Environment)
	if err != nil {
		return result, err
	}
	result.LaunchResult, err = Launch(ctx, LaunchRequest{
		Executable:  executable,
		Args:        append([]string(nil), request.Args...),
		Cwd:         cwd,
		Environment: environment,
		ProfileSID:  profile.SID,
		Stdout:      request.Stdout,
		Stderr:      request.Stderr,
	})
	return result, err
}

func validateExecutionRequest(request ExecutionRequest) (worktreeRoot, executable, cwd string, err error) {
	if strings.TrimSpace(request.Scope) == "" {
		return "", "", "", fmt.Errorf("sandbox execution scope is required")
	}
	worktreeRoot, err = canonicalSandboxDirectory(request.WorktreeRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("sandbox worktree root: %w", err)
	}
	executable, err = canonicalSandboxFile(request.Executable)
	if err != nil {
		return "", "", "", fmt.Errorf("sandbox executable: %w", err)
	}
	cwd, err = canonicalSandboxDirectory(request.Cwd)
	if err != nil {
		return "", "", "", fmt.Errorf("sandbox cwd: %w", err)
	}
	if !pathWithin(worktreeRoot, cwd) {
		return "", "", "", fmt.Errorf("sandbox cwd escapes worktree root")
	}
	return worktreeRoot, executable, cwd, nil
}

func createRuntimeDirectories(dirs RuntimeDirs) error {
	if err := dirs.Validate(); err != nil {
		return err
	}
	for _, dir := range []string{
		dirs.Root,
		dirs.ProfileRoot,
		filepath.Join(dirs.ProfileRoot, "AppData", "Roaming"),
		filepath.Join(dirs.ProfileRoot, "AppData", "Local"),
		dirs.TempRoot,
		dirs.CacheRoot,
		dirs.BuildRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create sandbox runtime directory %s: %w", dir, err)
		}
	}
	gitConfig := filepath.Join(dirs.ProfileRoot, ".gitconfig")
	if err := os.WriteFile(gitConfig, nil, 0o600); err != nil {
		return fmt.Errorf("create sandbox Git config: %w", err)
	}
	return nil
}

func canonicalSandboxDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute directory path is required")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	return filepath.Clean(resolved), nil
}

func canonicalSandboxFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute file path is required")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path is not a regular file")
	}
	return filepath.Clean(resolved), nil
}

func canonicalSandboxGrantPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute grant path is required")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func sameSandboxPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
