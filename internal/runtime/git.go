package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type GitStatusEntry struct {
	Path     string
	OrigPath string
	X        string
	Y        string
}

type GitDiff struct {
	Files []string
	Patch string
}

type Worktree struct {
	Path   string
	Head   string
	Branch string
	Bare   bool
}

var ErrGitBranchExists = errors.New("git branch already exists")

type GitError struct {
	Operation string
	ExitCode  int
	Stderr    string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s failed with exit code %d", e.Operation, e.ExitCode)
}

func (r *Native) GitStatus() ([]GitStatusEntry, error) {
	result, err := r.gitExec("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, gitResultError("status", result)
	}
	parts := strings.Split(result.Stdout, "\x00")
	entries := make([]GitStatusEntry, 0)
	for i := 0; i < len(parts); i++ {
		record := parts[i]
		if record == "" || len(record) < 3 {
			continue
		}
		entry := GitStatusEntry{
			X:    string(record[0]),
			Y:    string(record[1]),
			Path: filepath.Clean(filepath.FromSlash(strings.TrimPrefix(record[2:], " "))),
		}
		if (entry.X == "R" || entry.X == "C" || entry.Y == "R" || entry.Y == "C") && i+1 < len(parts) && parts[i+1] != "" {
			entry.OrigPath = filepath.Clean(filepath.FromSlash(parts[i+1]))
			i++
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func (r *Native) GitDiff() (GitDiff, error) {
	headResult, err := r.gitExec("rev-parse", "--verify", "HEAD")
	if err != nil {
		return GitDiff{}, err
	}
	baseArgs := []string{"HEAD"}
	if headResult.ExitCode != 0 {
		// On an unborn branch there is no HEAD yet. Diff the index instead of
		// failing with exit code 128; untracked files remain represented by
		// GitStatus while staged initial content can still produce a patch.
		baseArgs = []string{"--cached"}
	}
	filesArgs := append([]string{"diff"}, baseArgs...)
	filesArgs = append(filesArgs, "--name-only", "-z", "--")
	filesResult, err := r.gitExec(filesArgs...)
	if err != nil {
		return GitDiff{}, err
	}
	if filesResult.ExitCode != 0 {
		return GitDiff{}, gitResultError("diff --name-only", filesResult)
	}
	patchArgs := append([]string{"diff"}, baseArgs...)
	patchArgs = append(patchArgs, "--no-ext-diff", "--unified=3", "--")
	patchResult, err := r.gitExec(patchArgs...)
	if err != nil {
		return GitDiff{}, err
	}
	if patchResult.ExitCode != 0 {
		return GitDiff{}, gitResultError("diff", patchResult)
	}
	files := make([]string, 0)
	for _, raw := range strings.Split(filesResult.Stdout, "\x00") {
		if raw == "" {
			continue
		}
		files = append(files, filepath.Clean(filepath.FromSlash(raw)))
	}
	sort.Strings(files)
	return GitDiff{Files: files, Patch: patchResult.Stdout}, nil
}

func (r *Native) GitBranch() (string, error) {
	result, err := r.gitExec("branch", "--show-current")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", gitResultError("branch --show-current", result)
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (r *Native) GitResolveRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !safeGitRefInput(ref) {
		return "", &RuntimeError{Kind: ErrInvalidPath, Path: ref}
	}
	result, err := r.gitExec("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", gitResultError("rev-parse --verify", result)
	}
	commit := strings.TrimSpace(result.Stdout)
	if commit == "" {
		return "", &RuntimeError{Kind: ErrNotFound, Path: ref}
	}
	return commit, nil
}

func (r *Native) GitWorktrees() ([]Worktree, error) {
	result, err := r.gitExec("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, gitResultError("worktree list", result)
	}
	return parseWorktrees(result.Stdout), nil
}

func (r *Native) GitBranchExists(branch string) (bool, error) {
	branch = strings.TrimSpace(branch)
	if !safeGitRefInput(branch) {
		return false, &RuntimeError{Kind: ErrInvalidPath, Path: branch}
	}
	check, err := r.gitExec("check-ref-format", "--branch", branch)
	if err != nil {
		return false, err
	}
	if check.ExitCode != 0 {
		return false, &RuntimeError{Kind: ErrInvalidPath, Path: branch}
	}
	result, err := r.gitExec("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, gitResultError("show-ref --verify", result)
	}
}

func (r *Native) GitWorktreeGet(name string) (Worktree, error) {
	if !safeIdentifier(name) {
		return Worktree{}, &RuntimeError{Kind: ErrInvalidPath, Path: name}
	}
	managedRoot, err := r.managedWorktreeRoot()
	if err != nil {
		return Worktree{}, err
	}
	target := filepath.Join(managedRoot, name)
	worktrees, err := r.GitWorktrees()
	if err != nil {
		return Worktree{}, err
	}
	for _, worktree := range worktrees {
		if samePath(worktree.Path, target) {
			return worktree, nil
		}
	}
	return Worktree{}, &RuntimeError{Kind: ErrNotFound, Path: target}
}

func (r *Native) GitWorktreeCreate(name, branch string) (Worktree, error) {
	return r.GitWorktreeCreateAt(name, branch, "HEAD")
}

func (r *Native) GitWorktreeCreateAt(name, branch, startPoint string) (Worktree, error) {
	if !safeIdentifier(name) {
		return Worktree{}, &RuntimeError{Kind: ErrInvalidPath, Path: name}
	}
	branch = strings.TrimSpace(branch)
	exists, err := r.GitBranchExists(branch)
	if err != nil {
		return Worktree{}, err
	}
	if exists {
		return Worktree{}, fmt.Errorf("%w: %s", ErrGitBranchExists, branch)
	}
	startCommit, err := r.GitResolveRef(startPoint)
	if err != nil {
		return Worktree{}, err
	}
	managedRoot, err := r.managedWorktreeRoot()
	if err != nil {
		return Worktree{}, err
	}
	if err := os.MkdirAll(managedRoot, 0o755); err != nil {
		return Worktree{}, &RuntimeError{Kind: ErrIO, Path: managedRoot, Err: err}
	}
	target := filepath.Join(managedRoot, name)
	if _, err := os.Stat(target); err == nil {
		return Worktree{}, &RuntimeError{Kind: ErrInvalidPath, Path: target}
	} else if !os.IsNotExist(err) {
		return Worktree{}, &RuntimeError{Kind: ErrIO, Path: target, Err: err}
	}

	result, err := r.gitExec("worktree", "add", "-b", branch, target, startCommit)
	if err != nil {
		return Worktree{}, err
	}
	if result.ExitCode != 0 {
		return Worktree{}, gitResultError("worktree add", result)
	}
	worktrees, err := r.GitWorktrees()
	if err != nil {
		return Worktree{}, err
	}
	for _, worktree := range worktrees {
		if samePath(worktree.Path, target) {
			return worktree, nil
		}
	}
	return Worktree{}, &RuntimeError{Kind: ErrNotFound, Path: target}
}

func (r *Native) GitWorktreeRemove(name string) error {
	return r.GitWorktreeRemoveWithOptions(name, false)
}

func (r *Native) GitWorktreeRemoveWithOptions(name string, force bool) error {
	if !safeIdentifier(name) {
		return &RuntimeError{Kind: ErrInvalidPath, Path: name}
	}
	managedRoot, err := r.managedWorktreeRoot()
	if err != nil {
		return err
	}
	target := filepath.Join(managedRoot, name)
	if !within(managedRoot, target) || samePath(managedRoot, target) {
		return &RuntimeError{Kind: ErrPathOutsideWorkspace, Path: target}
	}
	if _, err := r.GitWorktreeGet(name); err != nil {
		return err
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, target)
	result, err := r.gitExec(args...)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return gitResultError("worktree remove", result)
	}
	return nil
}

func (r *Native) managedWorktreeRoot() (string, error) {
	if !safeIdentifier(r.workspace.ID) {
		return "", &RuntimeError{Kind: ErrInvalidPath, Path: r.workspace.ID}
	}
	return filepath.Clean(filepath.Join(filepath.Dir(r.root), ".ai-dev-manager-worktrees", r.workspace.ID)), nil
}

func (r *Native) gitExec(args ...string) (CommandResult, error) {
	return r.Exec(Command{Executable: "git", Args: args, Cwd: r.root})
}

func gitResultError(operation string, result CommandResult) error {
	return &GitError{Operation: operation, ExitCode: result.ExitCode, Stderr: strings.TrimSpace(result.Stderr)}
}

func parseWorktrees(output string) []Worktree {
	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	worktrees := make([]Worktree, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		var worktree Worktree
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "worktree "):
				worktree.Path = filepath.Clean(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				worktree.Head = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				worktree.Branch = strings.TrimPrefix(line, "branch ")
			case line == "bare":
				worktree.Bare = true
			}
		}
		if worktree.Path != "" {
			worktrees = append(worktrees, worktree)
		}
	}
	sort.Slice(worktrees, func(i, j int) bool { return strings.ToLower(worktrees[i].Path) < strings.ToLower(worktrees[j].Path) })
	return worktrees
}

func safeIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 64 || value == "." || value == ".." || strings.Contains(value, "..") {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func safeGitRefInput(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 || strings.HasPrefix(value, "-") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
