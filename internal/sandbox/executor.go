package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ExecutionRequest struct {
	Scope           string            `json:"scope"`
	RuntimeRoot     string            `json:"runtime_root"`
	WorktreeRoot    string            `json:"worktree_root"`
	Executable      string            `json:"executable"`
	Args            []string          `json:"args,omitempty"`
	Cwd             string            `json:"cwd"`
	BaseEnvironment []string          `json:"-"`
	ExtraEnvironment map[string]string `json:"-"`
	ToolRoots       []string          `json:"tool_roots,omitempty"`
	MaxOutputBytes  int               `json:"max_output_bytes,omitempty"`
	Stdout          io.Writer         `json:"-"`
	Stderr          io.Writer         `json:"-"`
}

type ExecutionResult struct {
	LaunchResult
	ProfileName string `json:"profile_name,omitempty"`
}

func (r ExecutionRequest) Validate() error {
	if strings.TrimSpace(r.Scope) == "" {
		return fmt.Errorf("sandbox execution scope is required")
	}
	for name, value := range map[string]string{
		"runtime_root":  r.RuntimeRoot,
		"worktree_root": r.WorktreeRoot,
		"executable":    r.Executable,
		"cwd":           r.Cwd,
	} {
		if strings.TrimSpace(value) == "" || !filepath.IsAbs(value) {
			return fmt.Errorf("sandbox %s must be an absolute path", name)
		}
	}
	if r.MaxOutputBytes < 0 {
		return fmt.Errorf("sandbox max_output_bytes must be nonnegative")
	}
	return nil
}

// Execute creates an isolated AppContainer identity for one execution,
// grants only the selected project/runtime/tool paths, launches the process,
// and revokes those grants before returning.
//
// There is intentionally no unsandboxed fallback here. Callers that want a
// trusted-host mode must choose that separate authority explicitly.
func Execute(ctx context.Context, request ExecutionRequest) (result ExecutionResult, err error) {
	if err := request.Validate(); err != nil {
		return ExecutionResult{}, err
	}
	support := PlatformSupport()
	if !support.Supported || !support.StrongFileBoundarySupported || !support.ProcessTreeBoundary {
		return ExecutionResult{}, fmt.Errorf("strong sandbox execution is unavailable on %s", support.Platform)
	}

	worktreeRoot, err := canonicalExistingDirectory(request.WorktreeRoot)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("resolve sandbox worktree root: %w", err)
	}
	cwd, err := canonicalExistingDirectory(request.Cwd)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("resolve sandbox cwd: %w", err)
	}
	if !pathWithin(worktreeRoot, cwd) {
		return ExecutionResult{}, fmt.Errorf("sandbox cwd must stay inside the worktree root")
	}
	executable, err := canonicalExistingFile(request.Executable)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("resolve sandbox executable: %w", err)
	}

	runtimeRoot, err := filepath.Abs(filepath.Clean(request.RuntimeRoot))
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("resolve sandbox runtime root: %w", err)
	}
	if pathWithin(worktreeRoot, runtimeRoot) || pathWithin(runtimeRoot, worktreeRoot) {
		return ExecutionResult{}, fmt.Errorf("sandbox runtime root and worktree root must not overlap")
	}
	dirs := DefaultRuntimeDirs(runtimeRoot)
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
			return ExecutionResult{}, fmt.Errorf("create sandbox runtime directory %s: %w", dir, err)
		}
	}
	defer func() {
		if removeErr := os.RemoveAll(runtimeRoot); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove sandbox runtime root: %w", removeErr))
		}
	}()

	profileName, err := StableAppContainerProfileName(request.Scope)
	if err != nil {
		return ExecutionResult{}, err
	}
	profile, _, err := EnsureWindowsAppContainerProfile(profileName)
	if err != nil {
		return ExecutionResult{}, err
	}
	result.ProfileName = profile.Name

	// Windows virtualizes GetTempPath for AppContainer processes to the
	// package-local AC\Temp directory, even when TEMP/TMP point elsewhere.
	// Create that private location inside the sandbox profile before launch.
	appContainerTemp := filepath.Join(
		dirs.ProfileRoot,
		"AppData", "Local", "Packages", profile.Name, "AC", "Temp",
	)
	if err := os.MkdirAll(appContainerTemp, 0o755); err != nil {
		return ExecutionResult{}, fmt.Errorf("create AppContainer temp directory: %w", err)
	}
	defer func() {
		if deleteErr := DeleteWindowsAppContainerProfile(profile.Name); deleteErr != nil {
			err = errors.Join(err, fmt.Errorf("delete sandbox profile: %w", deleteErr))
		}
	}()

	type grant struct {
		path   string
		access PathAccess
	}
	grants := make([]grant, 0, 4+len(request.ToolRoots))
	seen := map[string]struct{}{}
	addGrant := func(path string, access PathAccess) error {
		canonical, resolveErr := canonicalExistingPath(path)
		if resolveErr != nil {
			return resolveErr
		}
		key := strings.ToLower(filepath.Clean(canonical))
		if _, ok := seen[key]; ok {
			return nil
		}
		if access == PathAccessReadOnly {
			if validateErr := validateToolGrant(canonical); validateErr != nil {
				return validateErr
			}
		}
		if grantErr := GrantWindowsAppContainerPath(canonical, profile.SID, access); grantErr != nil {
			return grantErr
		}
		seen[key] = struct{}{}
		grants = append(grants, grant{path: canonical, access: access})
		return nil
	}
	defer func() {
		for i := len(grants) - 1; i >= 0; i-- {
			if revokeErr := RevokeWindowsAppContainerPath(grants[i].path, profile.SID); revokeErr != nil {
				err = errors.Join(err, fmt.Errorf("revoke sandbox path %s: %w", grants[i].path, revokeErr))
			}
		}
	}()

	if err := addGrant(worktreeRoot, PathAccessReadWrite); err != nil {
		return ExecutionResult{}, fmt.Errorf("grant sandbox worktree: %w", err)
	}
	if err := addGrant(runtimeRoot, PathAccessReadWrite); err != nil {
		return ExecutionResult{}, fmt.Errorf("grant sandbox runtime root: %w", err)
	}
	if err := addGrant(filepath.Dir(executable), PathAccessReadOnly); err != nil {
		return ExecutionResult{}, fmt.Errorf("grant sandbox executable directory: %w", err)
	}
	if err := addGrant(executable, PathAccessReadOnly); err != nil {
		return ExecutionResult{}, fmt.Errorf("grant sandbox executable: %w", err)
	}
	for _, root := range request.ToolRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		if err := addGrant(root, PathAccessReadOnly); err != nil {
			// Tool roots are discovered from the host toolchain and can disappear
			// between discovery and ACL application (for example CI toolcache
			// junction rotation). Missing optional roots are safe to skip: they
			// receive no sandbox access and the tool will fail normally if needed.
			if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
				continue
			}
			return ExecutionResult{}, fmt.Errorf("grant sandbox tool root %s: %w", root, err)
		}
	}

	environment, err := BuildEnvironment(request.BaseEnvironment, dirs, request.ExtraEnvironment)
	if err != nil {
		return ExecutionResult{}, err
	}
	launchResult, launchErr := Launch(ctx, LaunchRequest{
		Executable:     executable,
		Args:           append([]string(nil), request.Args...),
		Cwd:            cwd,
		Environment:    environment,
		ProfileSID:     profile.SID,
		MaxOutputBytes: request.MaxOutputBytes,
		Stdout:         request.Stdout,
		Stderr:         request.Stderr,
	})
	result.LaunchResult = launchResult
	return result, launchErr
}

func canonicalExistingPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absolute); err != nil {
		return "", err
	}
	resolved, err := resolveExistingPath(absolute)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// ResolveExistingPath returns the OS-final path for one existing file or
// directory. On Windows this resolves junctions using a file handle instead of
// relying on filepath.EvalSymlinks, which is important for hosted tool caches.
func ResolveExistingPath(path string) (string, error) {
	return canonicalExistingPath(path)
}

func canonicalExistingDirectory(path string) (string, error) {
	resolved, err := canonicalExistingPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return resolved, nil
}

func canonicalExistingFile(path string) (string, error) {
	resolved, err := canonicalExistingPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", resolved)
	}
	return resolved, nil
}

func validateToolGrant(path string) error {
	clean := filepath.Clean(path)
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("refuse to expose filesystem root %s to sandbox", clean)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		home, _ = filepath.Abs(filepath.Clean(home))
		if strings.EqualFold(home, clean) {
			return fmt.Errorf("refuse to expose the entire user profile to sandbox")
		}
	}
	return nil
}
