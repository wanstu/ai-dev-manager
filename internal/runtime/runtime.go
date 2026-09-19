package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/pathutil"
)

const (
	CapabilityTree      = "files.tree"
	CapabilityRead      = "files.read"
	CapabilitySearch    = "search.text"
	CapabilityWrite     = "files.write"
	CapabilityEdit      = "files.edit"
	CapabilityDelete    = "files.delete"
	CapabilityExec      = "shell.exec"
	CapabilityGitStatus = "git.status"
	CapabilityGitDiff   = "git.diff"
	CapabilityGitBranch = "git.branch"
)

type Runtime struct {
	root                     string
	allowedExecutables       []string
	blockedExecutables       []string
	baseEnvironment          []string
	allowUnlistedExecutables bool
}

type FileInfo struct {
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size,omitempty"`
}

type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type WriteResult struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

type EditResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
}

type DeleteResult struct {
	Path string `json:"path"`
}

type CommandResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

type GitStatusEntry struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

func New(root string, allowedExecutables []string) (*Runtime, error) {
	return NewWithAuthorization(root, allowedExecutables, false)
}

func NewWithAuthorization(root string, allowedExecutables []string, allowUnlistedExecutables bool) (*Runtime, error) {
	return NewWithExecutionPolicy(root, allowedExecutables, nil, os.Environ(), allowUnlistedExecutables)
}

func NewWithEnvironment(root string, allowedExecutables, environment []string) (*Runtime, error) {
	return NewWithExecutionPolicy(root, allowedExecutables, nil, environment, false)
}

func NewWithPolicy(root string, allowedExecutables, environment []string, allowUnlistedExecutables bool) (*Runtime, error) {
	return NewWithExecutionPolicy(root, allowedExecutables, nil, environment, allowUnlistedExecutables)
}

func NewWithExecutionPolicy(root string, allowedExecutables, blockedExecutables, environment []string, allowUnlistedExecutables bool) (*Runtime, error) {
	resolved, err := canonicalDir(root)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		root:                     resolved,
		allowedExecutables:       append([]string(nil), allowedExecutables...),
		blockedExecutables:       append([]string(nil), blockedExecutables...),
		baseEnvironment:          append([]string(nil), environment...),
		allowUnlistedExecutables: allowUnlistedExecutables,
	}, nil
}

func (r *Runtime) Root() string { return r.root }

// Command prepares an allowlisted command rooted in this Environment. Callers
// that own a long-lived protocol transport remain responsible for starting and
// closing the command.
func (r *Runtime) Command(ctx context.Context, executable string, args []string, extraEnv map[string]string) (*exec.Cmd, error) {
	resolved, err := r.allowedExecutable(executable)
	if err != nil {
		return nil, err
	}
	if err := validateReservedCommand(executable, args); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	cmd.Dir = r.root
	cmd.Env = append([]string(nil), r.baseEnvironment...)
	if len(extraEnv) != 0 {
		keys := make([]string, 0, len(extraEnv))
		for key := range extraEnv {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			cmd.Env = replaceEnvironmentValue(cmd.Env, key, extraEnv[key])
		}
	}
	configureCommand(cmd)
	return cmd, nil
}

func (r *Runtime) Capabilities(ctx context.Context) []string {
	caps := []string{CapabilityTree, CapabilityRead, CapabilitySearch, CapabilityWrite, CapabilityEdit, CapabilityDelete}
	if len(r.allowedExecutables) > 0 {
		caps = append(caps, CapabilityExec)
	}
	if r.gitSupported(ctx) {
		caps = append(caps, CapabilityGitStatus, CapabilityGitDiff, CapabilityGitBranch)
	}
	sort.Strings(caps)
	return caps
}

func (r *Runtime) Tree(path string, maxDepth, maxEntries int) ([]FileInfo, error) {
	if maxDepth <= 0 {
		maxDepth = 4
	}
	if maxEntries <= 0 {
		maxEntries = 800
	}
	start, err := r.existing(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(start)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("tree path is not a directory")
	}
	baseDepth := depth(start)
	var out []FileInfo
	err = filepath.WalkDir(start, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == start {
			return nil
		}
		currentDepth := depth(current) - baseDepth
		if currentDepth > maxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= maxEntries {
			return io.EOF
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(r.root, current)
		out = append(out, FileInfo{Path: filepath.ToSlash(rel), Dir: entry.IsDir(), Size: entryInfo.Size()})
		return nil
	})
	if errors.Is(err, io.EOF) {
		return out, nil
	}
	return out, err
}

func (r *Runtime) Read(path string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	target, err := r.existing(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("read path is not a regular file")
	}
	if info.Size() > int64(maxBytes) {
		return "", fmt.Errorf("file exceeds max_bytes=%d", maxBytes)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("binary file is not readable as text")
	}
	return string(data), nil
}

func (r *Runtime) Search(path, query string, maxFiles, maxMatches, maxBytesPerFile int) ([]SearchMatch, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if maxFiles <= 0 {
		maxFiles = 2000
	}
	if maxMatches <= 0 {
		maxMatches = 200
	}
	if maxBytesPerFile <= 0 {
		maxBytesPerFile = 1 << 20
	}
	start, err := r.existing(path)
	if err != nil {
		return nil, err
	}
	var out []SearchMatch
	files := 0
	walkOne := func(target string) error {
		info, err := os.Stat(target)
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(maxBytesPerFile) {
			return err
		}
		files++
		if files > maxFiles {
			return io.EOF
		}
		file, err := os.Open(target)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, maxBytesPerFile)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(text, query) {
				rel, _ := filepath.Rel(r.root, target)
				out = append(out, SearchMatch{Path: filepath.ToSlash(rel), Line: line, Text: text})
				if len(out) >= maxMatches {
					return io.EOF
				}
			}
		}
		return scanner.Err()
	}
	info, err := os.Stat(start)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		err = walkOne(start)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		return out, err
	}
	err = filepath.WalkDir(start, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		return walkOne(current)
	})
	if errors.Is(err, io.EOF) {
		return out, nil
	}
	return out, err
}

func (r *Runtime) Write(path, content string, createParents bool) (WriteResult, error) {
	target, err := r.writeTarget(path)
	if err != nil {
		return WriteResult{}, err
	}
	if createParents {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return WriteResult{}, err
		}
	}
	if err := atomicWrite(target, []byte(content)); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Path: cleanRelative(path), Bytes: len(content)}, nil
}

func (r *Runtime) Edit(path, oldText, newText string, expectedReplacements int) (EditResult, error) {
	if oldText == "" {
		return EditResult{}, fmt.Errorf("old_text is required")
	}
	target, err := r.existing(path)
	if err != nil {
		return EditResult{}, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return EditResult{}, err
	}
	current := string(data)
	count := strings.Count(current, oldText)
	if expectedReplacements <= 0 {
		expectedReplacements = 1
	}
	if count != expectedReplacements {
		return EditResult{}, fmt.Errorf("expected %d replacements, found %d", expectedReplacements, count)
	}
	updated := strings.ReplaceAll(current, oldText, newText)
	if err := atomicWrite(target, []byte(updated)); err != nil {
		return EditResult{}, err
	}
	return EditResult{Path: cleanRelative(path), Replacements: count}, nil
}

func (r *Runtime) Delete(path string) (DeleteResult, error) {
	target, err := r.existing(path)
	if err != nil {
		return DeleteResult{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return DeleteResult{}, err
	}
	if info.IsDir() {
		return DeleteResult{}, fmt.Errorf("recursive directory deletion is not supported")
	}
	if err := os.Remove(target); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{Path: cleanRelative(path)}, nil
}

// PrepareCommand validates the same executable/cwd authority used by Exec and
// returns a context-bound command without starting it. Long-lived resources
// such as Gateway-owned dev processes use this seam so they do not duplicate
// allowlist, containment, or OS cancellation policy.
func (r *Runtime) PrepareCommand(ctx context.Context, executable string, args []string, cwd string) (*exec.Cmd, error) {
	resolved, err := r.allowedExecutable(executable)
	if err != nil {
		return nil, err
	}
	if err := validateReservedCommand(executable, args); err != nil {
		return nil, err
	}
	workingDir := r.root
	if strings.TrimSpace(cwd) != "" {
		workingDir, err = r.existing(cwd)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(workingDir)
		if statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("exec cwd is not a directory")
		}
	}
	cmd := exec.CommandContext(ctx, resolved, args...)
	configureCommand(cmd)
	cmd.Dir = workingDir
	cmd.Env = append([]string(nil), r.baseEnvironment...)
	return cmd, nil
}

func (r *Runtime) Exec(ctx context.Context, executable string, args []string, cwd string, timeoutMS int64, maxOutputBytes int) (CommandResult, error) {
	if timeoutMS <= 0 {
		timeoutMS = 30000
	}
	if maxOutputBytes <= 0 {
		maxOutputBytes = 120000
	}
	commandCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	cmd, err := r.PrepareCommand(commandCtx, executable, args, cwd)
	if err != nil {
		return CommandResult{}, err
	}
	stdout := &limitedBuffer{limit: maxOutputBytes}
	stderr := &limitedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	result := CommandResult{ExitCode: 0, Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("command timed out after %dms", timeoutMS)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func (r *Runtime) GitStatus(ctx context.Context) ([]GitStatusEntry, error) {
	if !r.gitSupported(ctx) {
		return nil, fmt.Errorf("git.status is unsupported for this environment root")
	}
	out, err := r.gitOutput(ctx, "status", "--porcelain=v1")
	if err != nil {
		return nil, err
	}
	var entries []GitStatusEntry
	for _, line := range strings.Split(strings.TrimRight(out, "\r\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 3 {
			continue
		}
		entries = append(entries, GitStatusEntry{Code: line[:2], Path: strings.TrimSpace(line[3:])})
	}
	return entries, nil
}

func (r *Runtime) GitDiff(ctx context.Context) (string, error) {
	if !r.gitSupported(ctx) {
		return "", fmt.Errorf("git.diff is unsupported for this environment root")
	}
	return r.gitOutput(ctx, "diff", "--no-ext-diff", "--binary", "HEAD")
}

func (r *Runtime) GitBranch(ctx context.Context) (string, error) {
	if !r.gitSupported(ctx) {
		return "", fmt.Errorf("git.branch is unsupported for this environment root")
	}
	out, err := r.gitOutput(ctx, "branch", "--show-current")
	return strings.TrimSpace(out), err
}

func (r *Runtime) gitSupported(ctx context.Context) bool {
	git, err := exec.LookPath("git")
	if err != nil {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, git, "-C", r.root, "rev-parse", "--is-inside-work-tree")
	configureCommand(cmd)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func (r *Runtime) gitOutput(ctx context.Context, args ...string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	fullArgs := append([]string{"-C", r.root}, args...)
	cmd := exec.CommandContext(ctx, git, fullArgs...)
	configureCommand(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (r *Runtime) IsExplicitlyAllowed(executable string) bool {
	executable = strings.TrimSpace(executable)
	if executable == "" || isCommandProxyExecutable(executable) {
		return false
	}
	for _, allowed := range r.allowedExecutables {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if filepath.IsAbs(allowed) {
			if filepath.IsAbs(executable) && samePath(allowed, executable) {
				return true
			}
			continue
		}
		if strings.EqualFold(allowed, executable) {
			return true
		}
	}
	return false
}

func (r *Runtime) FullAuthorizationEnabled() bool { return r.allowUnlistedExecutables }

func (r *Runtime) IsBlockedExecutable(executable string) bool {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return false
	}
	for _, blocked := range r.blockedExecutables {
		blocked = strings.TrimSpace(blocked)
		if blocked == "" {
			continue
		}
		if filepath.IsAbs(blocked) {
			if filepath.IsAbs(executable) && samePath(blocked, executable) {
				return true
			}
			continue
		}
		if executablePolicyNameMatch(blocked, executable) {
			return true
		}
	}
	return false
}

func executablePolicyNameMatch(policy, executable string) bool {
	policy = strings.TrimSpace(policy)
	executable = strings.TrimSpace(executable)
	if policy == "" || executable == "" {
		return false
	}
	if strings.EqualFold(policy, executable) {
		return true
	}
	if filepath.Base(policy) != policy {
		return false
	}
	policyName := strings.TrimSuffix(strings.ToLower(policy), ".exe")
	executableName := strings.TrimSuffix(strings.ToLower(filepath.Base(executable)), ".exe")
	return policyName == executableName
}

func (r *Runtime) allowedExecutable(executable string) (string, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return "", fmt.Errorf("executable is required")
	}
	if r.IsBlockedExecutable(executable) {
		return "", fmt.Errorf("executable %q is blocked by the command blacklist", executable)
	}
	if isCommandProxyExecutable(executable) && !r.allowUnlistedExecutables {
		return "", fmt.Errorf("executable %q is not allowed in strict mode because it can dispatch arbitrary commands", executable)
	}
	for _, allowed := range r.allowedExecutables {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if filepath.IsAbs(allowed) {
			if filepath.IsAbs(executable) && samePath(allowed, executable) {
				return allowed, nil
			}
			continue
		}
		if strings.EqualFold(allowed, executable) {
			resolved, err := exec.LookPath(executable)
			if err != nil {
				return "", fmt.Errorf("allowed executable %q is not available: %w", executable, err)
			}
			return resolved, nil
		}
	}
	if r.allowUnlistedExecutables {
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return "", fmt.Errorf("executable %q is unavailable: %w", executable, err)
		}
		return resolved, nil
	}
	return "", fmt.Errorf("executable %q is not allowed", executable)
}

func validateReservedCommand(executable string, args []string) error {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	name = strings.TrimSuffix(name, ".exe")
	if name != "git" {
		return nil
	}
	if gitSubcommand(args) != "worktree" {
		return nil
	}
	return fmt.Errorf("git worktree is managed by ADM and cannot be invoked through exec; use environment_worktree_create, environment_worktree_list, or environment_worktree_destroy")
}

func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch arg {
		case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix", "--config-env", "--exec-path":
			i++
			continue
		case "--":
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
			return ""
		}
		if strings.HasPrefix(arg, "--git-dir=") ||
			strings.HasPrefix(arg, "--work-tree=") ||
			strings.HasPrefix(arg, "--namespace=") ||
			strings.HasPrefix(arg, "--super-prefix=") ||
			strings.HasPrefix(arg, "--config-env=") ||
			strings.HasPrefix(arg, "--exec-path=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

func isCommandProxyExecutable(executable string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "powershell", "pwsh", "cmd", "bash", "sh", "zsh", "fish", "wsl":
		return true
	default:
		return false
	}
}

func (r *Runtime) existing(path string) (string, error) {
	candidate, err := r.lexical(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if !within(r.root, resolved) {
		return "", fmt.Errorf("path escapes environment root")
	}
	return filepath.Clean(resolved), nil
}

func (r *Runtime) writeTarget(path string) (string, error) {
	candidate, err := r.lexical(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(candidate)
	for {
		resolvedParent, resolveErr := filepath.EvalSymlinks(parent)
		if resolveErr == nil {
			if !within(r.root, resolvedParent) {
				return "", fmt.Errorf("path escapes environment root")
			}
			break
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", resolveErr
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", resolveErr
		}
		parent = next
	}
	return candidate, nil
}

func (r *Runtime) lexical(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return r.root, nil
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not accepted; use environment-relative paths")
	}
	candidate := filepath.Clean(filepath.Join(r.root, path))
	if !within(r.root, candidate) {
		return "", fmt.Errorf("path escapes environment root")
	}
	return candidate, nil
}

func canonicalDir(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("runtime root is not a directory")
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

func cleanRelative(path string) string { return filepath.ToSlash(filepath.Clean(path)) }

func replaceEnvironmentValue(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		index := strings.IndexByte(item, '=')
		if index > 0 && strings.ToUpper(item[:index])+"=" == prefix {
			continue
		}
		result = append(result, item)
	}
	return append(result, key+"="+value)
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".adm-v2-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func depth(path string) int {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(filepath.Separator) {
		return 0
	}
	return len(strings.Split(clean, string(filepath.Separator)))
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return original, nil
}

func (b *limitedBuffer) String() string { return b.buf.String() }
